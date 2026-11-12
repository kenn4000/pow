package app

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"prisonersofwar/internal/campaigns"
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
	LastMoves      [2]rummyLastMove `json:"LastMoves,omitempty"`
	Turns          int
	Results        [2]game.RummyPartition
	Rules          *campaigns.RummyCampaignRules
	TurnLimit      int
	HandNumber     int
	HandScores     [2]int
}

type rummyLastMove struct {
	Action string     `json:"action"`
	Card   *game.Card `json:"card,omitempty"`
}

type webRummyState struct {
	Parameters       game.RummyParameters `json:"parameters"`
	Hand             []webCard            `json:"hand,omitempty"`
	OpponentHand     int                  `json:"opponentHand"`
	DrawPileCount    int                  `json:"drawPileCount"`
	FaceUpPile       []game.Card          `json:"faceUpPile"`
	OpponentLastMove *webRummyMove        `json:"opponentLastMove,omitempty"`
	CanDrawFaceUp    bool                 `json:"canDrawFaceUp"`
	CanDrawTwo       bool                 `json:"canDrawTwo"`
	TurnsCompleted   int                  `json:"turnsCompleted"`
	TurnLimit        int                  `json:"turnLimit"`
	MatchHand        int                  `json:"matchHand,omitempty"`
	MatchHands       int                  `json:"matchHands,omitempty"`
	MatchScores      [2]int               `json:"matchScores,omitempty"`
	Results          []webRummyResult     `json:"results,omitempty"`
}

type webRummyMove struct {
	Action string     `json:"action"`
	Card   *game.Card `json:"card,omitempty"`
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
	session := &rummySession{Parameters: params, TurnLimit: rummyTurnLimit, HandNumber: 1}
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

func newRummyCampaignSession(firstHero, secondHero heroes.Hero, rules *campaigns.RummyCampaignRules,
	rng *rand.Rand,
) (*rummySession, error) {
	if rules == nil || rng == nil {
		return nil, errors.New("Rummy campaign setup is unavailable")
	}
	params := rules.Parameters
	if params.J == 0 {
		params = game.NewRummyParameters(
			firstHero.Favored+secondHero.Favored, firstHero.Shift+secondHero.Shift,
			firstHero.Bounty+secondHero.Bounty, firstHero.Draft+secondHero.Draft,
		)
	}
	turnLimit := rules.TurnLimit
	if turnLimit <= 0 {
		turnLimit = rummyTurnLimit
	}
	if params.HandSize < 1 || turnLimit < 1 {
		return nil, errors.New("Rummy campaign has invalid hand size or turn limit")
	}

	deck := game.NewDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	remove := func(card game.Card) error {
		for index, candidate := range deck {
			if candidate == card {
				deck = append(deck[:index], deck[index+1:]...)
				return nil
			}
		}
		return fmt.Errorf("Rummy campaign card %s is duplicated or invalid", card)
	}
	used := make(map[game.Card]bool)
	removeScript := func(cards []game.Card) error {
		for _, card := range cards {
			if !card.Valid() || used[card] {
				return fmt.Errorf("Rummy campaign has a duplicate or invalid card %s", card)
			}
			used[card] = true
			if err := remove(card); err != nil {
				return err
			}
		}
		return nil
	}
	if err := removeScript(rules.PlayerHand); err != nil {
		return nil, err
	}
	if err := removeScript(rules.CPUHand); err != nil {
		return nil, err
	}
	if err := removeScript(rules.InitialFaceUp); err != nil {
		return nil, err
	}
	if err := removeScript(rules.DrawPileTop); err != nil {
		return nil, err
	}

	session := &rummySession{
		Parameters: params, Rules: rules, TurnLimit: turnLimit, HandNumber: 1,
		FaceUp: append([]game.Card(nil), rules.InitialFaceUp...),
	}
	handSizes := [2]int{params.HandSize, params.HandSize}
	scripted := [2][]game.Card{rules.PlayerHand, rules.CPUHand}
	for player := range session.Hands {
		hand := append([]game.Card(nil), scripted[player]...)
		if len(hand) == 0 {
			hand = make([]game.Card, 0, handSizes[player])
			for len(hand) < handSizes[player] {
				if len(deck) == 0 {
					return nil, errors.New("not enough cards to deal the Rummy campaign hand")
				}
				index := rng.Intn(len(deck))
				hand = append(hand, deck[index])
				deck = append(deck[:index], deck[index+1:]...)
			}
		}
		session.Hands[player] = hand
	}
	top := append([]game.Card(nil), rules.DrawPileTop...)
	for left, right := 0, len(top)-1; left < right; left, right = left+1, right-1 {
		top[left], top[right] = top[right], top[left]
	}
	session.DrawPile = append(deck, top...)
	return session, nil
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
		TurnLimit:      r.TurnLimit,
		MatchHand:      r.HandNumber,
		MatchScores:    r.HandScores,
	}
	if result.TurnLimit <= 0 {
		result.TurnLimit = rummyTurnLimit
	}
	if r.Rules != nil && r.Rules.MatchHands > 1 {
		result.MatchHands = r.Rules.MatchHands
	}
	lastMove := r.LastMoves[1-viewerPlayer]
	if lastMove.Action != "" {
		result.OpponentLastMove = &webRummyMove{Action: lastMove.Action}
		if lastMove.Card != nil {
			card := *lastMove.Card
			result.OpponentLastMove.Card = &card
		}
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
		if !s.ensureRummyStock(rummy, 1) {
			return errors.New("not enough cards remain to replace the face-up draw")
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
	r.LastMoves[player] = rummyLastMove{Action: "draw-two"}
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
	taken := r.FaceUp[lastFaceUp]
	r.Hands[player] = append(r.Hands[player], taken)
	r.FaceUp = r.FaceUp[:lastFaceUp]
	r.FaceUp = append(r.FaceUp, replacement)
	r.LastMoves[player] = rummyLastMove{Action: "draw-face-up", Card: &taken}
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
	turnLimit := rummy.TurnLimit
	if turnLimit <= 0 {
		turnLimit = rummyTurnLimit
	}
	if rummy.Turns >= turnLimit {
		for player := range rummy.Hands {
			rummy.Results[player] = game.BestRummyPartition(rummy.Hands[player], rummy.Parameters)
			rummy.HandScores[player] += rummy.Results[player].Total
		}
		if rummy.Rules != nil && rummy.Rules.MatchHands > rummy.HandNumber {
			playerHero, ok := heroes.ByID(session.playerHeroID)
			if !ok {
				return fmt.Errorf("unknown player hero %q", session.playerHeroID)
			}
			cpuHero, ok := heroes.ByID(session.opponentHeroID)
			if !ok {
				return fmt.Errorf("unknown Rummy CPU hero %q", session.opponentHeroID)
			}
			playerHero = applyWebHeroTuning(playerHero, session.conductorTunings[0])
			next, err := newRummyCampaignSession(playerHero, cpuHero, rummy.Rules, s.rng)
			if err != nil {
				return fmt.Errorf("start next Rummy campaign hand: %w", err)
			}
			next.HandNumber = rummy.HandNumber + 1
			next.HandScores = rummy.HandScores
			session.rummy = next
			session.actor, session.phase, session.winner = 0, "rummy-opening", -1
			session.message = fmt.Sprintf("Hand %d of %d. Draw two cards to begin.",
				next.HandNumber, next.Rules.MatchHands)
			return nil
		}
		switch {
		case rummy.HandScores[0] > rummy.HandScores[1]:
			session.winner = 0
			session.message = "Player 1 wins the Rummy game."
		case rummy.HandScores[1] > rummy.HandScores[0]:
			session.winner = 1
			session.message = "Player 2 wins the Rummy game."
		default:
			session.winner = -1
			session.message = "The Rummy game is a draw."
		}
		session.phase = "gameover"
		if session.campaign != nil && rummy.Rules != nil {
			won := false
			switch rummy.Rules.Goal {
			case "score":
				won = rummy.Results[0].Total >= rummy.Rules.Target
			case "match":
				won = rummy.HandScores[0] > rummy.HandScores[1]
			default:
				won = session.winner == 0
			}
			s.finishSpecialCampaignMission(won)
		} else {
			s.awardLocalMultiplayerPayout(rummy.Results[0].Total)
		}
		return nil
	}

	session.actor = 1 - session.actor
	if !s.ensureRummyStock(rummy, 1) {
		return errors.New("Rummy stock cannot be replenished from the discard pile")
	}
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
			forcedFaceUp := rummy.deckCount() < 2
			if session.phase == "rummy-turn" && len(rummy.FaceUp) > 0 &&
				(forcedFaceUp || s.shouldTakeRummyDiscard(rummy, session.actor)) {
				if !s.ensureRummyStock(rummy, 1) {
					return errors.New("Rummy stock cannot be replenished from the discard pile")
				}
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

func (s *webServer) ensureRummyStock(rummy *rummySession, minimum int) bool {
	if rummy == nil || minimum < 0 {
		return false
	}
	if rummy.Rules == nil || len(rummy.DrawPile) >= minimum {
		return true
	}
	if len(rummy.FaceUp) < 2 {
		return false
	}
	topDiscard := rummy.FaceUp[len(rummy.FaceUp)-1]
	rummy.DrawPile = append(rummy.DrawPile, rummy.FaceUp[:len(rummy.FaceUp)-1]...)
	rummy.FaceUp = []game.Card{topDiscard}
	s.rng.Shuffle(len(rummy.DrawPile), func(i, j int) {
		rummy.DrawPile[i], rummy.DrawPile[j] = rummy.DrawPile[j], rummy.DrawPile[i]
	})
	return len(rummy.DrawPile) >= minimum
}

func (s *webServer) aiRummyDiscard() error {
	rummy := s.session.rummy
	player := s.session.actor
	hand := rummy.Hands[player]
	if len(hand) < 2 {
		return errors.New("CPU has fewer than two cards to discard")
	}
	policy := "standard"
	if rummy.Rules != nil {
		policy = rummy.Rules.CPUPolicy
	}
	if policy == "random" {
		first := s.rng.Intn(len(hand))
		second := s.rng.Intn(len(hand) - 1)
		if second >= first {
			second++
		}
		return rummy.discard(player, first, second)
	}
	if policy == "hard" {
		faceUpIndex, buriedIndex, _ := bestRummyDiscard(hand, rummy.Parameters)
		return rummy.discard(player, faceUpIndex, buriedIndex)
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

func (s *webServer) shouldTakeRummyDiscard(rummy *rummySession, player int) bool {
	if rummy == nil || player < 0 || player >= len(rummy.Hands) || len(rummy.FaceUp) == 0 {
		return false
	}
	policy := "standard"
	if rummy.Rules != nil {
		policy = rummy.Rules.CPUPolicy
	}
	if policy == "random" || policy == "passive" {
		return false
	}
	hand := rummy.Hands[player]
	discard := rummy.FaceUp[len(rummy.FaceUp)-1]
	if policy == "hard" {
		if len(rummy.DrawPile) < 2 {
			return false
		}
		faceUpQuality := game.RummyHandQuality(
			append(append([]game.Card(nil), hand...), discard), rummy.Parameters)
		expectedDrawQuality := 0.0
		for sample := 0; sample < 10; sample++ {
			first := s.rng.Intn(len(rummy.DrawPile))
			second := s.rng.Intn(len(rummy.DrawPile) - 1)
			if second >= first {
				second++
			}
			candidate := append(append([]game.Card(nil), hand...),
				rummy.DrawPile[first], rummy.DrawPile[second])
			_, _, quality := bestRummyDiscard(candidate, rummy.Parameters)
			expectedDrawQuality += quality
		}
		return faceUpQuality > expectedDrawQuality/10
	}
	return game.ShouldTakeRummyDiscard(hand, discard, rummy.Parameters)
}

func bestRummyDiscard(hand []game.Card, params game.RummyParameters) (int, int, float64) {
	bestQuality := -1.0e9
	faceUpIndex, buriedIndex := 0, 1
	for first := range hand {
		for second := first + 1; second < len(hand); second++ {
			remaining := make([]game.Card, 0, len(hand)-2)
			for index, card := range hand {
				if index != first && index != second {
					remaining = append(remaining, card)
				}
			}
			quality := game.RummyHandQuality(remaining, params)
			if quality > bestQuality {
				bestQuality = quality
				faceUpIndex, buriedIndex = first, second
			}
		}
	}
	return faceUpIndex, buriedIndex, bestQuality
}
