package app

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/tugwar"
)

type tugWarSeriesProgress struct {
	Game       int `json:"game"`
	PlayerWins int `json:"playerWins"`
	CPUWins    int `json:"cpuWins"`
}

type webTugWarPlayer struct {
	Hero      string      `json:"hero"`
	HeroID    string      `json:"heroId"`
	You       bool        `json:"you"`
	CPU       bool        `json:"cpu"`
	Hand      []game.Card `json:"hand,omitempty"`
	HandCount int         `json:"handCount"`
	Pushes    int         `json:"pushes"`
}

type webTugWarState struct {
	BountySuit     *game.Suit                   `json:"bountySuit,omitempty"`
	BountyRevealed bool                         `json:"bountyRevealed"`
	S              int                          `json:"s"`
	F              int                          `json:"f"`
	B              int                          `json:"b"`
	Round          int                          `json:"round"`
	Current        int                          `json:"current"`
	Viewer         int                          `json:"viewer"`
	DeckCount      int                          `json:"deckCount"`
	Hand           []game.Card                  `json:"hand"`
	Players        [2]webTugWarPlayer           `json:"players"`
	Piles          [tugwar.NumPiles]tugwar.Pile `json:"piles"`
	PileValues     [tugwar.NumPiles]int         `json:"pileValues"`
	ValuesRevealed bool                         `json:"valuesRevealed"`
	CanAct         bool                         `json:"canAct"`
	CanPlay        bool                         `json:"canPlay"`
	CanFlip        bool                         `json:"canFlip"`
	CanPush        [tugwar.NumPiles]bool        `json:"canPush"`
	CanPass        bool                         `json:"canPass"`
	Log            []tugwar.ActionLogEntry      `json:"log"`
	Result         *tugwar.Result               `json:"result,omitempty"`
	MissionName    string                       `json:"missionName,omitempty"`
	MissionHint    string                       `json:"missionHint,omitempty"`
	SeriesGame     int                          `json:"seriesGame,omitempty"`
	SeriesGames    int                          `json:"seriesGames,omitempty"`
	PlayerWins     int                          `json:"playerWins,omitempty"`
	CPUWins        int                          `json:"cpuWins,omitempty"`
	WinsNeeded     int                          `json:"winsNeeded,omitempty"`
	SeriesComplete bool                         `json:"seriesComplete,omitempty"`
	SeriesWon      bool                         `json:"seriesWon,omitempty"`
}

func (s *webServer) startWebTugWar(mode, playerHeroID, opponentHeroID string, playerHero, opponentHero heroes.Hero) error {
	match, err := tugwar.NewGame(playerHero, opponentHero, s.rng)
	if err != nil {
		return fmt.Errorf("start Tug War game: %w", err)
	}
	s.session = &webSession{
		mode: mode, variant: "tug-war", tugWar: match, phase: "tug-war-play",
		actor: match.State.Current, winner: -1,
		playerHeroID: playerHeroID, opponentHeroID: opponentHeroID,
		message:             "Your turn. Choose a card, flip the deck, or spend a push.",
		initialAchievements: achievementLevels(s.playerProfile),
	}
	return nil
}

func (s *webServer) prepareTugWarCampaignMission(p *profile.Profile, campaign campaigns.Campaign,
	mission campaigns.Mission, playerHeroID string, playerHero heroes.Hero, tuning heroes.Tuning,
) error {
	if mission.TugWar == nil {
		return fmt.Errorf("Tug War campaign mission %d has no setup", mission.Index)
	}
	if s.rng == nil {
		s.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	progress := &tugWarSeriesProgress{Game: 1}
	if mission.TugWar.SeriesGames <= 0 {
		progress = nil
	}
	opponentHeroID := mission.AIHero
	match, opponentHeroID, err := s.newTugWarCampaignGame(
		playerHeroID, opponentHeroID, playerHero, tuning, *mission.TugWar, progress,
	)
	if err != nil {
		return err
	}
	if _, err := refreshEnergy(p, time.Now()); err != nil {
		return err
	}
	if cost := campaignMissionEnergyCost(mission); cost > 0 {
		if err := energy.Spend(p, cost, time.Now()); err != nil {
			return err
		}
	}
	replay := missionIsComplete(p.Campaigns[campaign.ID], mission.Index)
	if err := campaigns.Start(p, campaign.ID); err != nil {
		return err
	}
	s.session = &webSession{
		mode: "easy", variant: "tug-war", tugWar: match, tugWarSeries: progress,
		phase: "tug-war-play", actor: match.State.Current, winner: -1,
		playerHeroID: playerHeroID, opponentHeroID: opponentHeroID,
		conductorTunings:    [2]heroes.Tuning{tuning},
		message:             "Your turn. Choose a card, flip the deck, or spend a push.",
		initialAchievements: achievementLevels(p),
		campaign: &webCampaignContext{
			ID: campaign.ID, Index: mission.Index, Name: mission.Name, Hint: mission.Hint,
			Variant: mission.Variant, Replay: replay,
			HasNextMission: campaignHasPlayableMission(campaign, mission.Index+1),
			RewardMarks:    mission.RewardMarks,
		},
	}
	if progress != nil {
		s.session.message = fmt.Sprintf("Game 1 of %d. Your turn.", mission.TugWar.SeriesGames)
	}
	if err := s.captureRestartSnapshot(); err != nil {
		return err
	}
	return p.Save()
}

func (s *webServer) newTugWarCampaignGame(playerHeroID, opponentHeroID string, playerHero heroes.Hero,
	tuning heroes.Tuning, mission tugwar.CampaignMission, progress *tugWarSeriesProgress,
) (*tugwar.Game, string, error) {
	if progress != nil {
		gameIndex := progress.Game - 1
		if mission.MirrorHero {
			opponentHeroID = playerHeroID
		} else if gameIndex >= 0 && gameIndex < len(mission.SeriesCPUHeroes) {
			opponentHeroID = mission.SeriesCPUHeroes[gameIndex]
		}
		if gameIndex >= 0 && gameIndex < len(mission.SeriesPolicies) {
			mission.CPUPolicy = mission.SeriesPolicies[gameIndex]
		}
	}
	opponentHero, ok := heroes.ByID(opponentHeroID)
	if !ok {
		return nil, "", fmt.Errorf("unknown Tug War campaign CPU hero %q", opponentHeroID)
	}
	if mission.MirrorHero {
		opponentHero = playerHero
	}
	match, err := tugwar.NewCampaignGame(playerHero, opponentHero, s.rng, mission)
	if err != nil {
		return nil, "", fmt.Errorf("start Tug War campaign game: %w", err)
	}
	return match, opponentHeroID, nil
}

func (s *webServer) nextTugWarCampaignGame() error {
	session := s.session
	if session == nil || session.campaign == nil || session.tugWar == nil ||
		session.tugWar.Mission == nil || session.tugWarSeries == nil ||
		session.phase != "gameover" {
		return errors.New("no Tug War campaign game is ready to continue")
	}
	mission := session.tugWar.Mission
	progress := session.tugWarSeries
	if progress.Game >= mission.SeriesGames || progress.PlayerWins >= mission.SeriesWinsNeeded ||
		progress.CPUWins >= mission.SeriesWinsNeeded {
		return errors.New("the Tug War campaign series is already complete")
	}
	playerHero, ok := heroes.ByID(session.playerHeroID)
	if !ok {
		return fmt.Errorf("unknown player hero %q", session.playerHeroID)
	}
	playerHero = applyWebHeroTuning(playerHero, session.conductorTunings[0])
	progress.Game++
	match, opponentHeroID, err := s.newTugWarCampaignGame(
		session.playerHeroID, session.opponentHeroID, playerHero,
		session.conductorTunings[0], *mission, progress,
	)
	if err != nil {
		progress.Game--
		return err
	}
	session.tugWar, session.opponentHeroID = match, opponentHeroID
	session.phase, session.actor, session.winner = "tug-war-play", match.State.Current, -1
	session.message = fmt.Sprintf("Game %d of %d. Series: %d–%d. Your turn.",
		progress.Game, mission.SeriesGames, progress.PlayerWins, progress.CPUWins)
	session.campaign.Result = ""
	session.undoSnapshot = nil
	return s.captureRestartSnapshot()
}

func (s *webServer) applyTugWarAction(request actionRequest) error {
	session := s.session
	if session == nil || session.tugWar == nil || session.phase != "tug-war-play" ||
		session.tugWar.State.Over {
		return errors.New("Tug War is not accepting actions")
	}
	if s.onlineViewer {
		if session.actor != s.viewerPlayer {
			return errors.New("it is not your Tug War turn")
		}
	} else if session.actor != 0 || isAI(session.mode, session.actor) {
		return errors.New("it is not your Tug War turn")
	}

	action := tugwar.Action{}
	switch request.Action {
	case "tug-war-play":
		action = tugwar.Action{Kind: tugwar.ActionPlayCard, CardIndex: request.Index, PileIndex: request.Slot}
	case "tug-war-flip":
		action = tugwar.Action{Kind: tugwar.ActionFlipDeck, PileIndex: request.Slot}
	case "tug-war-push":
		action = tugwar.Action{Kind: tugwar.ActionSpendPush, PileIndex: request.Slot}
	case "tug-war-pass":
		action = tugwar.Action{Kind: tugwar.ActionPass}
	default:
		return errors.New("unknown Tug War action")
	}
	if err := session.tugWar.Apply(action); err != nil {
		return err
	}
	s.updateTugWarSession()
	return nil
}

func (s *webServer) updateTugWarSession() {
	session := s.session
	session.actor = session.tugWar.State.Current
	if session.tugWar.State.Over {
		session.phase = "gameover"
		session.winner = session.tugWar.State.Winner
		session.tiebreak = session.tugWar.State.Tiebreak
		switch {
		case session.winner < 0:
			session.message = "Tug War ends in a draw."
		case session.mode == "hvh":
			session.message = fmt.Sprintf("Player %d wins Tug War.", session.winner+1)
		case session.winner == 0:
			session.message = "You win Tug War."
		default:
			session.message = "The CPU wins Tug War."
		}
		if session.campaign != nil {
			s.finishTugWarCampaignGame()
		} else {
			s.awardLocalMultiplayerPayout(session.tugWar.Result().Scores[0])
		}
		return
	}
	session.phase = "tug-war-play"
	if session.mode == "hvh" {
		session.message = fmt.Sprintf("Player %d, choose your move.", session.actor+1)
	} else if session.actor == 0 {
		session.message = "Your turn. Choose a card, flip the deck, or spend a push."
	} else {
		session.message = "The CPU is choosing its move."
	}
}

func (s *webServer) advanceTugWarAI() error {
	session := s.session
	if session == nil || session.variant != "tug-war" || session.tugWar == nil ||
		session.phase == "gameover" || !isAI(session.mode, session.actor) {
		return nil
	}
	action := session.tugWar.ChooseCPUAction()
	if session.tugWar.Mission != nil {
		action = session.tugWar.ChooseCampaignCPUAction(s.rng)
	}
	if err := session.tugWar.Apply(action); err != nil {
		return fmt.Errorf("Tug War CPU action: %w", err)
	}
	s.updateTugWarSession()
	return nil
}

func (s *webServer) finishTugWarCampaignGame() {
	session := s.session
	if session.tugWarSeries == nil || session.tugWar.Mission == nil {
		s.finishSpecialCampaignMission(session.winner == 0)
		return
	}
	progress, mission := session.tugWarSeries, session.tugWar.Mission
	switch session.winner {
	case 0:
		progress.PlayerWins++
	case 1:
		progress.CPUWins++
	}
	seriesDone := progress.PlayerWins >= mission.SeriesWinsNeeded ||
		progress.CPUWins >= mission.SeriesWinsNeeded || progress.Game >= mission.SeriesGames
	record := fmt.Sprintf("Series: %d–%d after game %d of %d.",
		progress.PlayerWins, progress.CPUWins, progress.Game, mission.SeriesGames)
	if !seriesDone {
		session.campaign.Result = record + " Continue to the next game."
		session.message = session.campaign.Result
		if s.playerProfile != nil {
			if err := s.playerProfile.Save(); err != nil {
				session.profileError = err.Error()
			}
		}
		return
	}
	won := progress.PlayerWins >= mission.SeriesWinsNeeded
	s.finishSpecialCampaignMission(won)
	if session.campaign.Result != "" {
		session.campaign.Result += " " + record
		session.message = session.campaign.Result
	}
}

func tugWarWebView(server *webServer, session *webSession) *webTugWarState {
	if session.tugWar == nil {
		return nil
	}
	state := &session.tugWar.State
	viewer := 0
	if server.onlineViewer {
		viewer = server.viewerPlayer
	}
	canAct := !state.Over && state.Current == viewer &&
		(server.onlineViewer || !isAI(session.mode, state.Current))
	bountyRevealed := state.BountyRevealed || state.Over
	view := &webTugWarState{
		S: state.S, F: state.F, B: state.B, BountyRevealed: bountyRevealed,
		Round: state.Round, Current: state.Current, Viewer: viewer,
		DeckCount: len(state.Deck), Hand: append([]game.Card(nil), state.Hands[viewer]...),
		CanAct: canAct, CanPlay: canAct && len(state.Hands[viewer]) > 0 && session.tugWar.HasLegalAction(viewer),
		CanFlip: canAct && len(state.Deck) > 0, CanPass: canAct && session.tugWar.CanPass(),
		Log:            append([]tugwar.ActionLogEntry(nil), session.tugWar.Log...),
		ValuesRevealed: bountyRevealed,
	}
	if bountyRevealed {
		suit := state.BountySuit
		view.BountySuit = &suit
	}
	for pileIndex, pile := range state.Piles {
		view.Piles[pileIndex] = pile
		view.Piles[pileIndex].Cards = append([]game.Card(nil), pile.Cards...)
		if pile.Hidden && !state.Over && len(view.Piles[pileIndex].Cards) > 0 {
			view.Piles[pileIndex].Cards[len(view.Piles[pileIndex].Cards)-1] = game.Card{}
		} else if state.Over {
			view.Piles[pileIndex].Hidden = false
		}
		if bountyRevealed {
			view.PileValues[pileIndex] = tugwar.PileValue(pile, state.BountySuit, state.B)
		} else {
			view.PileValues[pileIndex] = len(pile.Cards)
		}
	}
	if session.campaign != nil {
		view.MissionName, view.MissionHint = session.campaign.Name, session.campaign.Hint
	}
	if session.tugWarSeries != nil && session.tugWar.Mission != nil {
		view.SeriesGame = session.tugWarSeries.Game
		view.SeriesGames = session.tugWar.Mission.SeriesGames
		view.PlayerWins = session.tugWarSeries.PlayerWins
		view.CPUWins = session.tugWarSeries.CPUWins
		view.WinsNeeded = session.tugWar.Mission.SeriesWinsNeeded
		view.SeriesComplete = state.Over && (view.PlayerWins >= view.WinsNeeded ||
			view.CPUWins >= view.WinsNeeded || view.SeriesGame >= view.SeriesGames)
		view.SeriesWon = view.SeriesComplete && view.PlayerWins >= view.WinsNeeded
	}
	for player := range view.Players {
		heroID := session.opponentHeroID
		if player == 0 {
			heroID = session.playerHeroID
		}
		view.Players[player] = webTugWarPlayer{
			Hero: heroName(heroID), HeroID: heroID, You: player == viewer,
			CPU:       !server.onlineViewer && isAI(session.mode, player),
			HandCount: len(state.Hands[player]), Pushes: state.Pushes[player],
		}
		if player == viewer || state.Over {
			view.Players[player].Hand = append([]game.Card(nil), state.Hands[player]...)
		}
	}
	for pileIndex := range view.CanPush {
		view.CanPush[pileIndex] = canAct && session.tugWar.CanSpendPush(viewer, pileIndex)
	}
	if state.Over {
		result := session.tugWar.Result()
		view.Result = &result
	}
	return view
}
