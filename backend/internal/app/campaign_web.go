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
	"prisonersofwar/internal/ladder"
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
	RewardCrowns   int64  `json:"rewardCrowns,omitempty"`
	OpenHand       bool   `json:"openHand,omitempty"`
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
		if mission.Abandon != nil {
			if mission.Abandon.Rounds > 1 || mission.Abandon.OpponentAlgo != "" {
				return name, fmt.Sprintf("%s %d pairs per player, %d round(s); opponent: %s.",
					description, mission.Abandon.PairCount, mission.Abandon.Rounds,
					strings.ReplaceAll(strings.ToLower(mission.Abandon.OpponentAlgo), "_", " ")), mission.Target, mission.Abandon.BoardSize
			}
			return name, fmt.Sprintf("%d pairs · %d board cards · Target score: %d.", mission.AbandonPairs, mission.AbandonBoardCards, mission.Target), mission.Target, mission.AbandonBoardCards
		}
	case "puzzler":
		if puzzle, ok := puzzleByID(mission.VariantID); ok {
			return puzzle.Title, puzzle.Hint, puzzle.Target, puzzle.CardCount
		}
	case "poker":
		if mission.PokerRules != nil {
			description := fmt.Sprintf("%s Par: %d.", mission.Hint, mission.PokerRules.Par)
			if mission.PokerRules.StreakCount > 0 {
				description = fmt.Sprintf("%s Par: %d points for %d consecutive games.",
					mission.Hint, mission.PokerRules.StreakScore, mission.PokerRules.StreakCount)
			}
			return mission.Name, description, mission.PokerRules.Par, 0
		}
	case "rummy":
		if mission.Rummy != nil {
			goal := fmt.Sprintf("Score at least %d.", mission.Rummy.Target)
			if mission.Rummy.Goal == "win" {
				goal = "Win the hand."
			} else if mission.Rummy.Goal == "match" {
				goal = fmt.Sprintf("Win the %d-hand match.", mission.Rummy.MatchHands)
			}
			return mission.Name, fmt.Sprintf("%s %s", mission.Hint, goal), mission.Rummy.Target, 0
		}
	case "ladder":
		if mission.Ladder != nil {
			wild := fmt.Sprintf("%d", mission.Ladder.WildFace)
			if mission.Ladder.WildFace == 0 {
				wild = "random"
			}
			bounty := mission.Ladder.BountySuit.String()
			if mission.Ladder.BountySuit < game.Shield || mission.Ladder.BountySuit > game.Blade {
				bounty = "random"
			}
			return mission.Name, fmt.Sprintf("%s Wild face: %s. Bounty suit: %s. Hand sizes: %d vs. %d.",
				mission.Hint, wild, bounty, mission.Ladder.PlayerSize, mission.Ladder.CPUSize), 0, 0
		}
	}
	return name, description, 0, 0
}

func campaignRecommendedHeroes(mission campaigns.Mission) []string {
	if mission.Variant != "poker" || mission.PokerRules == nil {
		return nil
	}
	recommended := make([]string, 0, len(mission.PokerRules.RecommendedHeroes))
	for _, heroID := range mission.PokerRules.RecommendedHeroes {
		recommended = append(recommended, heroName(heroID))
	}
	return recommended
}

func heroOption(hero heroes.Hero, unlocked bool) webHeroOption {
	return webHeroOption{ID: hero.ID, Name: hero.Name, Unlocked: unlocked, Flavor: hero.Flavor,
		Bounty: hero.Bounty, Shift: hero.Shift, Favored: hero.Favored, Draft: hero.Draft,
		DraftFlip: hero.DraftFlip, Flip: hero.Flip, Unlock: hero.Unlock}
}

func achievementRewardDescription(
	marks, crowns int64, badge string, chromaPicks, heroBoxes,
	woodenBoxes, silverBoxes, goldBoxes, skillPoints,
	undoTokens, restartTokens, skipTokens, swapTokens int,
) string {
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
	if woodenBoxes > 0 {
		parts = append(parts, fmt.Sprintf("%d Wooden Box(es)", woodenBoxes))
	}
	if silverBoxes > 0 {
		parts = append(parts, fmt.Sprintf("%d Silver Box(es)", silverBoxes))
	}
	if goldBoxes > 0 {
		parts = append(parts, fmt.Sprintf("%d Gold Box(es)", goldBoxes))
	}
	if skillPoints > 0 {
		parts = append(parts, fmt.Sprintf("%d Skill Tree Point(s)", skillPoints))
	}
	if undoTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d Undo token(s)", undoTokens))
	}
	if restartTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d Restart token(s)", restartTokens))
	}
	if skipTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d Skip Mission token(s)", skipTokens))
	}
	if swapTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d Swap Card token(s)", swapTokens))
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
	if _, err := refreshEnergy(p, time.Now()); err != nil {
		return err
	}
	if cost := campaignMissionEnergyCost(mission); cost > 0 {
		if err := energy.Spend(p, cost, time.Now()); err != nil {
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
	replay := missionIsComplete(p.Campaigns[campaign.ID], mission.Index)
	s.session = &webSession{
		initialAchievements: achievementLevels(p),
		state:               state, mode: "easy", variant: "main", whitePlayer: white, phase: phase, actor: actor, winner: -1,
		playerHeroID: heroID, opponentHeroID: cpuID,
		message: message,
		campaign: &webCampaignContext{
			ID: campaign.ID, Index: mission.Index, Name: mission.Name, Hint: mission.Hint,
			Variant: missionVariant(mission), Replay: replay, HasNextMission: campaignHasPlayableMission(campaign, mission.Index+1),
			RewardMarks:  mission.RewardMarks,
			RewardCrowns: mission.RewardCrowns,
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
	heroID := input.PlayerHeroID
	if len(mission.PlayerHeroRestriction) > 0 {
		if heroID == "" {
			heroID = mission.PlayerHeroRestriction[0]
		} else if !containsHeroID(mission.PlayerHeroRestriction, heroID) {
			return fmt.Errorf("mission %d requires the %s hero", mission.Index, heroName(mission.PlayerHeroRestriction[0]))
		}
	}
	if heroID == "" {
		heroID = s.requestedHeroID("")
	}
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
	var abandonDuel *abandonDuelGame
	var poker *pokerSession
	var rummy *rummySession
	var ladderGame *ladder.LadderGame
	opponentHeroID := mission.AIHero
	switch mission.Variant {
	case "tug-war":
		return s.prepareTugWarCampaignMission(p, campaign, mission, heroID, playerHero, tuning)
	case "rummy":
		if mission.Rummy == nil {
			return errors.New("Rummy mission rules are unavailable")
		}
		if opponentHeroID == "random" {
			available := heroes.All()
			opponentHeroID = available[s.rng.Intn(len(available))].ID
		}
		opponentHero, exists := heroes.ByID(opponentHeroID)
		if !exists {
			return fmt.Errorf("unknown Rummy campaign CPU hero %q", opponentHeroID)
		}
		rummy, err = newRummyCampaignSession(playerHero, opponentHero, mission.Rummy, s.rng)
		if err != nil {
			return err
		}
	case "ladder":
		if mission.Ladder == nil {
			return errors.New("Ladder mission rules are unavailable")
		}
		if opponentHeroID == "random" {
			if mission.Index == 15 && input.OpponentHeroID != "" {
				opponentHeroID = input.OpponentHeroID
			} else {
				available := heroes.All()
				opponentHeroID = available[s.rng.Intn(len(available))].ID
			}
		}
		opponentHero, exists := heroes.ByID(opponentHeroID)
		if !exists {
			return fmt.Errorf("unknown Ladder campaign CPU hero %q", opponentHeroID)
		}
		rules := mission.Ladder
		stats := (*ladder.SharedStats)(nil)
		if rules.SharedStats != nil {
			stats = &ladder.SharedStats{
				Bounty: rules.SharedStats.Bounty, Draft: rules.SharedStats.Draft,
				Shift: rules.SharedStats.Shift, Favor: rules.SharedStats.Favor,
			}
		}
		setup := ladder.CampaignSetup{
			PlayerHand: rules.PlayerHand, CPUHand: rules.CPUHand,
			PlayerSize: rules.PlayerSize, CPUSize: rules.CPUSize,
			WildFace: rules.WildFace, BountySuit: rules.BountySuit, SharedStats: stats,
		}
		ladderGame, err = ladder.NewCampaignGame(playerHero, opponentHero, setup, s.rng)
		if err != nil {
			return fmt.Errorf("start Ladder campaign mission: %w", err)
		}
	case "puzzler":
		definition, exists := puzzleByID(mission.VariantID)
		if !exists {
			return fmt.Errorf("unknown puzzle %q", mission.VariantID)
		}
		puzzle = newPuzzleSession(definition, playerHero)
		puzzle.DrawPile = replacementDeck(definition.Cards, s.rng)
	case "abandon":
		if mission.Abandon == nil {
			return errors.New("Abandon campaign rules are unavailable")
		}
		rules := mission.Abandon
		if rules.Rounds > 1 || rules.OpponentAlgo != "" {
			opponentHeroID = "recruit"
			opponentHero, exists := heroes.ByID(opponentHeroID)
			if !exists {
				return errors.New("Abandon campaign opponent hero is unavailable")
			}
			setup := abandonDuelSetup{
				Board: rules.Board, PlayerHand: rules.PlayerHand, CPUHand: rules.CPUHand,
				BoardSize: rules.BoardSize, HandSize: rules.HandSize, PairCount: rules.PairCount,
				Rounds: rules.Rounds, BountySuits: rules.BountySuits,
				MirrorHands: rules.MirrorHands, Snake: rules.Snake, OpponentAlgo: rules.OpponentAlgo,
			}
			abandonDuel, err = newAbandonDuelGameWithSetup(playerHero, opponentHero, setup, s.rng)
			if err != nil {
				return fmt.Errorf("start Abandon campaign mission: %w", err)
			}
		} else {
			abandon, err = newAbandonSessionForCampaign(
				playerHero, s.rng, rules.Board, rules.PlayerHand, game.Suit(rules.BountySuits[0]),
				rules.PairCount, rules.Target, rules.MinCaptured, rules.MaxAbandoned,
				rules.Goal, rules.TargetCard,
			)
			if err != nil {
				return err
			}
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

	if _, err := refreshEnergy(p, time.Now()); err != nil {
		return err
	}
	if cost := campaignMissionEnergyCost(mission); cost > 0 {
		if err := energy.Spend(p, cost, time.Now()); err != nil {
			return err
		}
	}
	if err := campaigns.Start(p, campaign.ID); err != nil {
		return err
	}
	message, phase := "Choose an attacker.", "puzzle-pick"
	actor := 0
	var ladderReadyAt time.Time
	if abandonDuel != nil {
		message, phase = "Flip a face-down board card.", abandonDuel.phase()
		actor = abandonDuel.ActivePlayer
	} else if abandon != nil {
		message, phase = "Flip a face-down board card.", "abandon-reveal"
	} else if poker != nil {
		message, phase = "Place cards into three poker hands or redraw one card.", "poker-play"
	} else if rummy != nil {
		phase = "rummy-opening"
		if len(rummy.FaceUp) > 0 {
			phase = "rummy-turn"
		}
		message = "Your turn. Take the face-up card or draw two cards."
	} else if ladderGame != nil {
		state := ladderGame.State()
		actor = int(state.Current)
		if state.OpeningPending && isAI("easy", int(state.Current)) {
			phase, message = "ladder-thinking", "The CPU is choosing the opening card."
			ladderReadyAt = time.Now().Add(time.Second)
		} else if state.OpeningPending {
			phase, message = "ladder-opening-reveal", "Review your hand before the opening card is played."
			ladderReadyAt = time.Now().Add(3 * time.Second)
		} else {
			phase, message = "ladder-play", "Choose a legal card to play."
		}
	}
	mode := "easy"
	if mission.Ladder != nil && mission.Ladder.CPUPolicy == "hard" {
		mode = "hard"
	} else if mission.Ladder != nil {
		mode = "normal"
	}
	variant := mission.Variant
	if abandonDuel != nil {
		variant = "abandon-duel"
	}
	replay := missionIsComplete(p.Campaigns[campaign.ID], mission.Index)
	s.session = &webSession{
		initialAchievements: achievementLevels(p),
		mode:                mode, variant: variant, puzzle: puzzle, abandon: abandon, abandonDuel: abandonDuel,
		poker: poker, rummy: rummy, ladder: ladderGame,
		phase: phase, actor: actor, winner: -1, playerHeroID: heroID, opponentHeroID: opponentHeroID, message: message,
		ladderReadyAt: ladderReadyAt,
		campaign: &webCampaignContext{
			ID: campaign.ID, Index: mission.Index, Name: mission.Name, Hint: mission.Hint,
			Variant: mission.Variant, VariantID: mission.VariantID,
			Replay: replay, HasNextMission: campaignHasPlayableMission(campaign, mission.Index+1),
			RewardMarks: mission.RewardMarks, RewardCrowns: mission.RewardCrowns,
			OpenHand: mission.Ladder != nil && mission.Ladder.OpenHand,
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
	if outcome.MissionCrowns > 0 {
		session.campaign.Result = fmt.Sprintf("Mission complete! +%d Marks, +%d Crowns.",
			outcome.MissionMarks, outcome.MissionCrowns)
	}
	session.campaign.HasNextMission = campaigns.CanPlayMission(p, session.campaign.ID, session.campaign.Index+1)
	if outcome.CampaignCompleted {
		evaluateCampaignCompletionAchievement(p, session.campaign.ID)
		session.campaign.Result += " Campaign complete."
	}
	session.message = session.campaign.Result
	session.achievementUpdates = s.matchAchievementUpdates()
	if err := p.Save(); err != nil {
		session.profileError = err.Error()
	}
}

func (s *webServer) finishAbandonDuel() {
	session := s.session
	if session == nil || session.abandonDuel == nil {
		return
	}
	if session.campaign != nil {
		mission, ok := s.campaignMission()
		if !ok || mission.Abandon == nil {
			session.profileError = "Abandon campaign objective rules are unavailable."
			return
		}
		duel, rules := session.abandonDuel, mission.Abandon
		won := false
		switch rules.Goal {
		case "win", "match_win":
			won = duel.Winner == 0
		case "win_bounty":
			bountyCaptures := 0
			for _, pairing := range duel.Pairings {
				if pairing.Player == 0 && pairing.Captured != nil && pairing.Captured.Suit == duel.Bounty.Suit {
					bountyCaptures++
				}
			}
			won = duel.Winner == 0 && bountyCaptures >= rules.Target
		case "score":
			won = duel.TotalScores[0] >= rules.Target
		case "score_all_suits":
			suits := [3]bool{}
			for _, card := range duel.Players[0].Prisoners {
				suits[int(card.Suit)] = true
			}
			won = duel.TotalScores[0] >= rules.Target && suits[0] && suits[1] && suits[2]
		default:
			session.profileError = fmt.Sprintf("Unknown Abandon campaign objective %q.", rules.Goal)
			return
		}
		s.finishSpecialCampaignMission(won)
		return
	}
	s.awardLocalMultiplayerPayout(session.abandonDuel.TotalScores[0])
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
	rewardCrowns := int64(0)
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
			rewardCrowns = outcome.MissionCrowns
			message = fmt.Sprintf("Mission complete! +%d Marks.", reward)
			if outcome.MissionCrowns > 0 {
				message = fmt.Sprintf("Mission complete! +%d Marks, +%d Crowns.",
					reward, outcome.MissionCrowns)
			}
			if readinessGranted && rewardErr == nil {
				message += " Rations & Readiness: +1 Restart token, +1 Undo token, and +1 Energy."
			}
			if outcome.CampaignCompleted {
				evaluateCampaignCompletionAchievement(p, session.campaign.ID)
				message += " Campaign complete: heroes and completion rewards have been added to your commander."
			}
			if outcome.SkillPointsAwarded > 0 {
				message += fmt.Sprintf(" +%d Skill Tree Point(s).", outcome.SkillPointsAwarded)
			}
		}
	}
	if !session.campaign.Replay {
		recordHeroProgress(p, result, session.playerHeroID, session.opponentHeroID)
		now := time.Now()
		updateAchievementStatsForGameMode(p, result, economy.ModeCPUEasy, session.variant, now)
		switch {
		case won:
			p.TotalWins++
		case tied:
			p.TotalDraws++
		default:
			p.TotalLosses++
		}
		ctx := achievements.NewMatchContext(p, result, economy.ModeIDCPUEasy, result.PlayerSeat.String())
		ctx.Now = now
		ctx.GameMode = session.variant
		for _, event := range achievements.EvaluateAll(ctx) {
			applyUnlockEvent(p, event)
		}
		p.RecordMatch(buildMatchRecord(ctx, economy.ModeCPUEasy, reward, ctx.Now))
	}
	session.campaign.Result, session.message = message, message
	session.payout = &webMatchPayout{
		PlayerScore: first.Total, PlayerWon: won, TotalMarks: reward,
		TotalCrowns: rewardCrowns, WinMultiplier: 1,
	}
	session.achievementUpdates = s.matchAchievementUpdates()
	if err := p.Save(); err != nil {
		session.profileError = err.Error()
	}
}
