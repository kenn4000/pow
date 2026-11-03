package app

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

const rummyTurnLimit = 12

type rummySession struct {
	Parameters     game.RummyParameters
	DrawPile       []game.Card
	PreviousBuried []game.Card `json:"Buried,omitempty"`
	FaceUp         []game.Card
	Hands          [2][]game.Card
	Turns          int
	Results        [2]game.RummyPartition
}

type webRummyState struct {
	Parameters     game.RummyParameters `json:"parameters"`
	Hand           []webCard            `json:"hand,omitempty"`
	OpponentHand   int                  `json:"opponentHand"`
	DrawPileCount  int                  `json:"drawPileCount"`
	FaceUpPile     []game.Card          `json:"faceUpPile"`
	CanDrawFaceUp  bool                 `json:"canDrawFaceUp"`
	CanDrawTwo     bool                 `json:"canDrawTwo"`
	TurnsCompleted int                  `json:"turnsCompleted"`
	TurnLimit      int                  `json:"turnLimit"`
	Results        []webRummyResult     `json:"results,omitempty"`
}

type webRummyResult struct {
	Player    int                 `json:"player"`
	Hand      []game.Card         `json:"hand"`
	Partition game.RummyPartition `json:"partition"`
}

func newRummySession(firstHero, secondHero heroes.Hero, rng *rand.Rand) *rummySession {
	params := game.NewRummyParameters(
		firstHero.Favored+secondHero.Favored,
		firstHero.Shift+secondHero.Shift,
		firstHero.Bounty+secondHero.Bounty,
		firstHero.Draft+secondHero.Draft,
	)
	deck := game.NewDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	session := &rummySession{Parameters: params}
	for player := range session.Hands {
		session.Hands[player] = make([]game.Card, 0, params.HandSize)
		for cardIndex := 0; cardIndex < params.HandSize; cardIndex++ {
			session.Hands[player] = append(session.Hands[player], deck[len(deck)-1])
			deck = deck[:len(deck)-1]
		}
	}
	session.DrawPile = deck
	return session
}

func (s *webServer) startWebRummy(mode, playerHeroID, opponentHeroID string, firstHero, secondHero heroes.Hero) {
	s.session = &webSession{
		mode:                mode,
		variant:             "rummy",
		phase:               "rummy-opening",
		actor:               0,
		winner:              -1,
		playerHeroID:        playerHeroID,
		opponentHeroID:      opponentHeroID,
		rummy:               newRummySession(firstHero, secondHero, s.rng),
		message:             "Player 1 draws two cards, then discards one face-up and buries one.",
		initialAchievements: achievementLevels(s.playerProfile),
	}
}

func (r *rummySession) webView(session *webSession, viewer int, online bool) *webRummyState {
	viewerPlayer := session.actor
	if online {
		viewerPlayer = viewer
	}
	canAct := !online || viewer == session.actor
	result := &webRummyState{
		Parameters:     r.Parameters,
		OpponentHand:   len(r.Hands[1-viewerPlayer]),
		DrawPileCount:  len(r.DrawPile),
		FaceUpPile:     append([]game.Card(nil), r.FaceUp...),
		TurnsCompleted: r.Turns,
		TurnLimit:      rummyTurnLimit,
	}
	canDrawTwo := session.phase == "rummy-opening" || session.phase == "rummy-turn"
	result.CanDrawTwo = canAct && canDrawTwo && r.deckCount() >= 2
	result.CanDrawFaceUp = session.phase == "rummy-turn" &&
		canAct && len(r.FaceUp) > 0 && r.deckCount() >= 1
	if session.phase != "handoff" && session.phase != "gameover" {
		hand := r.Hands[viewerPlayer]
		result.Hand = make([]webCard, len(hand))
		selectable := session.phase == "rummy-discard" && canAct
		for index, card := range hand {
			result.Hand[index] = webCard{
				Card:       card,
				Index:      index,
				Selectable: selectable,
			}
		}
	}
	if session.phase == "gameover" {
		result.Results = make([]webRummyResult, 2)
		for player := range result.Results {
			result.Results[player] = webRummyResult{
				Player:    player,
				Hand:      append([]game.Card(nil), r.Hands[player]...),
				Partition: r.Results[player],
			}
		}
	}
	return result
}

func (s *webServer) applyRummyAction(request actionRequest) error {
	session := s.session
	rummy := session.rummy
	if rummy == nil {
		return errors.New("Rummy state is unavailable")
	}
	if session.phase == "handoff" || session.phase == "gameover" ||
		isAI(session.mode, session.actor) {
		return errors.New("it is not your Rummy turn")
	}
	switch request.Action {
	case "rummy-draw-two":
		if session.phase != "rummy-opening" && session.phase != "rummy-turn" {
			return errors.New("Rummy is not waiting for a draw")
		}
		if err := rummy.drawTwoForPlayer(session.actor); err != nil {
			return err
		}
		session.phase = "rummy-discard"
		session.message = "Choose one card to discard face-up and another to bury."
		return nil
	case "rummy-draw-face-up":
		if session.phase != "rummy-turn" || len(rummy.FaceUp) == 0 {
			return errors.New("there is no face-up card to take")
		}
		if err := rummy.drawFaceUp(session.actor); err != nil {
			return err
		}
		return s.completeRummyTurn()
	case "rummy-discard":
		if session.phase != "rummy-discard" {
			return errors.New("Rummy is not waiting for discards")
		}
		if err := rummy.discard(session.actor, request.FaceUpIndex, request.BuriedIndex); err != nil {
			return err
		}
		return s.completeRummyTurn()
	default:
		return errors.New("unknown Rummy action")
	}
}

func (r *rummySession) drawTwoForPlayer(player int) error {
	if player < 0 || player >= len(r.Hands) {
		return errors.New("invalid Rummy player")
	}
	if r.deckCount() < 2 {
		return errors.New("not enough cards remain to draw")
	}
	for drawn := 0; drawn < 2; drawn++ {
		card, ok := r.drawDeckCard()
		if !ok {
			return errors.New("not enough cards remain to draw")
		}
		r.Hands[player] = append(r.Hands[player], card)
	}
	return nil
}

func (r *rummySession) drawFaceUp(player int) error {
	if player < 0 || player >= len(r.Hands) || len(r.FaceUp) == 0 {
		return errors.New("there is no face-up card to take")
	}
	if r.deckCount() == 0 {
		return errors.New("not enough cards remain to draw")
	}
	replacement, ok := r.drawDeckCard()
	if !ok {
		return errors.New("not enough cards remain to draw")
	}
	lastFaceUp := len(r.FaceUp) - 1
	r.Hands[player] = append(r.Hands[player], r.FaceUp[lastFaceUp])
	r.FaceUp = r.FaceUp[:lastFaceUp]
	r.FaceUp = append(r.FaceUp, replacement)
	return nil
}

func (r *rummySession) discard(player, faceUpIndex, buriedIndex int) error {
	if player < 0 || player >= len(r.Hands) {
		return errors.New("invalid Rummy player")
	}
	hand := r.Hands[player]
	if faceUpIndex < 0 || faceUpIndex >= len(hand) ||
		buriedIndex < 0 || buriedIndex >= len(hand) || faceUpIndex == buriedIndex {
		return errors.New("choose two different cards from your hand")
	}
	faceUp := hand[faceUpIndex]
	buried := hand[buriedIndex]
	remaining := make([]game.Card, 0, len(hand)-2)
	for index, card := range hand {
		if index != faceUpIndex && index != buriedIndex {
			remaining = append(remaining, card)
		}
	}
	r.Hands[player] = remaining
	r.FaceUp = append(r.FaceUp, faceUp)
	r.DrawPile = append(r.DrawPile, game.Card{})
	copy(r.DrawPile[1:], r.DrawPile[:len(r.DrawPile)-1])
	r.DrawPile[0] = buried
	return nil
}

func (r *rummySession) deckCount() int {
	return len(r.DrawPile)
}

func (r *rummySession) drawDeckCard() (game.Card, bool) {
	if len(r.DrawPile) > 0 {
		top := len(r.DrawPile) - 1
		card := r.DrawPile[top]
		r.DrawPile = r.DrawPile[:top]
		return card, true
	}
	return game.Card{}, false
}

func (s *webServer) completeRummyTurn() error {
	session := s.session
	rummy := session.rummy
	rummy.Turns++
	if rummy.Turns >= rummyTurnLimit {
		for player := range rummy.Hands {
			rummy.Results[player] = game.BestRummyPartition(rummy.Hands[player], rummy.Parameters)
		}
		switch {
		case rummy.Results[0].Total > rummy.Results[1].Total:
			session.winner = 0
			session.message = "Player 1 wins the Rummy game."
		case rummy.Results[1].Total > rummy.Results[0].Total:
			session.winner = 1
			session.message = "Player 2 wins the Rummy game."
		default:
			session.winner = -1
			session.message = "The Rummy game is a draw."
		}
		session.phase = "gameover"
		return nil
	}

	session.actor = 1 - session.actor
	if session.mode == "hvh" {
		session.phase = "handoff"
		session.handoffTarget = session.actor
		session.handoffPhase = "rummy-turn"
		session.message = fmt.Sprintf("Pass the screen to Player %d.", session.actor+1)
		return nil
	}
	session.phase = "rummy-turn"
	if session.actor == 0 {
		session.message = "Your turn. Take the face-up card or draw two cards."
	} else {
		session.message = "The CPU is taking its turn."
	}
	return nil
}

func (s *webServer) advanceRummyAI() error {
	session := s.session
	for session != nil && session.rummy != nil && isAI(session.mode, session.actor) {
		switch session.phase {
		case "rummy-opening", "rummy-turn":
			rummy := session.rummy
			if session.phase == "rummy-turn" && len(rummy.FaceUp) > 0 &&
				rummy.deckCount() > 0 &&
				game.ShouldTakeRummyDiscard(rummy.Hands[session.actor],
					rummy.FaceUp[len(rummy.FaceUp)-1], rummy.Parameters) {
				if err := rummy.drawFaceUp(session.actor); err != nil {
					return err
				}
				if err := s.completeRummyTurn(); err != nil {
					return err
				}
				continue
			}
			if err := rummy.drawTwoForPlayer(session.actor); err != nil {
				return err
			}
			if err := s.aiRummyDiscard(); err != nil {
				return err
			}
			if err := s.completeRummyTurn(); err != nil {
				return err
			}
		default:
			return nil
		}
	}
	return nil
}

func (s *webServer) aiRummyDiscard() error {
	rummy := s.session.rummy
	player := s.session.actor
	hand := rummy.Hands[player]
	if len(hand) < 2 {
		return errors.New("CPU has fewer than two cards to discard")
	}
	indices := make([]int, len(hand))
	for index := range indices {
		indices[index] = index
	}
	sort.SliceStable(indices, func(i, j int) bool {
		first := game.RummyMeldProximity(hand[indices[i]], hand)
		second := game.RummyMeldProximity(hand[indices[j]], hand)
		return first < second
	})
	faceUpIndex, buriedIndex := indices[0], indices[1]
	if s.rng.Intn(2) == 0 {
		faceUpIndex, buriedIndex = buriedIndex, faceUpIndex
	}
	return rummy.discard(player, faceUpIndex, buriedIndex)
}
