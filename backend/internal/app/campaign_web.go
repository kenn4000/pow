package app

import (
	"errors"
	"fmt"
	"math/rand"
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
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

type webCampaignContext struct {
	ID             string `json:"id"`
	Index          int    `json:"index"`
	Name           string `json:"name"`
	Hint           string `json:"hint"`
	Variant        string `json:"variant"`
	VariantID      string `json:"variantId,omitempty"`
	Replay         bool   `json:"replay"`
	HasNextMission bool   `json:"hasNextMission"`
	RewardMarks    int64  `json:"rewardMarks"`
	Result         string `json:"result,omitempty"`
}

func campaignVariant(campaign campaigns.Campaign) string {
	if campaign.Variant != "" {
		return campaign.Variant
	}
	return "main"
}

func missionVariant(mission campaigns.Mission) string {
	if mission.Variant != "" {
		return mission.Variant
	}
	return "main"
}

func campaignHasPlayableMission(campaign campaigns.Campaign, index int) bool {
	for _, mission := range campaign.Missions {
		if mission.Index == index {
			return !mission.Disabled
		}
	}
	return false
}

func campaignMissionMetadata(mission campaigns.Mission) (string, string, int, int) {
	name, description := mission.Name, mission.Hint
	switch mission.Variant {
	case "abandon":
		if !mission.Disabled && mission.AbandonPairs > 0 && mission.AbandonBoardCards > 0 && mission.Target > 0 {
			return name, fmt.Sprintf("%d pairs · %d board cards · Target score: %d.", mission.AbandonPairs, mission.AbandonBoardCards, mission.Target), mission.Target, mission.AbandonBoardCards
		}
	case "puzzler":
		if puzzle, ok := puzzleByID(mission.VariantID); ok {
			return puzzle.Title, puzzle.Hint, puzzle.Target, puzzle.CardCount
		}
	case "poker":
		if mission.PokerRules != nil {
			description := mission.Hint
			if mission.PokerRules.StreakCount == 0 {
				description = fmt.Sprintf("%s Par: %d.", description, mission.PokerRules.Par)
			}
			return mission.Name, description, mission.PokerRules.Par, 0
		}
	}
	return name, description, 0, 0
}

func heroOption(hero heroes.Hero, unlocked bool) webHeroOption {
	return webHeroOption{ID: hero.ID, Name: hero.Name, Unlocked: unlocked, Flavor: hero.Flavor,
		Bounty: hero.Bounty, Shift: hero.Shift, Favored: hero.Favored, Draft: hero.Draft,
		DraftFlip: hero.DraftFlip, Flip: hero.Flip, Unlock: hero.Unlock}
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
	if mission.Variant != "" {
		return s.prepareSpecialWebMission(p, campaign, mission, input)
	}
	heroID := input.PlayerHeroID
	if heroID == "" && len(mission.PlayerHeroRestriction) > 0 {
		heroID = mission.PlayerHeroRestriction[0]
	}
	tuning, err := webConductorTuning(p, input.ConductorModifier, input.ConductorAmount)
	if err != nil {
		return err
	}
	state, cpuID, err := campaigns.PrepareMissionWithTuning(p, mission, heroID, tuning, s.rng)
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
		if err := energy.Spend(p, campaignMissionEnergyCost(mission), time.Now()); err != nil {
			return err
		}
	}
	if err := campaigns.Start(p, campaign.ID); err != nil {
		return err
	}
	white, _ := state.PlayerAtSeat(game.SeatX)
	actor, phase := 0, "commit"
	message := "Choose three cards for your round pile. " + mission.Hint
	if state.Draft != nil {
		seat, ok := state.Draft.CurrentSeat()
		if !ok {
			return errors.New("campaign draft has no pending first pick")
		}
		actor, err = state.PlayerAtSeat(seat)
		if err != nil {
			return err
		}
		phase = "draft"
		message = "Choose one card from the public pool."
	}
	s.session = &webSession{
		initialAchievements: achievementLevels(p),
		state:               state, mode: "easy", variant: "main", whitePlayer: white, phase: phase, actor: actor, winner: -1,
		playerHeroID: heroID, opponentHeroID: cpuID,
		message: message,
		campaign: &webCampaignContext{
			ID: campaign.ID, Index: mission.Index, Name: mission.Name, Hint: mission.Hint,
			Variant: missionVariant(mission), Replay: replay, HasNextMission: campaignHasPlayableMission(campaign, mission.Index+1),
			RewardMarks: mission.RewardMarks,
		},
	}
	if state.Draft != nil {
		s.session.message = s.draftTurnMessage(actor)
	}
	if err := s.captureRestartSnapshot(); err != nil {
		return err
	}
	return p.Save()
}

func (s *webServer) prepareSpecialWebMission(p *profile.Profile, campaign campaigns.Campaign, mission campaigns.Mission, input actionRequest) error {
	heroID := s.requestedHeroID(input.PlayerHeroID)
	playerHero, ok := heroes.ByID(heroID)
	if !ok || !s.heroUnlocked(heroID) {
		return fmt.Errorf("player hero %q is not unlocked", heroID)
	}
	tuning, err := webConductorTuning(p, input.ConductorModifier, input.ConductorAmount)
	if err != nil {
		return err
	}
	playerHero = applyWebHeroTuning(playerHero, tuning)
	if s.rng == nil {
		s.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	var puzzle *puzzleSession
	var abandon *abandonSession
	var poker *pokerSession
	switch mission.Variant {
	case "puzzler":
		definition, exists := puzzleByID(mission.VariantID)
		if !exists {
			return fmt.Errorf("unknown puzzle %q", mission.VariantID)
		}
		puzzle = newPuzzleSession(definition, playerHero)
		puzzle.DrawPile = replacementDeck(definition.Cards, s.rng)
	case "abandon":
		var err error
		abandon, err = newAbandonSessionForMission(playerHero, s.rng, mission.AbandonPairs, mission.AbandonBoardCards, mission.Target)
		if err != nil {
			return err
		}
	case "poker":
		if mission.PokerRules == nil {
			return errors.New("Poker mission rules are unavailable")
		}
		var err error
		poker, err = newPokerSession(heroID, playerHero, *mission.PokerRules, s.rng)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported campaign variant %q", mission.Variant)
	}

	replay := missionIsComplete(p.Campaigns[campaign.ID], mission.Index)
	if !replay {
		if _, err := refreshEnergy(p, time.Now()); err != nil {
			return err
		}
		if err := energy.Spend(p, campaignMissionEnergyCost(mission), time.Now()); err != nil {
			return err
		}
	}
	if err := campaigns.Start(p, campaign.ID); err != nil {
		return err
	}
	message, phase := "Choose an attacker.", "puzzle-pick"
	if abandon != nil {
		message, phase = "Flip a face-down board card.", "abandon-reveal"
	} else if poker != nil {
		message, phase = "Place cards into three poker hands or redraw one card.", "poker-play"
	}
	s.session = &webSession{
		initialAchievements: achievementLevels(p),
		mode:                "easy", variant: mission.Variant, puzzle: puzzle, abandon: abandon, poker: poker,
		phase: phase, actor: 0, winner: -1, playerHeroID: heroID, message: message,
		campaign: &webCampaignContext{
			ID: campaign.ID, Index: mission.Index, Name: mission.Name, Hint: mission.Hint,
			Variant: mission.Variant, VariantID: mission.VariantID,
			Replay: replay, HasNextMission: campaignHasPlayableMission(campaign, mission.Index+1),
			RewardMarks: mission.RewardMarks,
		},
	}
	if err := s.captureRestartSnapshot(); err != nil {
		return err
	}
	return p.Save()
}

func (s *webServer) finishSpecialCampaignMission(won bool) {
	session := s.session
	if session == nil || session.campaign == nil {
		return
	}
	p := s.playerProfile
	if session.campaign.Replay {
		session.campaign.Result = "Practice complete. No progression rewards were awarded."
		session.message = session.campaign.Result
		return
	}
	if !won {
		session.campaign.Result = "Mission not completed. Try again."
		session.message = session.campaign.Result
		return
	}
	outcome, err := campaigns.CompleteMission(p, session.campaign.ID, session.campaign.Index, 0)
	if err != nil {
		session.profileError = err.Error()
		return
	}
	session.campaign.Result = fmt.Sprintf("Mission complete! +%d Marks.", outcome.MissionMarks)
	session.campaign.HasNextMission = campaigns.CanPlayMission(p, session.campaign.ID, session.campaign.Index+1)
	if outcome.CampaignCompleted {
		session.campaign.Result += " Campaign complete."
	}
	session.message = session.campaign.Result
	if err := p.Save(); err != nil {
		session.profileError = err.Error()
	}
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
			bonusMarks, readinessGranted, rewardErr := applyCampaignWinSkillRewards(
				p, session.campaign.ID, outcome.MissionMarks+outcome.CompletionMarks, time.Now(),
			)
			if rewardErr != nil {
				session.profileError = rewardErr.Error()
			} else {
				outcome.MissionMarks += bonusMarks
			}
			reward = outcome.MissionMarks + outcome.CompletionMarks
			message = fmt.Sprintf("Mission complete! +%d Marks.", reward)
			if readinessGranted && rewardErr == nil {
				message += " Rations & Readiness: +1 Restart token, +1 Undo token, and +1 Energy."
			}
			if outcome.CampaignCompleted {
				message += " Campaign complete: heroes and completion rewards have been added to your commander."
			}
			if outcome.SkillPointsAwarded > 0 {
				message += fmt.Sprintf(" +%d Skill Tree Point(s).", outcome.SkillPointsAwarded)
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
