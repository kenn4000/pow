package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/ai"
	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/skills"
)

type webCampaignContext struct {
	ID          string `json:"id"`
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Hint        string `json:"hint"`
	Replay      bool   `json:"replay"`
	RewardMarks int64  `json:"rewardMarks"`
	Result      string `json:"result,omitempty"`
}

func heroOption(hero heroes.Hero, unlocked bool) webHeroOption {
	return webHeroOption{ID: hero.ID, Name: hero.Name, Unlocked: unlocked, Flavor: hero.Flavor,
		Bounty: hero.Bounty, Shift: hero.Shift, Favored: hero.Favored, Flip: hero.Flip, Unlock: hero.Unlock}
}

func achievementRewardDescription(marks, crowns int64, badge string, chromaPicks, heroBoxes int) string {
	parts := []string{}
	if marks > 0 {
		parts = append(parts, fmt.Sprintf("%d Marks", marks))
	}
	if crowns > 0 {
		parts = append(parts, fmt.Sprintf("%d Crowns", crowns))
	}
	if badge != "" {
		parts = append(parts, "badge: "+strings.ReplaceAll(strings.TrimPrefix(badge, "badge_"), "_", " "))
	}
	if chromaPicks > 0 {
		parts = append(parts, fmt.Sprintf("%d Chroma Pick token(s)", chromaPicks))
	}
	if heroBoxes > 0 {
		parts = append(parts, fmt.Sprintf("%d Hero Box(es)", heroBoxes))
	}
	return strings.Join(parts, " · ")
}

func (s *webServer) missionHeroOptions(mission campaigns.Mission) []webHeroOption {
	options := []webHeroOption{}
	for _, hero := range s.heroOptions() {
		if hero.Unlocked && (len(mission.PlayerHeroRestriction) == 0 || containsHeroID(mission.PlayerHeroRestriction, hero.ID)) {
			options = append(options, hero)
		}
	}
	return options
}

func (s *webServer) chooseWebCPUCommit() ([]int, error) {
	if mission, ok := s.campaignMission(); ok {
		return campaigns.ChooseCommit(s.session.state, mission, s.rng)
	}
	return ai.ChooseCommit(s.session.state, s.session.actor, webDifficulty(s.session.mode), s.rng)
}

func (s *webServer) chooseWebCPUAttack() (int, error) {
	if mission, ok := s.campaignMission(); ok {
		return campaigns.ChooseAttack(s.session.state, mission, s.session.exchange, s.rng)
	}
	return ai.ChoosePoolIndex(s.session.state, s.session.attacker, true, nil, webDifficulty(s.session.mode), s.rng)
}

func (s *webServer) chooseWebCPUDefense() (int, error) {
	if mission, ok := s.campaignMission(); ok {
		return campaigns.ChooseDefense(s.session.state, mission, s.session.exchange, s.session.attackCard, s.rng)
	}
	return ai.ChoosePoolIndex(s.session.state, s.session.actor, false, &s.session.attackCard, webDifficulty(s.session.mode), s.rng)
}

func (s *webServer) playWebExchange(defendIndex int) (game.Card, game.Card, game.Outcome, error) {
	session := s.session
	if mission, ok := s.campaignMission(); ok {
		return campaigns.PlayExchange(&session.state, mission, session.exchange, session.attacker, session.attackIndex, defendIndex)
	}
	defender, _ := session.state.Player(1 - session.attacker)
	defenseCard := defender.Pool[defendIndex]
	outcome, err := game.PlayExchange(&session.state, session.attacker, session.attackIndex, 1-session.attacker, defendIndex)
	return session.attackCard, defenseCard, outcome, err
}

func (s *webServer) campaignMission() (campaigns.Mission, bool) {
	if s.session == nil || s.session.campaign == nil {
		return campaigns.Mission{}, false
	}
	campaign, ok := campaigns.ByID(s.session.campaign.ID)
	if !ok {
		return campaigns.Mission{}, false
	}
	return findCampaignMission(campaign, s.session.campaign.Index)
}

func (s *webServer) startWebMission(w http.ResponseWriter, r *http.Request, input actionRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.prepareWebMission(input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}

func (s *webServer) prepareWebMission(input actionRequest) error {
	p := s.playerProfile
	if p == nil || !campaigns.CanPlayMission(p, input.CampaignID, input.MissionIndex) {
		return errors.New("complete the previous mission and unlock this campaign first")
	}
	campaign, _ := campaigns.ByID(input.CampaignID)
	mission, ok := findCampaignMission(campaign, input.MissionIndex)
	if !ok {
		return errors.New("mission not found")
	}
	heroID := input.PlayerHeroID
	if heroID == "" && len(mission.PlayerHeroRestriction) > 0 {
		heroID = mission.PlayerHeroRestriction[0]
	}
	state, cpuID, err := campaigns.PrepareMission(p, mission, heroID, s.rng)
	if err != nil {
		return err
	}
	if heroID == "" {
		heroID = p.DefaultHeroID
	}
	replay := missionIsComplete(p.Campaigns[campaign.ID], mission.Index)
	if !replay {
		if _, err := refreshEnergy(p, time.Now()); err != nil {
			return err
		}
		if err := energy.Spend(p, campaignMissionEnergyCost(p, mission), time.Now()); err != nil {
			return err
		}
	}
	if err := campaigns.Start(p, campaign.ID); err != nil {
		return err
	}
	white, _ := state.PlayerAtSeat(game.SeatX)
	s.session = &webSession{
		initialAchievements: achievementLevels(p),
		state:               state, mode: "easy", whitePlayer: white, phase: "commit", actor: 0, winner: -1,
		playerHeroID: heroID, opponentHeroID: cpuID,
		message:  "Choose three cards for your round pile. " + mission.Hint,
		campaign: &webCampaignContext{ID: campaign.ID, Index: mission.Index, Name: mission.Name, Hint: mission.Hint, Replay: replay, RewardMarks: mission.RewardMarks},
	}
	return p.Save()
}

func (s *webServer) finishWebMission() {
	session := s.session
	p := s.playerProfile
	first := game.ScorePlayer(session.state.P1, session.state.Bounty.Suit, session.state.Modifiers)
	second := game.ScorePlayer(session.state.P2, session.state.Bounty.Suit, session.state.Modifiers)
	won, tied := first.Total > second.Total, first.Total == second.Total
	session.phase, session.winner = "gameover", 1
	if won {
		session.winner = 0
	}
	result := game.MatchResult{Won: won, Tied: tied, PlayerSeat: session.state.P1.Seat, PlayerScore: first.Total, OpponentScore: second.Total,
		Games: []game.MatchGame{{Won: won, PlayerSeat: session.state.P1.Seat, PlayerScore: first.Total, OpponentScore: second.Total,
			PlayerJail: session.state.P1.Jail, OpponentJail: session.state.P2.Jail, PlayerCapturedByRound: session.state.P1.CapturedByRound,
			OpponentCapturedByRound: session.state.P2.CapturedByRound, Modifiers: session.state.Modifiers, BountySuit: session.state.Bounty.Suit, Clashes: session.state.Clashes}},
	}
	reward := int64(0)
	message := "Mission not completed. Review the hint and retry."
	if session.campaign.Replay {
		message = "Practice complete. No energy was spent and no progression rewards were awarded."
	} else if won {
		outcome, err := campaigns.CompleteMission(p, session.campaign.ID, session.campaign.Index, skills.AggregateBonuses(p).CampaignMarksPercent)
		if err != nil {
			session.profileError = err.Error()
		} else {
			reward = outcome.MissionMarks + outcome.CompletionMarks
			message = fmt.Sprintf("Mission complete! +%d Marks.", reward)
			if outcome.CampaignCompleted {
				message += " Campaign complete: heroes and completion rewards have been added to your commander."
			}
		}
	}
	if !session.campaign.Replay {
		updateAchievementStats(p, result, economy.ModeCPUEasy)
		recordHeroProgress(p, result, session.playerHeroID, session.opponentHeroID)
		switch {
		case won:
			p.TotalWins++
		case tied:
			p.TotalDraws++
		default:
			p.TotalLosses++
		}
		ctx := achievements.NewMatchContext(p, result, economy.ModeIDCPUEasy, result.PlayerSeat.String())
		ctx.Now = time.Now()
		for _, event := range achievements.EvaluateAll(ctx) {
			applyUnlockEvent(p, event)
		}
		p.RecordMatch(buildMatchRecord(ctx, economy.ModeCPUEasy, reward, ctx.Now))
	}
	session.campaign.Result, session.message = message, message
	session.payout = &webMatchPayout{PlayerScore: first.Total, PlayerWon: won, TotalMarks: reward, WinMultiplier: 1}
	session.achievementUpdates = s.matchAchievementUpdates()
	if err := p.Save(); err != nil {
		session.profileError = err.Error()
	}
}
