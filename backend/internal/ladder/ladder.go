package ladder

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

type PlayerID int

const (
	PlayerA PlayerID = iota
	PlayerB
)

var (
	ErrIllegalPlay    = errors.New("illegal Ladder play")
	ErrPassNotAllowed = errors.New("passing is not allowed while a legal play exists")
	ErrGameOver       = errors.New("Ladder game is over")
	ErrWrongTurn      = errors.New("it is not this player's turn")
	ErrCardNotInHand  = errors.New("card is not in this player's hand")
	ErrOpeningPending = errors.New("the opening card has not been played yet")
)

type SharedStats struct {
	Bounty int `json:"bounty"`
	Draft  int `json:"draft"`
	Shift  int `json:"shift"`
	Favor  int `json:"favor"`
}

type LadderState struct {
	HandA                    []game.Card `json:"handA"`
	HandB                    []game.Card `json:"handB"`
	Deck                     []game.Card `json:"deck"`
	Discard                  []game.Card `json:"discard"`
	PlayedCards              []game.Card `json:"playedCards"`
	ChainCard                *game.Card  `json:"chainCard,omitempty"`
	ChainValue               *int        `json:"chainValue,omitempty"`
	ChainSuit                *game.Suit  `json:"chainSuit,omitempty"`
	ChainIsWild              bool        `json:"chainIsWild"`
	Current                  PlayerID    `json:"current"`
	OpeningPending           bool        `json:"openingPending,omitempty"`
	OpeningPlayer            PlayerID    `json:"openingPlayer,omitempty"`
	OpeningCard              *game.Card  `json:"openingCard,omitempty"`
	OpeningResponder         PlayerID    `json:"openingResponder,omitempty"`
	SharedStats              SharedStats `json:"sharedStats"`
	SharedShift              int         `json:"sharedShift"`
	SharedFavor              int         `json:"sharedFavor"`
	WildFaceRaw              int         `json:"wildFaceRaw"`
	BountyCard               game.Card   `json:"bountyCard"`
	InitiativeSuit           game.Suit   `json:"initiativeSuit"`
	FallbackNoInitiativeSuit bool        `json:"fallbackNoInitiativeSuit"`
	Winner                   *PlayerID   `json:"winner,omitempty"`
}

type LadderGame struct {
	handA                    []game.Card
	handB                    []game.Card
	deck                     []game.Card
	discard                  []game.Card
	playedCards              []game.Card
	chainCard                *game.Card
	chainValue               *int
	chainSuit                *game.Suit
	chainIsWild              bool
	current                  PlayerID
	openingPending           bool
	openingPlayer            PlayerID
	openingCard              *game.Card
	openingResponder         PlayerID
	sharedStats              SharedStats
	wildFaceRaw              int
	bountyCard               game.Card
	initiativeSuit           game.Suit
	fallbackNoInitiativeSuit bool
	winner                   *PlayerID
}

type Policy interface {
	ChoosePlay(g *LadderGame, legal []game.Card) game.Card
}

type greedyPolicy struct{}

func SharedStatsFor(a, b heroes.Hero) SharedStats {
	bounty := floorHalf(a.Bounty + b.Bounty)
	shift := floorHalf(a.Shift + b.Shift)
	return SharedStats{
		Bounty: clamp(bounty, -2, 3),
		Draft:  max(0, a.Draft+b.Draft),
		Shift:  clamp(shift, 0, 4),
		Favor:  a.Favored + b.Favored,
	}
}

func NewLadderGame(heroA, heroB heroes.Hero, deck []game.Card, seed int64) (*LadderGame, error) {
	return newLadderGame(heroA, heroB, deck, seed, false)
}

func NewLadderGameWithDeferredOpening(heroA, heroB heroes.Hero, deck []game.Card, seed int64) (*LadderGame, error) {
	return newLadderGame(heroA, heroB, deck, seed, true)
}

func newLadderGame(heroA, heroB heroes.Hero, deck []game.Card, seed int64, deferOpening bool) (*LadderGame, error) {
	stats := SharedStatsFor(heroA, heroB)
	if deck == nil {
		deck = game.NewDeck()
		if seed == 0 {
			seed = time.Now().UnixNano()
		}
		game.Shuffle(deck, rand.New(rand.NewSource(seed)))
	} else {
		deck = append([]game.Card(nil), deck...)
	}
	if err := validateDeck(deck); err != nil {
		return nil, err
	}

	handSize := 9 + stats.Bounty
	handA := popCards(&deck, handSize)
	handB := popCards(&deck, handSize)
	bountyCard := popCard(&deck)
	initiativeSuit := bountyCard.Suit
	attempts := 0
	for !anyoneHolds(handA, handB, initiativeSuit) && len(deck) > 0 && attempts < 50 {
		initiativeSuit = popCard(&deck).Suit
		attempts++
	}
	fallbackNoInitiativeSuit := !anyoneHolds(handA, handB, initiativeSuit)

	if stats.Draft > 0 {
		passA := lowestCards(handA, stats.Draft)
		passB := lowestCards(handB, stats.Draft)
		handA = removeCards(handA, passA)
		handB = removeCards(handB, passB)
		handA = append(handA, passB...)
		handB = append(handB, passA...)
	}

	leader := chooseLeader(handA, handB, initiativeSuit, fallbackNoInitiativeSuit)
	result := &LadderGame{
		handA: handA, handB: handB, deck: append([]game.Card(nil), deck...),
		sharedStats: stats, wildFaceRaw: bountyCard.Face,
		bountyCard: bountyCard, initiativeSuit: initiativeSuit,
		fallbackNoInitiativeSuit: fallbackNoInitiativeSuit,
	}
	var opening game.Card
	if fallbackNoInitiativeSuit {
		opening = chooseOpening(result.handFor(leader), result.wildFaceRaw)
	} else {
		var found bool
		opening, found = lowestCardOfSuit(result.handFor(leader), result.initiativeSuit)
		if !found {
			return nil, errors.New("Ladder leader has no card of the initiative suit")
		}
	}
	if deferOpening {
		result.openingPending = true
		result.openingPlayer = leader
		result.openingCard = cloneCard(&opening)
		result.current = leader
	} else {
		result.openChainWithCard(leader, opening)
		if result.winner == nil {
			result.current = otherPlayer(leader)
		}
	}
	return result, nil
}

func (g *LadderGame) State() LadderState {
	if g == nil {
		return LadderState{HandA: []game.Card{}, HandB: []game.Card{}}
	}
	return LadderState{
		HandA:       append([]game.Card(nil), g.handA...),
		HandB:       append([]game.Card(nil), g.handB...),
		Deck:        append([]game.Card(nil), g.deck...),
		Discard:     append([]game.Card(nil), g.discard...),
		PlayedCards: append([]game.Card(nil), g.playedCards...),
		ChainCard:   cloneCard(g.chainCard),
		ChainValue:  cloneInt(g.chainValue), ChainSuit: cloneSuit(g.chainSuit),
		ChainIsWild: g.chainIsWild, Current: g.current, SharedStats: g.sharedStats,
		OpeningPending: g.openingPending, OpeningPlayer: g.openingPlayer,
		OpeningCard: cloneCard(g.openingCard), OpeningResponder: g.openingResponder,
		SharedShift: g.sharedStats.Shift, SharedFavor: g.sharedStats.Favor,
		WildFaceRaw: g.wildFaceRaw, BountyCard: g.bountyCard,
		InitiativeSuit:           g.initiativeSuit,
		FallbackNoInitiativeSuit: g.fallbackNoInitiativeSuit,
		Winner:                   clonePlayerID(g.winner),
	}
}

func Restore(state LadderState) *LadderGame {
	chainCard := cloneCard(state.ChainCard)
	if chainCard == nil {
		chainCard = inferLegacyChainCard(state)
	}
	chainValue := cloneInt(state.ChainValue)
	if chainCard != nil && !state.ChainIsWild {
		value := chainCard.Face + state.SharedStats.Shift
		chainValue = &value
	}
	playedCards := append([]game.Card(nil), state.PlayedCards...)
	if len(playedCards) == 0 && chainCard != nil {
		playedCards = append(playedCards, *chainCard)
	}
	return &LadderGame{
		handA:                    append([]game.Card(nil), state.HandA...),
		handB:                    append([]game.Card(nil), state.HandB...),
		deck:                     append([]game.Card(nil), state.Deck...),
		discard:                  append([]game.Card(nil), state.Discard...),
		playedCards:              playedCards,
		chainCard:                chainCard,
		chainValue:               chainValue,
		chainSuit:                cloneSuit(state.ChainSuit),
		chainIsWild:              state.ChainIsWild,
		current:                  state.Current,
		openingPending:           state.OpeningPending,
		openingPlayer:            state.OpeningPlayer,
		openingCard:              cloneCard(state.OpeningCard),
		openingResponder:         state.OpeningResponder,
		sharedStats:              state.SharedStats,
		wildFaceRaw:              state.WildFaceRaw,
		bountyCard:               state.BountyCard,
		initiativeSuit:           state.InitiativeSuit,
		fallbackNoInitiativeSuit: state.FallbackNoInitiativeSuit,
		winner:                   clonePlayerID(state.Winner),
	}
}

func inferLegacyChainCard(state LadderState) *game.Card {
	if state.ChainSuit == nil {
		return nil
	}
	if state.ChainIsWild {
		if state.WildFaceRaw < 1 || state.WildFaceRaw > 9 {
			return nil
		}
		return &game.Card{Suit: *state.ChainSuit, Face: state.WildFaceRaw}
	}
	if state.ChainValue == nil {
		return nil
	}
	faces := make(map[int]struct{}, 2)
	addFace := func(face int) {
		if face >= 1 && face <= 9 {
			faces[face] = struct{}{}
		}
	}
	addFace(*state.ChainValue - state.SharedStats.Shift)
	scaledValue := *state.ChainValue - state.SharedStats.Favor
	if scaledValue%2 == 0 {
		addFace(scaledValue/2 - state.SharedStats.Shift)
	}
	if len(faces) != 1 {
		return nil
	}
	for face := range faces {
		return &game.Card{Suit: *state.ChainSuit, Face: face}
	}
	return nil
}

func (g *LadderGame) ReplaceCard(player PlayerID, index int, rng *rand.Rand) error {
	if g == nil || g.winner != nil {
		return ErrGameOver
	}
	if g.openingPending {
		return ErrOpeningPending
	}
	if !validPlayer(player) {
		return fmt.Errorf("%w: unknown player %d", ErrIllegalPlay, player)
	}
	if player != g.current {
		return ErrWrongTurn
	}
	if rng == nil {
		return errors.New("random source is required")
	}
	hand := g.handFor(player)
	if index < 0 || index >= len(hand) {
		return ErrCardNotInHand
	}
	if len(g.deck) == 0 {
		return errors.New("there are no cards left to draw")
	}
	deckIndex := rng.Intn(len(g.deck))
	hand[index], g.deck[deckIndex] = g.deck[deckIndex], hand[index]
	g.discard = append(g.discard, g.deck[deckIndex])
	g.deck = append(g.deck[:deckIndex], g.deck[deckIndex+1:]...)
	g.setHand(player, hand)
	return nil
}

func (g *LadderGame) LegalPlays(player PlayerID) []game.Card {
	legal := make([]game.Card, 0)
	if g == nil || g.winner != nil || g.openingPending || !validPlayer(player) {
		return legal
	}
	for _, card := range g.handFor(player) {
		isWild := card.Face == g.wildFaceRaw
		if g.chainIsWild {
			if isWild && g.chainSuit != nil && card.Suit.Beats(*g.chainSuit) {
				legal = append(legal, card)
			}
			continue
		}
		if isWild || g.chainValue == nil ||
			Effective(card, g.chainSuit, g.sharedStats.Shift, g.sharedStats.Favor) >= *g.chainValue {
			legal = append(legal, card)
		}
	}
	return legal
}

func (g *LadderGame) Play(player PlayerID, card game.Card) error {
	if g == nil || g.winner != nil {
		return ErrGameOver
	}
	if g.openingPending {
		return ErrOpeningPending
	}
	if !validPlayer(player) {
		return fmt.Errorf("%w: unknown player %d", ErrIllegalPlay, player)
	}
	if player != g.current {
		return ErrWrongTurn
	}
	hand := g.handFor(player)
	index := cardIndex(hand, card)
	if index < 0 {
		return ErrCardNotInHand
	}
	if !containsCard(g.LegalPlays(player), card) {
		return fmt.Errorf("%w: %s", ErrIllegalPlay, card)
	}
	hand = removeCardAt(hand, index)
	g.setHand(player, hand)
	g.setChainFromPlayedCard(card)
	if len(hand) == 0 {
		winner := player
		g.winner = &winner
		return nil
	}
	g.current = otherPlayer(player)
	return nil
}

func (g *LadderGame) PassTurn(player PlayerID) error {
	if g == nil || g.winner != nil {
		return ErrGameOver
	}
	if g.openingPending {
		return ErrOpeningPending
	}
	if !validPlayer(player) {
		return fmt.Errorf("%w: unknown player %d", ErrIllegalPlay, player)
	}
	if player != g.current {
		return ErrWrongTurn
	}
	if len(g.LegalPlays(player)) > 0 {
		return ErrPassNotAllowed
	}
	opponent := otherPlayer(player)
	opponentHand := g.handFor(opponent)
	if len(opponentHand) == 0 {
		winner := opponent
		g.winner = &winner
		return nil
	}
	g.openingPending = true
	g.openingPlayer = opponent
	g.openingCard = nil
	g.openingResponder = player
	g.current = opponent
	return nil
}

func (g *LadderGame) PlayOpeningCard() error {
	if g == nil || g.winner != nil {
		return ErrGameOver
	}
	if !g.openingPending || g.openingCard == nil {
		return ErrOpeningPending
	}
	if g.openingPlayer != g.current {
		return ErrWrongTurn
	}
	if !validPlayer(g.openingPlayer) {
		return fmt.Errorf("%w: unknown player %d", ErrIllegalPlay, g.openingPlayer)
	}
	if cardIndex(g.handFor(g.openingPlayer), *g.openingCard) < 0 {
		return ErrCardNotInHand
	}
	player, opening := g.openingPlayer, *g.openingCard
	g.openingPending = false
	g.openingPlayer = 0
	g.openingCard = nil
	g.openingResponder = 0
	g.openChainWithCard(player, opening)
	if g.winner == nil {
		g.current = otherPlayer(player)
	}
	return nil
}

func (g *LadderGame) OpenStack(player PlayerID, card game.Card) error {
	if g == nil || g.winner != nil {
		return ErrGameOver
	}
	if !g.openingPending || g.openingCard != nil {
		return ErrOpeningPending
	}
	if !validPlayer(player) {
		return fmt.Errorf("%w: unknown player %d", ErrIllegalPlay, player)
	}
	if player != g.openingPlayer || player != g.current {
		return ErrWrongTurn
	}
	hand := g.handFor(player)
	index := cardIndex(hand, card)
	if index < 0 {
		return ErrCardNotInHand
	}
	responder := g.openingResponder
	hand = removeCardAt(hand, index)
	g.setHand(player, hand)
	g.playedCards = nil
	g.openingPending = false
	g.openingPlayer = 0
	g.openingResponder = 0
	g.setChainFromPlayedCard(card)
	if len(hand) == 0 {
		winner := player
		g.winner = &winner
		return nil
	}
	g.current = responder
	return nil
}

func (g *LadderGame) Winner() *PlayerID {
	if g == nil {
		return nil
	}
	return clonePlayerID(g.winner)
}

func (g *LadderGame) SharedStats() SharedStats {
	if g == nil {
		return SharedStats{}
	}
	return g.sharedStats
}

func Effective(card game.Card, chainSuit *game.Suit, shift, favor int) int {
	shifted := card.Face + shift
	if chainSuit != nil && card.Suit.Beats(*chainSuit) {
		return shifted*2 + favor
	}
	return shifted
}

func GreedyPolicy() Policy {
	return greedyPolicy{}
}

func (greedyPolicy) ChoosePlay(g *LadderGame, legal []game.Card) game.Card {
	candidates := append([]game.Card(nil), legal...)
	sort.Slice(candidates, func(i, j int) bool {
		leftWild, rightWild := false, false
		if g != nil {
			leftWild = candidates[i].Face == g.wildFaceRaw
			rightWild = candidates[j].Face == g.wildFaceRaw
		}
		if leftWild != rightWild {
			return !leftWild
		}
		if candidates[i].Face != candidates[j].Face {
			return candidates[i].Face < candidates[j].Face
		}
		return candidates[i].Suit < candidates[j].Suit
	})
	if len(candidates) == 0 {
		return game.Card{}
	}
	return candidates[0]
}

func (g *LadderGame) openFreshChain(player PlayerID) {
	hand := g.handFor(player)
	if len(hand) == 0 {
		winner := player
		g.winner = &winner
		return
	}
	g.openChainWithCard(player, chooseOpening(hand, g.wildFaceRaw))
}

func (g *LadderGame) openInitiativeChain(player PlayerID) bool {
	opening, found := lowestCardOfSuit(g.handFor(player), g.initiativeSuit)
	if !found {
		return false
	}
	g.openChainWithCard(player, opening)
	return true
}

func (g *LadderGame) openChainWithCard(player PlayerID, opening game.Card) {
	hand := g.handFor(player)
	g.setHand(player, removeCardAt(hand, cardIndex(hand, opening)))
	g.setChainFromPlayedCard(opening)
	if len(g.handFor(player)) == 0 {
		winner := player
		g.winner = &winner
	}
}

func (g *LadderGame) setChainFromPlayedCard(card game.Card) {
	g.playedCards = append(g.playedCards, card)
	g.chainCard = cloneCard(&card)
	if card.Face == g.wildFaceRaw {
		g.chainIsWild = true
		g.chainSuit = suitPointer(card.Suit)
		g.chainValue = nil
		return
	}
	value := card.Face + g.sharedStats.Shift
	g.chainIsWild = false
	g.chainValue = &value
	g.chainSuit = suitPointer(card.Suit)
}

func (g *LadderGame) handFor(player PlayerID) []game.Card {
	if player == PlayerA {
		return g.handA
	}
	return g.handB
}

func (g *LadderGame) setHand(player PlayerID, hand []game.Card) {
	if player == PlayerA {
		g.handA = hand
	} else {
		g.handB = hand
	}
}

func validateDeck(deck []game.Card) error {
	if len(deck) != len(game.NewDeck()) {
		return fmt.Errorf("Ladder requires exactly 27 cards, got %d", len(deck))
	}
	seen := make(map[game.Card]struct{}, len(deck))
	for _, card := range deck {
		if !card.Valid() {
			return fmt.Errorf("invalid Ladder card %v", card)
		}
		if _, ok := seen[card]; ok {
			return fmt.Errorf("duplicate Ladder card %s", card)
		}
		seen[card] = struct{}{}
	}
	return nil
}

func popCards(deck *[]game.Card, count int) []game.Card {
	start := len(*deck) - count
	cards := append([]game.Card(nil), (*deck)[start:]...)
	*deck = (*deck)[:start]
	for left, right := 0, len(cards)-1; left < right; left, right = left+1, right-1 {
		cards[left], cards[right] = cards[right], cards[left]
	}
	return cards
}

func popCard(deck *[]game.Card) game.Card {
	last := len(*deck) - 1
	card := (*deck)[last]
	*deck = (*deck)[:last]
	return card
}

func anyoneHolds(handA, handB []game.Card, suit game.Suit) bool {
	for _, hand := range [][]game.Card{handA, handB} {
		for _, card := range hand {
			if card.Suit == suit {
				return true
			}
		}
	}
	return false
}

func lowestCards(hand []game.Card, count int) []game.Card {
	cards := append([]game.Card(nil), hand...)
	sortCards(cards)
	if count > len(cards) {
		count = len(cards)
	}
	return cards[:count]
}

func removeCards(hand, removed []game.Card) []game.Card {
	remove := make(map[game.Card]struct{}, len(removed))
	for _, card := range removed {
		remove[card] = struct{}{}
	}
	result := make([]game.Card, 0, len(hand)-len(removed))
	for _, card := range hand {
		if _, ok := remove[card]; !ok {
			result = append(result, card)
		}
	}
	return result
}

func chooseLeader(handA, handB []game.Card, initiative game.Suit, noInitiative bool) PlayerID {
	if noInitiative {
		return lowestFaceLeader(handA, handB)
	}
	lowestA, hasA := lowestFaceOfSuit(handA, initiative)
	lowestB, hasB := lowestFaceOfSuit(handB, initiative)
	switch {
	case !hasA && !hasB:
		return lowestFaceLeader(handA, handB)
	case !hasA:
		return PlayerB
	case !hasB:
		return PlayerA
	case lowestA < lowestB:
		return PlayerA
	default:
		return PlayerB
	}
}

func lowestFaceLeader(handA, handB []game.Card) PlayerID {
	lowestA, lowestB := lowestFace(handA), lowestFace(handB)
	if lowestA <= lowestB {
		return PlayerA
	}
	return PlayerB
}

func lowestFaceOfSuit(hand []game.Card, suit game.Suit) (int, bool) {
	lowest, found := 0, false
	for _, card := range hand {
		if card.Suit == suit && (!found || card.Face < lowest) {
			lowest, found = card.Face, true
		}
	}
	return lowest, found
}

func lowestCardOfSuit(hand []game.Card, suit game.Suit) (game.Card, bool) {
	var lowest game.Card
	found := false
	for _, card := range hand {
		if card.Suit == suit && (!found || card.Face < lowest.Face) {
			lowest = card
			found = true
		}
	}
	return lowest, found
}

func lowestFace(hand []game.Card) int {
	lowest := 10
	for _, card := range hand {
		if card.Face < lowest {
			lowest = card.Face
		}
	}
	return lowest
}

func chooseOpening(hand []game.Card, wildFace int) game.Card {
	nonWild := make([]game.Card, 0, len(hand))
	for _, card := range hand {
		if card.Face != wildFace {
			nonWild = append(nonWild, card)
		}
	}
	candidates := hand
	if len(nonWild) > 0 {
		candidates = nonWild
	}
	sorted := append([]game.Card(nil), candidates...)
	sortCards(sorted)
	return sorted[0]
}

func sortCards(cards []game.Card) {
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Face != cards[j].Face {
			return cards[i].Face < cards[j].Face
		}
		return cards[i].Suit < cards[j].Suit
	})
}

func cardIndex(hand []game.Card, card game.Card) int {
	for index, held := range hand {
		if held == card {
			return index
		}
	}
	return -1
}

func removeCardAt(hand []game.Card, index int) []game.Card {
	copy(hand[index:], hand[index+1:])
	return hand[:len(hand)-1]
}

func containsCard(cards []game.Card, target game.Card) bool {
	for _, card := range cards {
		if card == target {
			return true
		}
	}
	return false
}

func otherPlayer(player PlayerID) PlayerID {
	if player == PlayerA {
		return PlayerB
	}
	return PlayerA
}

func validPlayer(player PlayerID) bool {
	return player == PlayerA || player == PlayerB
}

func floorHalf(value int) int {
	if value < 0 && value%2 != 0 {
		return value/2 - 1
	}
	return value / 2
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneCard(value *game.Card) *game.Card {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneSuit(value *game.Suit) *game.Suit {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func clonePlayerID(value *PlayerID) *PlayerID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func suitPointer(suit game.Suit) *game.Suit {
	return &suit
}
