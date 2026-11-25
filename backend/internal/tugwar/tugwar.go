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
	Cards  []game.Card `json:"cards"`
	Zone   int         `json:"zone"`
	Hidden bool        `json:"hidden,omitempty"`
}

func (g *Game) hasHiddenPile() bool {
	for _, pile := range g.State.Piles {
		if pile.Hidden {
			return true
		}
	}
	return false
}

func (p Pile) Top() (game.Card, bool) {
	if len(p.Cards) == 0 {
		return game.Card{}, false
	}
	return p.Cards[len(p.Cards)-1], true
}

type GameState struct {
	Deck           []game.Card    `json:"deck"`
	Discard        []game.Card    `json:"discard"`
	Hands          [2][]game.Card `json:"hands"`
	Piles          [NumPiles]Pile `json:"piles"`
	BountySuit     game.Suit      `json:"bountySuit"`
	BountyRevealed bool           `json:"bountyRevealed"`
	S              int            `json:"s"`
	F              int            `json:"f"`
	B              int            `json:"b"`
	Pushes         [2]int         `json:"pushes"`
	Current        int            `json:"current"`
	Round          int            `json:"round"`
	Passes         int            `json:"passes"`
	Over           bool           `json:"over"`
	Winner         int            `json:"winner"`
	Tiebreak       string         `json:"tiebreak,omitempty"`
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
	State   GameState        `json:"state"`
	Log     []ActionLogEntry `json:"log"`
	Mission *CampaignMission `json:"mission,omitempty"`
}

type MissionWinCondition string

const (
	WinByScore             MissionWinCondition = "score"
	WinByPositiveScorePile MissionWinCondition = "positive-score-pile"
	WinByAllPiles          MissionWinCondition = "all-piles"
	WinByPile              MissionWinCondition = "pile"
	WinByAnyPile           MissionWinCondition = "any-pile"
	WinByMostPiles         MissionWinCondition = "most-piles"
	WinByScoreMargin       MissionWinCondition = "score-margin"
)

type CPUPolicy string

const (
	CPUPolicyStandard     CPUPolicy = "standard"
	CPUPolicyPassive      CPUPolicy = "passive"
	CPUPolicyPushIt       CPUPolicy = "push-it"
	CPUPolicyDenial       CPUPolicy = "denial"
	CPUPolicyHighGround   CPUPolicy = "high-ground"
	CPUPolicySpreader     CPUPolicy = "spreader"
	CPUPolicyInfoGatherer CPUPolicy = "info-gatherer"
	CPUPolicyDominantSuit CPUPolicy = "dominant-suit"
	CPUPolicyBlitz        CPUPolicy = "blitz"
	CPUPolicyTunnelVision CPUPolicy = "tunnel-vision"
	CPUPolicyEfficient    CPUPolicy = "efficient"
	CPUPolicyExploitative CPUPolicy = "exploitative"
	CPUPolicyAdaptive     CPUPolicy = "adaptive"
	CPUPolicyBluffer      CPUPolicy = "bluffer"
	CPUPolicyEverything   CPUPolicy = "everything"
)

type CampaignMission struct {
	PlayerHand          []game.Card         `json:"playerHand,omitempty"`
	CPUHand             []game.Card         `json:"cpuHand,omitempty"`
	Piles               [NumPiles]Pile      `json:"piles,omitempty"`
	Deck                []game.Card         `json:"deck,omitempty"`
	BountySuit          game.Suit           `json:"bountySuit,omitempty"`
	Pushes              [2]int              `json:"pushes,omitempty"`
	HideBountyUntilFlip bool                `json:"hideBountyUntilFlip,omitempty"`
	RefillHands         bool                `json:"refillHands"`
	MaxRounds           int                 `json:"maxRounds,omitempty"`
	PlayerPushOneWay    bool                `json:"playerPushOneWay,omitempty"`
	WinCondition        MissionWinCondition `json:"winCondition,omitempty"`
	TargetPile          int                 `json:"targetPile,omitempty"`
	ScoreMargin         int                 `json:"scoreMargin,omitempty"`
	EarlyWin            bool                `json:"earlyWin,omitempty"`
	BestOf              int                 `json:"bestOf,omitempty"`
	CPUHeroes           []string            `json:"cpuHeroes,omitempty"`
	CPUPolicy           CPUPolicy           `json:"cpuPolicy,omitempty"`
	SeriesGames         int                 `json:"seriesGames,omitempty"`
	SeriesWinsNeeded    int                 `json:"seriesWinsNeeded,omitempty"`
	SeriesCPUHeroes     []string            `json:"seriesCPUHeroes,omitempty"`
	SeriesPolicies      []CPUPolicy         `json:"seriesPolicies,omitempty"`
	MirrorHero          bool                `json:"mirrorHero,omitempty"`
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
		B:       clamp(1+heroA.Bounty+heroB.Bounty, BountyMin, BountyMax),
	}
	pushesEach := 2 + clamp(heroA.Draft+heroB.Draft, DraftMin, DraftMax)
	state.Pushes = [2]int{pushesEach, pushesEach}

	deck := game.NewDeck()
	game.Shuffle(deck, rng)
	state.BountySuit = deck[len(deck)-1].Suit
	state.BountyRevealed = true
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

func NewCampaignGame(heroA, heroB heroes.Hero, rng *rand.Rand, mission CampaignMission) (*Game, error) {
	_, ok := heroes.ByID(heroA.ID)
	if !ok {
		return nil, fmt.Errorf("unknown first hero %q", heroA.ID)
	}
	_, ok = heroes.ByID(heroB.ID)
	if !ok {
		return nil, fmt.Errorf("unknown second hero %q", heroB.ID)
	}
	if rng == nil {
		return nil, errors.New("random source is required")
	}
	firstHero, secondHero := heroA, heroB

	gameState := GameState{
		Current: 0, Round: 1, Winner: -1,
		S:      min(firstHero.Shift+secondHero.Shift, ShiftCap),
		F:      firstHero.Favored + secondHero.Favored,
		B:      clamp(firstHero.Bounty+secondHero.Bounty, BountyMin, BountyMax),
		Pushes: mission.Pushes,
	}
	if len(mission.PlayerHand) == 0 && len(mission.CPUHand) == 0 {
		match, err := NewGame(firstHero, secondHero, rng)
		if err != nil {
			return nil, err
		}
		match.State.S = gameState.S
		match.State.F = gameState.F
		match.State.B = clamp(firstHero.Bounty+secondHero.Bounty, BountyMin, BountyMax)
		match.Mission = CloneCampaignMission(mission)
		return match, nil
	}
	if len(mission.PlayerHand) == 0 || len(mission.CPUHand) == 0 {
		return nil, errors.New("campaign Tug War missions must define both hands or neither")
	}
	if len(mission.PlayerHand) > HandSize || len(mission.CPUHand) > HandSize {
		return nil, errors.New("campaign Tug War hands exceed the hand size")
	}
	if !mission.BountySuit.Valid() {
		return nil, errors.New("campaign Tug War bounty suit is invalid")
	}
	gameState.Hands[0] = append([]game.Card(nil), mission.PlayerHand...)
	gameState.Hands[1] = append([]game.Card(nil), mission.CPUHand...)
	gameState.Piles = mission.Piles
	for index := range gameState.Piles {
		gameState.Piles[index].Cards = append([]game.Card(nil), mission.Piles[index].Cards...)
	}
	for _, pile := range gameState.Piles {
		if pile.Zone < -1 || pile.Zone > 1 {
			return nil, errors.New("campaign Tug War pile zone is invalid")
		}
		if len(pile.Cards) == 0 {
			return nil, errors.New("campaign Tug War piles cannot be empty")
		}
		for _, card := range pile.Cards {
			if !card.Valid() {
				return nil, errors.New("campaign Tug War pile contains an invalid card")
			}
		}
	}
	for _, hand := range gameState.Hands {
		for _, card := range hand {
			if !card.Valid() {
				return nil, errors.New("campaign Tug War hand contains an invalid card")
			}
		}
	}
	for _, card := range mission.Deck {
		if !card.Valid() {
			return nil, errors.New("campaign Tug War deck contains an invalid card")
		}
	}
	gameState.Deck = reverseCards(mission.Deck)
	gameState.BountySuit = mission.BountySuit
	if mission.HideBountyUntilFlip {
		gameState.BountyRevealed = false
	} else {
		gameState.BountyRevealed = true
	}
	return &Game{State: gameState, Mission: CloneCampaignMission(mission)}, nil
}

func CloneCampaignMission(mission CampaignMission) *CampaignMission {
	mission.PlayerHand = append([]game.Card(nil), mission.PlayerHand...)
	mission.CPUHand = append([]game.Card(nil), mission.CPUHand...)
	mission.Deck = append([]game.Card(nil), mission.Deck...)
	mission.CPUHeroes = append([]string(nil), mission.CPUHeroes...)
	mission.SeriesCPUHeroes = append([]string(nil), mission.SeriesCPUHeroes...)
	mission.SeriesPolicies = append([]CPUPolicy(nil), mission.SeriesPolicies...)
	for index := range mission.Piles {
		mission.Piles[index].Cards = append([]game.Card(nil), mission.Piles[index].Cards...)
	}
	return &mission
}

func reverseCards(cards []game.Card) []game.Card {
	reversed := make([]game.Card, len(cards))
	for index, card := range cards {
		reversed[len(cards)-1-index] = card
	}
	return reversed
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

// CanPlayCard reports whether this exact hand card can beat this pile.
func (g *Game) CanPlayCard(player, cardIndex, pileIndex int) bool {
	if g == nil || g.State.Over || player < 0 || player > 1 || player != g.State.Current ||
		cardIndex < 0 || cardIndex >= len(g.State.Hands[player]) || pileIndex < 0 || pileIndex >= len(g.State.Piles) {
		return false
	}
	top, ok := g.State.Piles[pileIndex].Top()
	return ok && CardBeatsTop(g.State.Hands[player][cardIndex], top, g.State.S, g.State.F)
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
	if len(g.State.Deck) > 0 || g.HasWinningPlay(player) {
		return true
	}
	for pileIndex := range g.State.Piles {
		if g.CanSpendPush(player, pileIndex) {
			return true
		}
	}
	return false
}

func (g *Game) CanSpendPush(player, pileIndex int) bool {
	if g == nil || player < 0 || player > 1 || player != g.State.Current ||
		pileIndex < 0 || pileIndex >= len(g.State.Piles) || g.State.Over ||
		g.State.Pushes[player] <= 0 {
		return false
	}
	if len(g.State.Deck) == 0 && g.Mission == nil {
		return false
	}
	if g.Mission != nil && player == 0 && g.Mission.PlayerPushOneWay &&
		g.State.Piles[pileIndex].Zone == 1 {
		return false
	}
	return g.State.Piles[pileIndex].Zone != playerZone(player)
}

func (g *Game) CanPass() bool {
	if g == nil || g.State.Over {
		return false
	}
	if g.Mission != nil && missionCPUCanPass(g, g.Mission.CPUPolicy) {
		return true
	}
	return !g.HasLegalAction(g.State.Current)
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
		if !g.CanPlayCard(player, action.CardIndex, action.PileIndex) {
			return errors.New("that card cannot beat the selected pile")
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
		log.Succeeded = true
		pile.Cards = append(pile.Cards, card)
		pile.Zone = moveZone(pile.Zone, player)
		pile.Hidden = false
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
		g.State.BountyRevealed = true
		pile.Hidden = false
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
		log.Succeeded = true
		pile.Zone = moveZone(pile.Zone, player)
		log.ZoneAfter = pile.Zone
	case ActionPass:
		if !g.CanPass() {
			return errors.New("passing is only allowed when no legal action is available")
		}
		g.State.Passes++
		g.State.Passes = min(g.State.Passes, 2)
		g.Log = append(g.Log, log)
		if g.State.Passes >= 2 {
			if g.Mission != nil && g.Mission.MaxRounds > g.State.Round {
				g.finishRound(&g.Log[len(g.Log)-1])
			} else {
				g.finish()
			}
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
	g.checkEarlyWin()
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
	g.finishRound(log)
}

func (g *Game) finishRound(log *ActionLogEntry) {
	g.State.Passes = 0
	if g.Mission == nil || g.Mission.RefillHands {
		for hand := range g.State.Hands {
			for len(g.State.Hands[hand]) < HandSize && len(g.State.Deck) > 0 {
				last := len(g.State.Deck) - 1
				g.State.Hands[hand] = append(g.State.Hands[hand], g.State.Deck[last])
				g.State.Deck = g.State.Deck[:last]
				if log != nil {
					log.Refilled[hand]++
				}
			}
		}
	}
	g.State.Round++
	g.State.Current = (g.State.Round + 1) % 2
	if g.Mission != nil && g.Mission.MaxRounds > 0 && g.State.Round > g.Mission.MaxRounds {
		g.finish()
	}
}

func (g *Game) finish() {
	if g == nil {
		return
	}
	g.State.Over = true
	result := g.Result()
	g.State.Winner, g.State.Tiebreak = result.Winner, result.Tiebreak
}

func (g *Game) checkEarlyWin() {
	if g == nil || g.Mission == nil || !g.Mission.EarlyWin || g.State.Over {
		return
	}
	if missionObjectiveMet(g, 0) {
		g.finish()
	}
}

func missionObjectiveMet(g *Game, player int) bool {
	if g == nil || g.Mission == nil {
		return false
	}
	result := g.scoreResult()
	owned := 0
	for _, pile := range g.State.Piles {
		if pile.Zone == playerZone(player) {
			owned++
		}
	}
	switch g.Mission.WinCondition {
	case WinByPositiveScorePile:
		return result.Scores[player] > 0 && owned > 0
	case WinByAllPiles:
		return owned == NumPiles
	case WinByPile:
		return g.Mission.TargetPile >= 0 && g.Mission.TargetPile < NumPiles &&
			g.State.Piles[g.Mission.TargetPile].Zone == playerZone(player)
	case WinByAnyPile:
		return owned > 0
	case WinByMostPiles:
		opponentOwned := 0
		for _, pile := range g.State.Piles {
			if pile.Zone == playerZone(1-player) {
				opponentOwned++
			}
		}
		return owned > opponentOwned
	case WinByScoreMargin:
		return result.Scores[player] > result.Scores[1-player]+g.Mission.ScoreMargin
	case WinByScore:
		return result.Scores[player] > result.Scores[1-player]
	case "":
		return result.Winner == player
	default:
		return false
	}
}

func (g *Game) Result() Result {
	result := g.scoreResult()
	if g != nil && g.Mission != nil && g.State.Over && g.Mission.WinCondition != "" {
		if missionObjectiveMet(g, 0) {
			result.Winner = 0
			result.Tiebreak = "campaign-objective"
		} else {
			result.Winner = 1
			result.Tiebreak = "campaign-objective"
		}
	}
	return result
}

func (g *Game) scoreResult() Result {
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
	if g.State.Pushes[player] > 0 {
		for pileIndex, pile := range g.State.Piles {
			if !g.CanSpendPush(player, pileIndex) {
				continue
			}
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

func (g *Game) ChooseCampaignCPUAction(rng *rand.Rand) Action {
	if g == nil || g.Mission == nil || g.State.Current != 1 {
		return g.ChooseCPUAction()
	}
	policy := g.Mission.CPUPolicy
	switch policy {
	case CPUPolicyPassive:
		return g.chooseLowestWinningPlay(-1, false)
	case CPUPolicyPushIt, CPUPolicyEfficient:
		if action := g.chooseLowestWinningPlay(-1, false); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignFlip()
	case CPUPolicyHighGround:
		if action := g.chooseLowestWinningPlay(0, false); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseLowestWinningPlay(-1, false); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignFlip()
	case CPUPolicyDenial:
		if action := g.choosePolicyPlay(func(card game.Card, pileIndex int) bool {
			return card.Suit == g.State.BountySuit && g.State.Piles[pileIndex].Zone != 1
		}, false); action.Kind != ActionPass {
			return action
		}
		if action := g.choosePolicyPlay(func(card game.Card, pileIndex int) bool {
			return card.Suit != g.State.BountySuit && g.State.Piles[pileIndex].Zone == 1
		}, true); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseHighestWinningPlay(-1); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseBestPush(func(pile Pile) bool {
			return pile.Zone >= 0 && countSuit(pile.Cards, g.State.BountySuit) >= 2
		}); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignFlip()
	case CPUPolicySpreader:
		if action := g.chooseSpreaderPush(); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseSpreaderPlay(); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignFlip()
	case CPUPolicyInfoGatherer:
		if g.State.Round <= 2 {
			if action := g.chooseHiddenFlip(); action.Kind != ActionPass {
				return action
			}
		}
		return g.chooseCampaignStandardAction()
	case CPUPolicyDominantSuit:
		if action := g.choosePolicyPlay(func(card game.Card, _ int) bool {
			return card.Suit == game.Shield
		}, true); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseBestPush(func(pile Pile) bool { return pile.Zone == 0 }); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignFlip()
	case CPUPolicyBlitz:
		if action := g.chooseBestPushByStack(func(Pile) bool { return true }); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseHighestWinningPlay(-1); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignFlip()
	case CPUPolicyTunnelVision:
		if action := g.choosePushAt(1); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseHighestWinningPlay(1); action.Kind != ActionPass {
			return action
		}
		if len(g.State.Deck) > 0 {
			return Action{Kind: ActionFlipDeck, PileIndex: 1}
		}
		return Action{Kind: ActionPass}
	case CPUPolicyExploitative:
		if action := g.chooseBestPush(func(pile Pile) bool { return pile.Zone == -1 }); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignStandardAction()
	case CPUPolicyAdaptive:
		if g.State.Round < 3 {
			return g.chooseCampaignStandardAction()
		}
		if g.playerIsAggressive() {
			if action := g.chooseBestPush(func(pile Pile) bool { return pile.Zone == -1 }); action.Kind != ActionPass {
				return action
			}
			return g.chooseCampaignStandardAction()
		}
		if action := g.chooseBestPush(func(pile Pile) bool { return pile.Zone == 0 }); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignStandardAction()
	case CPUPolicyBluffer:
		if rng != nil && rng.Intn(10) < 3 {
			if action := g.chooseSuboptimalBait(rng); action.Kind != ActionPass {
				return action
			}
		}
		if action := g.chooseRandomPushAmong(rng, 0, 2); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseLowestWinningPlay(0, false); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseLowestWinningPlay(2, false); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseLowestWinningPlay(-1, false); action.Kind != ActionPass {
			return action
		}
		if action := g.chooseCampaignFlip(); action.Kind != ActionPass {
			return action
		}
		return Action{Kind: ActionPass}
	case CPUPolicyEverything:
		if rng != nil && rng.Intn(10) == 0 {
			if action := g.chooseSuboptimalBait(rng); action.Kind != ActionPass {
				return action
			}
		}
		if action := g.chooseEverythingPush(rng); action.Kind != ActionPass {
			return action
		}
		if action := g.choosePolicyPlay(func(card game.Card, _ int) bool {
			return card.Suit == g.dominantCampaignSuit()
		}, true); action.Kind != ActionPass {
			return action
		}
		return g.chooseCampaignStandardAction()
	}
	return g.chooseCampaignStandardAction()
}

func (g *Game) chooseCampaignStandardAction() Action {
	if g.hasHiddenPile() {
		if action := g.chooseLowestWinningPlay(-1, false); action.Kind != ActionPass {
			return action
		}
		for pileIndex, pile := range g.State.Piles {
			if pile.Hidden {
				continue
			}
			if g.CanSpendPush(1, pileIndex) {
				return Action{Kind: ActionSpendPush, PileIndex: pileIndex}
			}
		}
		return g.chooseCampaignFlip()
	}
	action := g.ChooseCPUAction()
	if action.Kind == ActionFlipDeck {
		return g.chooseCampaignFlip()
	}
	return action
}

func (g *Game) chooseCampaignFlip() Action {
	if g == nil || len(g.State.Deck) == 0 {
		return Action{Kind: ActionPass}
	}
	target, height := 0, -1
	for index, pile := range g.State.Piles {
		if pile.Hidden {
			return Action{Kind: ActionFlipDeck, PileIndex: index}
		}
		if len(pile.Cards) > height {
			target, height = index, len(pile.Cards)
		}
	}
	return Action{Kind: ActionFlipDeck, PileIndex: target}
}

func (g *Game) chooseLowestWinningPlay(targetPile int, highest bool) Action {
	return g.choosePolicyPlay(func(_ game.Card, pileIndex int) bool {
		return targetPile < 0 || targetPile == pileIndex
	}, highest)
}

func (g *Game) chooseHighestWinningPlay(targetPile int) Action {
	return g.chooseLowestWinningPlay(targetPile, true)
}

func (g *Game) choosePolicyPlay(accept func(game.Card, int) bool, highest bool) Action {
	if g == nil || accept == nil {
		return Action{Kind: ActionPass}
	}
	best := Action{Kind: ActionPass}
	bestFace := 0
	for cardIndex, card := range g.State.Hands[1] {
		for pileIndex, pile := range g.State.Piles {
			if pile.Hidden {
				continue
			}
			top, ok := pile.Top()
			if !ok || !CardBeatsTop(card, top, g.State.S, g.State.F) || !accept(card, pileIndex) {
				continue
			}
			if best.Kind == ActionPass || highest && card.Face > bestFace ||
				!highest && card.Face < bestFace ||
				card.Face == bestFace && len(pile.Cards) > len(g.State.Piles[best.PileIndex].Cards) ||
				card.Face == bestFace && len(pile.Cards) == len(g.State.Piles[best.PileIndex].Cards) && pileIndex < best.PileIndex {
				best, bestFace = Action{Kind: ActionPlayCard, CardIndex: cardIndex, PileIndex: pileIndex}, card.Face
			}
		}
	}
	return best
}

func (g *Game) chooseBestPush(accept func(Pile) bool) Action {
	if g == nil || accept == nil {
		return Action{Kind: ActionPass}
	}
	bestPile, bestValue := -1, -1
	for index, pile := range g.State.Piles {
		if !accept(pile) || !g.CanSpendPush(1, index) {
			continue
		}
		value := PileValue(pile, g.State.BountySuit, g.State.B)
		if value > bestValue {
			bestPile, bestValue = index, value
		}
	}
	if bestPile >= 0 {
		return Action{Kind: ActionSpendPush, PileIndex: bestPile}
	}
	return Action{Kind: ActionPass}
}

func (g *Game) chooseBestPushByStack(accept func(Pile) bool) Action {
	if g == nil || accept == nil {
		return Action{Kind: ActionPass}
	}
	bestPile, bestSize := -1, -1
	for index, pile := range g.State.Piles {
		if !accept(pile) || !g.CanSpendPush(1, index) {
			continue
		}
		if len(pile.Cards) > bestSize || len(pile.Cards) == bestSize && (bestPile < 0 || index < bestPile) {
			bestPile, bestSize = index, len(pile.Cards)
		}
	}
	if bestPile >= 0 {
		return Action{Kind: ActionSpendPush, PileIndex: bestPile}
	}
	return Action{Kind: ActionPass}
}

func (g *Game) chooseSpreaderPush() Action {
	bestPile, fewestCards, bestValue := -1, int(^uint(0)>>1), -1
	for index, pile := range g.State.Piles {
		if pile.Zone != 0 || !g.CanSpendPush(1, index) {
			continue
		}
		cpuCards := g.playedCardCount(1, index)
		value := PileValue(pile, g.State.BountySuit, g.State.B)
		if cpuCards < fewestCards || cpuCards == fewestCards && value > bestValue ||
			cpuCards == fewestCards && value == bestValue && (bestPile < 0 || index < bestPile) {
			bestPile, fewestCards, bestValue = index, cpuCards, value
		}
	}
	if bestPile >= 0 {
		return Action{Kind: ActionSpendPush, PileIndex: bestPile}
	}
	return Action{Kind: ActionPass}
}

func (g *Game) chooseSpreaderPlay() Action {
	best := Action{Kind: ActionPass}
	bestFace, fewestCards, tallest := 0, int(^uint(0)>>1), -1
	for cardIndex, card := range g.State.Hands[1] {
		for pileIndex, pile := range g.State.Piles {
			if pile.Zone == 1 || pile.Hidden {
				continue
			}
			top, ok := pile.Top()
			if !ok || !CardBeatsTop(card, top, g.State.S, g.State.F) {
				continue
			}
			cpuCards := g.playedCardCount(1, pileIndex)
			if best.Kind == ActionPass || card.Face < bestFace ||
				card.Face == bestFace && cpuCards < fewestCards ||
				card.Face == bestFace && cpuCards == fewestCards && len(pile.Cards) > tallest ||
				card.Face == bestFace && cpuCards == fewestCards && len(pile.Cards) == tallest && pileIndex < best.PileIndex {
				best = Action{Kind: ActionPlayCard, CardIndex: cardIndex, PileIndex: pileIndex}
				bestFace, fewestCards, tallest = card.Face, cpuCards, len(pile.Cards)
			}
		}
	}
	return best
}

func (g *Game) playedCardCount(player, pileIndex int) int {
	count := 0
	for _, entry := range g.Log {
		if entry.Player == player && entry.PileIndex == pileIndex &&
			entry.Kind == ActionPlayCard && entry.Succeeded {
			count++
		}
	}
	return count
}

func (g *Game) choosePushAt(pileIndex int) Action {
	if g == nil || pileIndex < 0 || pileIndex >= len(g.State.Piles) ||
		!g.CanSpendPush(1, pileIndex) {
		return Action{Kind: ActionPass}
	}
	return Action{Kind: ActionSpendPush, PileIndex: pileIndex}
}

func (g *Game) chooseHiddenFlip() Action {
	bestPile, bestSize := -1, -1
	for index, pile := range g.State.Piles {
		if !pile.Hidden {
			continue
		}
		if len(pile.Cards) > bestSize {
			bestPile, bestSize = index, len(pile.Cards)
		}
	}
	if bestPile >= 0 && len(g.State.Deck) > 0 {
		return Action{Kind: ActionFlipDeck, PileIndex: bestPile}
	}
	return Action{Kind: ActionPass}
}

func (g *Game) chooseSuboptimalBait(rng *rand.Rand) Action {
	if g == nil || rng == nil {
		return Action{Kind: ActionPass}
	}
	candidates := make([]Action, 0)
	for cardIndex := range g.State.Hands[1] {
		for pileIndex := range g.State.Piles {
			if (pileIndex == 1 || pileIndex == 2) && g.CanPlayCard(1, cardIndex, pileIndex) {
				candidates = append(candidates, Action{Kind: ActionPlayCard, CardIndex: cardIndex, PileIndex: pileIndex})
			}
		}
	}
	if len(candidates) == 0 {
		return Action{Kind: ActionPass}
	}
	candidate := candidates[rng.Intn(len(candidates))]
	if g.HasLegalAction(1) {
		return candidate
	}
	return Action{Kind: ActionPass}
}

func (g *Game) chooseRandomPushAmong(rng *rand.Rand, first, second int) Action {
	if rng != nil && rng.Intn(2) == 1 {
		first, second = second, first
	}
	if action := g.choosePushAt(first); action.Kind != ActionPass {
		return action
	}
	return g.choosePushAt(second)
}

func (g *Game) playerIsAggressive() bool {
	total, aggressive := 0, 0
	for _, entry := range g.Log {
		if entry.Player != 0 {
			continue
		}
		total++
		if entry.Kind == ActionSpendPush || entry.Kind == ActionFlipDeck {
			aggressive++
		}
	}
	return total > 0 && aggressive*2 > total
}

func (g *Game) chooseEverythingPush(rng *rand.Rand) Action {
	if g.State.Round >= 4 || len(g.State.Deck) == 0 {
		return g.chooseBestPush(func(pile Pile) bool { return pile.Zone == 0 })
	}
	lockPile := rng == nil || rng.Intn(10) < 6
	if lockPile {
		if action := g.chooseBestPush(func(pile Pile) bool { return pile.Zone == 0 }); action.Kind != ActionPass {
			return action
		}
		return g.chooseBestPush(func(pile Pile) bool { return pile.Zone == -1 })
	}
	if action := g.chooseBestPush(func(pile Pile) bool { return pile.Zone == -1 }); action.Kind != ActionPass {
		return action
	}
	return g.chooseBestPush(func(pile Pile) bool { return pile.Zone == 0 })
}

func (g *Game) dominantCampaignSuit() game.Suit {
	bestSuit, bestWins := game.Shield, -1
	for _, suit := range []game.Suit{game.Shield, game.Blade, game.Fang} {
		wins := 0
		for _, card := range g.State.Hands[1] {
			if card.Suit != suit {
				continue
			}
			for _, pile := range g.State.Piles {
				if top, ok := pile.Top(); ok && CardBeatsTop(card, top, g.State.S, g.State.F) {
					wins++
					break
				}
			}
		}
		if wins > bestWins {
			bestSuit, bestWins = suit, wins
		}
	}
	return bestSuit
}

func missionCPUCanPass(g *Game, policy CPUPolicy) bool {
	if g == nil || g.State.Current != 1 {
		return false
	}
	if policy == CPUPolicyTunnelVision {
		return !hasActionForPile(g, 1)
	}
	if policy == CPUPolicyPassive {
		return !g.HasWinningPlay(1)
	}
	if policy == CPUPolicyDominantSuit {
		if len(g.State.Deck) > 0 {
			return false
		}
		for index, pile := range g.State.Piles {
			if pile.Zone == 0 && g.CanSpendPush(1, index) {
				return false
			}
		}
		return g.choosePolicyPlay(func(card game.Card, _ int) bool {
			return card.Suit == game.Shield
		}, true).Kind == ActionPass
	}
	if policy == CPUPolicyInfoGatherer && len(g.State.Deck) == 0 {
		for index, pile := range g.State.Piles {
			if !pile.Hidden && g.CanSpendPush(1, index) {
				return false
			}
		}
		return g.choosePolicyPlay(func(game.Card, int) bool { return true }, false).Kind == ActionPass
	}
	if policy == CPUPolicyDenial || policy == CPUPolicySpreader {
		if len(g.State.Deck) > 0 {
			return false
		}
		if policy == CPUPolicyDenial {
			if g.choosePolicyPlay(func(card game.Card, pileIndex int) bool {
				return card.Suit == g.State.BountySuit && g.State.Piles[pileIndex].Zone != 1
			}, false).Kind != ActionPass ||
				g.choosePolicyPlay(func(card game.Card, pileIndex int) bool {
					return card.Suit != g.State.BountySuit && g.State.Piles[pileIndex].Zone == 1
				}, true).Kind != ActionPass {
				return false
			}
			for index, pile := range g.State.Piles {
				if pile.Zone >= 0 && countSuit(pile.Cards, g.State.BountySuit) >= 2 &&
					g.CanSpendPush(1, index) {
					return false
				}
			}
			return true
		}
		if g.chooseSpreaderPlay().Kind != ActionPass || g.chooseSpreaderPush().Kind != ActionPass {
			return false
		}
		return true
	}
	if policy == CPUPolicyBluffer && len(g.State.Deck) == 0 {
		if g.chooseLowestWinningPlay(-1, false).Kind != ActionPass ||
			g.choosePushAt(0).Kind != ActionPass || g.choosePushAt(2).Kind != ActionPass {
			return false
		}
		return true
	}
	return false
}

func hasActionForPile(g *Game, pileIndex int) bool {
	if len(g.State.Deck) > 0 {
		return true
	}
	if g.CanSpendPush(1, pileIndex) {
		return true
	}
	top, ok := g.State.Piles[pileIndex].Top()
	if !ok {
		return false
	}
	for _, card := range g.State.Hands[1] {
		if CardBeatsTop(card, top, g.State.S, g.State.F) {
			return true
		}
	}
	return false
}

func countSuit(cards []game.Card, suit game.Suit) int {
	count := 0
	for _, card := range cards {
		if card.Suit == suit {
			count++
		}
	}
	return count
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
