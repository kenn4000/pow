package app

import (
	"errors"
	"fmt"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/tugwar"
)

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
	BountySuit game.Suit                    `json:"bountySuit"`
	S          int                          `json:"s"`
	F          int                          `json:"f"`
	B          int                          `json:"b"`
	Round      int                          `json:"round"`
	Current    int                          `json:"current"`
	Viewer     int                          `json:"viewer"`
	DeckCount  int                          `json:"deckCount"`
	Hand       []game.Card                  `json:"hand"`
	Players    [2]webTugWarPlayer           `json:"players"`
	Piles      [tugwar.NumPiles]tugwar.Pile `json:"piles"`
	CanAct     bool                         `json:"canAct"`
	CanPlay    bool                         `json:"canPlay"`
	CanFlip    bool                         `json:"canFlip"`
	CanPush    [tugwar.NumPiles]bool        `json:"canPush"`
	CanPass    bool                         `json:"canPass"`
	Log        []tugwar.ActionLogEntry      `json:"log"`
	Result     *tugwar.Result               `json:"result,omitempty"`
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
	if err := session.tugWar.Apply(action); err != nil {
		return fmt.Errorf("Tug War CPU action: %w", err)
	}
	s.updateTugWarSession()
	return nil
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
	view := &webTugWarState{
		BountySuit: state.BountySuit, S: state.S, F: state.F, B: state.B,
		Round: state.Round, Current: state.Current, Viewer: viewer,
		DeckCount: len(state.Deck), Hand: append([]game.Card(nil), state.Hands[viewer]...),
		Piles: state.Piles, CanAct: canAct, CanPlay: canAct && len(state.Hands[viewer]) > 0 && session.tugWar.HasLegalAction(viewer),
		CanFlip: canAct && len(state.Deck) > 0, CanPass: canAct && session.tugWar.CanPass(),
		Log: append([]tugwar.ActionLogEntry(nil), session.tugWar.Log...),
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
