package tugwar

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

const (
	HandSize  = 6
	NumPiles  = 3
	ShiftCap  = 3
	BountyMin = -1
	BountyMax = 3
	DraftMin  = -1
	DraftMax  = 1
)

type Pile struct {
	Cards []game.Card `json:"cards"`
	Zone  int         `json:"zone"`
}

func (p Pile) Top() (game.Card, bool) {
	if len(p.Cards) == 0 {
		return game.Card{}, false
	}
	return p.Cards[len(p.Cards)-1], true
}

type GameState struct {
	Deck       []game.Card    `json:"deck"`
	Discard    []game.Card    `json:"discard"`
	Hands      [2][]game.Card `json:"hands"`
	Piles      [NumPiles]Pile `json:"piles"`
	BountySuit game.Suit      `json:"bountySuit"`
	S          int            `json:"s"`
	F          int            `json:"f"`
	B          int            `json:"b"`
	Pushes     [2]int         `json:"pushes"`
	Current    int            `json:"current"`
	Round      int            `json:"round"`
	Passes     int            `json:"passes"`
	Over       bool           `json:"over"`
	Winner     int            `json:"winner"`
	Tiebreak   string         `json:"tiebreak,omitempty"`
}

type ActionKind string

const (
	ActionPlayCard  ActionKind = "play-card"
	ActionFlipDeck  ActionKind = "flip-deck"
	ActionSpendPush ActionKind = "spend-push"
	ActionPass      ActionKind = "pass"
)

type Action struct {
	Kind      ActionKind `json:"kind"`
	CardIndex int        `json:"cardIndex,omitempty"`
	PileIndex int        `json:"pileIndex,omitempty"`
}

type ActionLogEntry struct {
	Round      int        `json:"round"`
	Player     int        `json:"player"`
	Kind       ActionKind `json:"kind"`
	PileIndex  int        `json:"pileIndex"`
	Card       *game.Card `json:"card,omitempty"`
	Top        *game.Card `json:"top,omitempty"`
	CardValue  int        `json:"cardValue,omitempty"`
	TopValue   int        `json:"topValue,omitempty"`
	Succeeded  bool       `json:"succeeded,omitempty"`
	ZoneBefore int        `json:"zoneBefore"`
	ZoneAfter  int        `json:"zoneAfter"`
	Refilled   [2]int     `json:"refilled,omitempty"`
}

type Game struct {
	State GameState        `json:"state"`
	Log   []ActionLogEntry `json:"log"`
}

type PileScore struct {
	Zone  int `json:"zone"`
	Cards int `json:"cards"`
	Value int `json:"value"`
}

type Result struct {
	Scores    [2]int              `json:"scores"`
	ZoneCards [2]int              `json:"zoneCards"`
	Piles     [NumPiles]PileScore `json:"piles"`
	Winner    int                 `json:"winner"`
	Tiebreak  string              `json:"tiebreak"`
}

func NewGame(heroA, heroB heroes.Hero, rng *rand.Rand) (*Game, error) {
	if rng == nil {
		return nil, errors.New("random source is required")
	}
	firstHero, ok := heroes.ByID(heroA.ID)
	if !ok {
		return nil, fmt.Errorf("unknown first hero %q", heroA.ID)
	}
	secondHero, ok := heroes.ByID(heroB.ID)
	if !ok {
		return nil, fmt.Errorf("unknown second hero %q", heroB.ID)
	}
	heroA, heroB = firstHero, secondHero

	state := GameState{
		Current: 0,
		Round:   1,
		Winner:  -1,
		S:       min(heroA.Shift+heroB.Shift, ShiftCap),
		F:       heroA.Favored + heroB.Favored,
		B:       clamp(heroA.Bounty+heroB.Bounty, BountyMin, BountyMax),
	}
	pushesEach := 2 + clamp(heroA.Draft+heroB.Draft, DraftMin, DraftMax)
	state.Pushes = [2]int{pushesEach, pushesEach}

	deck := game.NewDeck()
	game.Shuffle(deck, rng)
	state.BountySuit = deck[len(deck)-1].Suit
	deck = deck[:len(deck)-1]
	drawTop := func() game.Card {
		last := len(deck) - 1
		card := deck[last]
		deck = deck[:last]
		return card
	}
	for player := range state.Hands {
		for count := 0; count < HandSize; count++ {
			state.Hands[player] = append(state.Hands[player], drawTop())
		}
	}
	for pileIndex := range state.Piles {
		state.Piles[pileIndex] = Pile{
			Cards: []game.Card{drawTop()},
		}
	}
	state.Deck = append([]game.Card(nil), deck...)
	return &Game{State: state}, nil
}

func CardValue(card, opposing game.Card, shift, favor int) int {
	base := card.Face + shift
	if card.Suit.Beats(opposing.Suit) {
		return 2*base + favor
	}
	return base
}

func CardBeatsTop(card, top game.Card, shift, favor int) bool {
	return CardValue(card, top, shift, favor) > CardValue(top, card, shift, favor)
}

func PileValue(pile Pile, bountySuit game.Suit, bounty int) int {
	total := 0
	for _, card := range pile.Cards {
		value := 1
		if card.Suit == bountySuit {
			value += bounty
		}
		total += value
	}
	return total
}

func (g *Game) HasWinningPlay(player int) bool {
	if g == nil || player < 0 || player > 1 || g.State.Over {
		return false
	}
	for _, card := range g.State.Hands[player] {
		for _, pile := range g.State.Piles {
			top, ok := pile.Top()
			if ok && CardBeatsTop(card, top, g.State.S, g.State.F) {
				return true
			}
		}
	}
	return false
}

func (g *Game) HasLegalAction(player int) bool {
	if g == nil || player != g.State.Current || player < 0 || player > 1 || g.State.Over {
		return false
	}
	return len(g.State.Deck) > 0 || g.HasWinningPlay(player)
}

func (g *Game) CanSpendPush(player, pileIndex int) bool {
	if g == nil || player < 0 || player > 1 || player != g.State.Current ||
		pileIndex < 0 || pileIndex >= len(g.State.Piles) || g.State.Over ||
		len(g.State.Deck) == 0 || g.State.Pushes[player] <= 0 {
		return false
	}
	return g.State.Piles[pileIndex].Zone != playerZone(player)
}

func (g *Game) CanPass() bool {
	return g != nil && !g.HasLegalAction(g.State.Current)
}

func (g *Game) Apply(action Action) error {
	if g == nil || g.State.Over {
		return errors.New("Tug War game is over")
	}
	player := g.State.Current
	if player < 0 || player > 1 {
		return errors.New("current player is invalid")
	}
	log := ActionLogEntry{Round: g.State.Round, Player: player, Kind: action.Kind}
	switch action.Kind {
	case ActionPlayCard:
		if action.CardIndex < 0 || action.CardIndex >= len(g.State.Hands[player]) ||
			action.PileIndex < 0 || action.PileIndex >= len(g.State.Piles) {
			return errors.New("card or pile selection is out of range")
		}
		if !g.HasLegalAction(player) {
			return errors.New("no legal play is available; pass instead")
		}
		pile := &g.State.Piles[action.PileIndex]
		top, ok := pile.Top()
		if !ok {
			return errors.New("cannot play on an empty pile")
		}
		card := g.State.Hands[player][action.CardIndex]
		log.PileIndex, log.Card, log.Top = action.PileIndex, cardPointer(card), cardPointer(top)
		log.CardValue = CardValue(card, top, g.State.S, g.State.F)
		log.TopValue = CardValue(top, card, g.State.S, g.State.F)
		log.ZoneBefore = pile.Zone
		g.State.Hands[player] = removeCard(g.State.Hands[player], action.CardIndex)
		if log.CardValue > log.TopValue {
			log.Succeeded = true
			pile.Cards = append(pile.Cards, card)
			pile.Zone = moveZone(pile.Zone, player)
		} else {
			g.State.Discard = append(g.State.Discard, card)
		}
		log.ZoneAfter = pile.Zone
	case ActionFlipDeck:
		if action.PileIndex < 0 || action.PileIndex >= len(g.State.Piles) {
			return errors.New("pile selection is out of range")
		}
		if len(g.State.Deck) == 0 {
			return errors.New("the draw deck is empty")
		}
		pile := &g.State.Piles[action.PileIndex]
		top, ok := pile.Top()
		if !ok {
			return errors.New("cannot flip onto an empty pile")
		}
		card := g.State.Deck[len(g.State.Deck)-1]
		g.State.Deck = g.State.Deck[:len(g.State.Deck)-1]
		log.PileIndex, log.Card, log.Top = action.PileIndex, cardPointer(card), cardPointer(top)
		log.CardValue = CardValue(card, top, g.State.S, g.State.F)
		log.TopValue = CardValue(top, card, g.State.S, g.State.F)
		log.ZoneBefore = pile.Zone
		log.Succeeded = log.CardValue > log.TopValue
		pile.Cards = append(pile.Cards, card)
		if log.Succeeded {
			pile.Zone = moveZone(pile.Zone, player)
		}
		log.ZoneAfter = pile.Zone
	case ActionSpendPush:
		if !g.CanSpendPush(player, action.PileIndex) {
			return errors.New("push is unavailable for that pile")
		}
		pile := &g.State.Piles[action.PileIndex]
		log.PileIndex, log.ZoneBefore = action.PileIndex, pile.Zone
		g.State.Pushes[player]--
		pile.Zone = moveZone(pile.Zone, player)
		log.ZoneAfter = pile.Zone
	case ActionPass:
		if !g.CanPass() {
			return errors.New("passing is only allowed when no legal action is available")
		}
		g.State.Passes++
		g.Log = append(g.Log, log)
		if g.State.Passes >= 2 {
			g.State.Over = true
			result := g.Result()
			g.State.Winner, g.State.Tiebreak = result.Winner, result.Tiebreak
			return nil
		}
		g.advance(player, &log)
		g.Log[len(g.Log)-1] = log
		return nil
	default:
		return errors.New("unknown Tug War action")
	}

	g.State.Passes = 0
	g.Log = append(g.Log, log)
	g.advance(player, &g.Log[len(g.Log)-1])
	return nil
}

func (g *Game) advance(player int, log *ActionLogEntry) {
	lastPlayer := 1
	if g.State.Round%2 == 0 {
		lastPlayer = 0
	}
	if player != lastPlayer {
		g.State.Current = 1 - player
		return
	}
	for hand := range g.State.Hands {
		for len(g.State.Hands[hand]) < HandSize && len(g.State.Deck) > 0 {
			last := len(g.State.Deck) - 1
			g.State.Hands[hand] = append(g.State.Hands[hand], g.State.Deck[last])
			g.State.Deck = g.State.Deck[:last]
			log.Refilled[hand]++
		}
	}
	g.State.Round++
	g.State.Current = (g.State.Round + 1) % 2
}

func (g *Game) Result() Result {
	result := Result{Winner: -1, Tiebreak: "draw"}
	if g == nil {
		return result
	}
	for index, pile := range g.State.Piles {
		value := PileValue(pile, g.State.BountySuit, g.State.B)
		result.Piles[index] = PileScore{Zone: pile.Zone, Cards: len(pile.Cards), Value: value}
		if pile.Zone == -1 {
			result.Scores[0] += value
			result.ZoneCards[0] += len(pile.Cards)
		} else if pile.Zone == 1 {
			result.Scores[1] += value
			result.ZoneCards[1] += len(pile.Cards)
		}
	}
	switch {
	case result.Scores[0] > result.Scores[1]:
		result.Winner, result.Tiebreak = 0, "score"
	case result.Scores[1] > result.Scores[0]:
		result.Winner, result.Tiebreak = 1, "score"
	case result.ZoneCards[0] > result.ZoneCards[1]:
		result.Winner, result.Tiebreak = 0, "zone-cards"
	case result.ZoneCards[1] > result.ZoneCards[0]:
		result.Winner, result.Tiebreak = 1, "zone-cards"
	}
	return result
}

func (g *Game) ChooseCPUAction() Action {
	if g == nil || g.State.Over || g.State.Current < 0 || g.State.Current > 1 {
		return Action{Kind: ActionPass}
	}
	player := g.State.Current
	myZone := playerZone(player)
	bestPile, bestDelta := -1, 0
	if g.State.Pushes[player] > 0 && len(g.State.Deck) > 0 {
		for pileIndex, pile := range g.State.Piles {
			nextZone := moveZone(pile.Zone, player)
			if nextZone == pile.Zone {
				continue
			}
			value := PileValue(pile, g.State.BountySuit, g.State.B)
			myDelta := zonePoints(nextZone, myZone, value) - zonePoints(pile.Zone, myZone, value)
			oppZone := -myZone
			oppDelta := zonePoints(nextZone, oppZone, value) - zonePoints(pile.Zone, oppZone, value)
			delta := myDelta - oppDelta
			if delta > bestDelta {
				bestPile, bestDelta = pileIndex, delta
			}
		}
	}
	if bestPile >= 0 {
		return Action{Kind: ActionSpendPush, PileIndex: bestPile}
	}

	bestCard, bestPlayPile := -1, -1
	bestKey := [4]int{}
	for cardIndex, card := range g.State.Hands[player] {
		for pileIndex, pile := range g.State.Piles {
			top, ok := pile.Top()
			if !ok || !CardBeatsTop(card, top, g.State.S, g.State.F) {
				continue
			}
			nextZone := moveZone(pile.Zone, player)
			enters := nextZone == myZone
			cardValue := 1
			if card.Suit == g.State.BountySuit {
				cardValue += g.State.B
			}
			gain := 0
			cardPriority := 0
			if enters {
				gain = cardValue
				if pile.Zone != myZone {
					gain += PileValue(pile, g.State.BountySuit, g.State.B)
				}
				cardPriority = -cardValue
			}
			key := [4]int{-gain, cardPriority, card.Face, -len(pile.Cards)}
			if bestCard < 0 || lessKey(key, bestKey) {
				bestCard, bestPlayPile, bestKey = cardIndex, pileIndex, key
			}
		}
	}
	if bestCard >= 0 {
		return Action{Kind: ActionPlayCard, CardIndex: bestCard, PileIndex: bestPlayPile}
	}
	if len(g.State.Deck) > 0 {
		target := 0
		for pileIndex := 1; pileIndex < len(g.State.Piles); pileIndex++ {
			current := g.State.Piles[target]
			candidate := g.State.Piles[pileIndex]
			currentTop, _ := current.Top()
			candidateTop, _ := candidate.Top()
			if len(candidate.Cards) > len(current.Cards) ||
				len(candidate.Cards) == len(current.Cards) && candidateTop.Face < currentTop.Face {
				target = pileIndex
			}
		}
		return Action{Kind: ActionFlipDeck, PileIndex: target}
	}
	return Action{Kind: ActionPass}
}

func moveZone(zone, player int) int {
	direction := playerZone(player)
	return max(-1, min(1, zone+direction))
}

func playerZone(player int) int {
	if player == 0 {
		return -1
	}
	return 1
}

func zonePoints(zone, owner, value int) int {
	if zone == owner {
		return value
	}
	return 0
}

func removeCard(cards []game.Card, index int) []game.Card {
	copy(cards[index:], cards[index+1:])
	return cards[:len(cards)-1]
}

func cardPointer(card game.Card) *game.Card {
	return &card
}

func lessKey(first, second [4]int) bool {
	for index := range first {
		if first[index] != second[index] {
			return first[index] < second[index]
		}
	}
	return false
}

func clamp(value, lower, upper int) int {
	return max(lower, min(upper, value))
}
