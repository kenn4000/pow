package ladder

import (
	"errors"
	"reflect"
	"testing"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

func TestSharedStatsForUsesSymmetricModifiersAndFloorAverage(t *testing.T) {
	first := heroes.Hero{Bounty: -1, Draft: -2, Shift: -3, Favored: -2}
	second := heroes.Hero{Bounty: 0, Draft: 1, Shift: 0, Favored: 1}
	if got, want := SharedStatsFor(first, second), (SharedStats{
		Bounty: -1, Draft: 0, Shift: 0, Favor: -1,
	}); got != want {
		t.Fatalf("shared stats = %+v, want %+v", got, want)
	}

	first = heroes.Hero{Bounty: 8, Draft: 2, Shift: 9, Favored: 3}
	second = heroes.Hero{Bounty: 8, Draft: 3, Shift: 9, Favored: 4}
	if got, want := SharedStatsFor(first, second), (SharedStats{
		Bounty: 3, Draft: 5, Shift: 4, Favor: 7,
	}); got != want {
		t.Fatalf("clamped shared stats = %+v, want %+v", got, want)
	}
}

func TestEffectiveAppliesFavorOnlyToIncomingSuitWin(t *testing.T) {
	chain := game.Fang
	tests := []struct {
		name  string
		card  game.Card
		suit  *game.Suit
		shift int
		favor int
		want  int
	}{
		{name: "suit win with negative favor", card: game.Card{Suit: game.Blade, Face: 3}, suit: &chain, shift: 2, favor: -4, want: 6},
		{name: "same suit", card: game.Card{Suit: game.Fang, Face: 3}, suit: &chain, shift: 2, favor: 4, want: 5},
		{name: "losing suit", card: game.Card{Suit: game.Shield, Face: 3}, suit: &chain, shift: 2, favor: 4, want: 5},
		{name: "fresh chain", card: game.Card{Suit: game.Blade, Face: 3}, shift: 2, favor: 4, want: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Effective(test.card, test.suit, test.shift, test.favor); got != test.want {
				t.Fatalf("Effective() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestNewLadderGameIsDeterministicAndDoesNotMutateDeck(t *testing.T) {
	heroA, _ := heroes.ByID("recruit")
	heroB, _ := heroes.ByID("warden")
	deck := game.NewDeck()
	original := append([]game.Card(nil), deck...)
	first, err := NewLadderGame(heroA, heroB, nil, 44)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewLadderGame(heroA, heroB, nil, 44)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.State(), second.State()) {
		t.Fatal("same seed produced different starting states")
	}
	provided, err := NewLadderGame(heroA, heroB, deck, 44)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deck, original) {
		t.Fatal("constructor mutated the caller's deck")
	}
	state := provided.State()
	if len(state.HandA) != 8 || len(state.HandB) != 9 || state.WildFaceRaw != 9 {
		t.Fatalf("unexpected ordered-deck setup: hand sizes %d/%d wild %d",
			len(state.HandA), len(state.HandB), state.WildFaceRaw)
	}
	if !state.FallbackNoInitiativeSuit || state.Current != PlayerB || state.ChainCard == nil ||
		*state.ChainCard != (game.Card{Suit: game.Shield, Face: 1}) ||
		state.ChainValue == nil || *state.ChainValue != 1 ||
		state.ChainSuit == nil || *state.ChainSuit != game.Shield {
		t.Fatalf("unexpected fallback opening state: %+v", state)
	}
}

func TestNewLadderGameAutoPlaysOnlyInitiativeCard(t *testing.T) {
	hero, _ := heroes.ByID("recruit")
	handA := []game.Card{
		{Suit: game.Blade, Face: 1},
		{Suit: game.Fang, Face: 1}, {Suit: game.Fang, Face: 2},
		{Suit: game.Fang, Face: 3}, {Suit: game.Fang, Face: 4},
		{Suit: game.Shield, Face: 1}, {Suit: game.Shield, Face: 2},
		{Suit: game.Shield, Face: 3}, {Suit: game.Shield, Face: 4},
	}
	handB := []game.Card{
		{Suit: game.Fang, Face: 5}, {Suit: game.Fang, Face: 6},
		{Suit: game.Fang, Face: 7}, {Suit: game.Fang, Face: 8},
		{Suit: game.Fang, Face: 9}, {Suit: game.Shield, Face: 5},
		{Suit: game.Shield, Face: 6}, {Suit: game.Shield, Face: 7},
		{Suit: game.Shield, Face: 8},
	}
	used := make(map[game.Card]struct{}, len(handA)+len(handB)+1)
	for _, card := range append(append([]game.Card{}, handA...), handB...) {
		used[card] = struct{}{}
	}
	bounty := game.Card{Suit: game.Blade, Face: 9}
	used[bounty] = struct{}{}
	deck := make([]game.Card, 0, len(game.NewDeck()))
	for _, card := range game.NewDeck() {
		if _, ok := used[card]; !ok {
			deck = append(deck, card)
		}
	}
	deck = append(deck, bounty)
	for index := len(handB) - 1; index >= 0; index-- {
		deck = append(deck, handB[index])
	}
	for index := len(handA) - 1; index >= 0; index-- {
		deck = append(deck, handA[index])
	}

	match, err := NewLadderGame(hero, hero, deck, 1)
	if err != nil {
		t.Fatal(err)
	}
	state := match.State()
	if state.Current != PlayerB || state.ChainCard == nil ||
		*state.ChainCard != handA[0] || !reflect.DeepEqual(state.PlayedCards, []game.Card{handA[0]}) {
		t.Fatalf("single initiative card was not auto-played: %+v", state)
	}
}

func TestDeferredOpeningPreservesAndThenPlaysTheAutomaticCard(t *testing.T) {
	heroA, _ := heroes.ByID("recruit")
	heroB, _ := heroes.ByID("warden")
	automatic, err := NewLadderGame(heroA, heroB, nil, 44)
	if err != nil {
		t.Fatal(err)
	}
	deferred, err := NewLadderGameWithDeferredOpening(heroA, heroB, nil, 44)
	if err != nil {
		t.Fatal(err)
	}
	before := deferred.State()
	automaticState := automatic.State()
	openingHand, otherHand := before.HandA, before.HandB
	automaticOpeningHand, automaticOtherHand := automaticState.HandA, automaticState.HandB
	if before.OpeningPlayer == PlayerB {
		openingHand, otherHand = before.HandB, before.HandA
		automaticOpeningHand, automaticOtherHand = automaticState.HandB, automaticState.HandA
	}
	if !before.OpeningPending || before.OpeningPlayer != otherPlayer(automaticState.Current) ||
		before.OpeningCard == nil || automaticState.ChainCard == nil ||
		*before.OpeningCard != *automaticState.ChainCard ||
		len(before.PlayedCards) != 0 || len(openingHand) != len(automaticOpeningHand)+1 ||
		len(otherHand) != len(automaticOtherHand) {
		t.Fatalf("deferred opening did not preserve the original hand: deferred=%+v automatic=%+v",
			before, automaticState)
	}
	if err := deferred.Play(before.OpeningPlayer, *before.OpeningCard); !errors.Is(err, ErrOpeningPending) {
		t.Fatalf("Play while opening pending returned %v, want ErrOpeningPending", err)
	}
	if err := deferred.PlayOpeningCard(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deferred.State(), automaticState) {
		t.Fatalf("playing deferred opening changed initial state: got=%+v want=%+v",
			deferred.State(), automaticState)
	}
}

func TestNewLadderGameRejectsInvalidDecks(t *testing.T) {
	hero, _ := heroes.ByID("recruit")
	tests := []struct {
		name string
		deck []game.Card
	}{
		{name: "short", deck: game.NewDeck()[:26]},
		{name: "duplicate", deck: func() []game.Card {
			deck := game.NewDeck()
			deck[len(deck)-1] = deck[0]
			return deck
		}()},
		{name: "invalid card", deck: func() []game.Card {
			deck := game.NewDeck()
			deck[0].Face = 10
			return deck
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewLadderGame(hero, hero, test.deck, 1); err == nil {
				t.Fatal("accepted invalid deck")
			}
		})
	}
}

func TestNewLadderGameSwapsDraftCardsBeforeSelectingLeader(t *testing.T) {
	heroA, _ := heroes.ByID("gambler")
	heroB, _ := heroes.ByID("warden")
	g, err := NewLadderGame(heroA, heroB, game.NewDeck(), 2)
	if err != nil {
		t.Fatal(err)
	}
	state := g.State()
	if state.SharedStats != (SharedStats{Bounty: 1, Draft: 1, Favor: 1}) {
		t.Fatalf("shared stats = %+v", state.SharedStats)
	}
	if len(state.HandA) != 10 || len(state.HandB) != 9 ||
		containsCard(state.HandA, game.Card{Suit: game.Shield, Face: 1}) ||
		!containsCard(state.HandA, game.Card{Suit: game.Fang, Face: 1}) ||
		!containsCard(state.HandB, game.Card{Suit: game.Shield, Face: 1}) ||
		containsCard(state.HandB, game.Card{Suit: game.Fang, Face: 1}) {
		t.Fatalf("draft did not exchange each player's lowest card: hands=%v/%v", state.HandA, state.HandB)
	}
	if state.InitiativeSuit != game.Blade || state.FallbackNoInitiativeSuit {
		t.Fatalf("leader was not selected from the drafted initiative-suit cards: %+v", state)
	}
	if state.ChainCard == nil || state.ChainCard.Suit != state.InitiativeSuit {
		t.Fatalf("opening card does not match initiative suit: %+v", state)
	}
	for _, card := range append(append([]game.Card(nil), state.HandA...), state.HandB...) {
		if card.Suit == state.InitiativeSuit && card.Face < state.ChainCard.Face {
			t.Fatalf("opening card %v is not the lowest initiative card", *state.ChainCard)
		}
	}
}

func TestStateReturnsDeepCopies(t *testing.T) {
	value := 5
	suit := game.Blade
	winner := PlayerA
	g := &LadderGame{
		handA:       []game.Card{{Suit: game.Blade, Face: 1}},
		handB:       []game.Card{{Suit: game.Fang, Face: 2}},
		playedCards: []game.Card{{Suit: game.Fang, Face: 2}},
		chainValue:  &value, chainSuit: &suit, winner: &winner,
	}
	state := g.State()
	state.HandA[0].Face = 9
	state.PlayedCards[0].Face = 9
	*state.ChainValue = 1
	*state.ChainSuit = game.Shield
	*state.Winner = PlayerB
	if g.handA[0].Face != 1 || g.playedCards[0].Face != 2 ||
		*g.chainValue != 5 || *g.chainSuit != game.Blade ||
		*g.winner != PlayerA {
		t.Fatal("State() exposed mutable internal data")
	}
}

func TestLegalPlaysEnforceNumericAndWildChains(t *testing.T) {
	chainSuit := game.Fang
	chainValue := 5
	g := &LadderGame{
		handA: []game.Card{
			{Suit: game.Blade, Face: 2},
			{Suit: game.Blade, Face: 9},
			{Suit: game.Fang, Face: 4},
			{Suit: game.Shield, Face: 4},
			{Suit: game.Shield, Face: 9},
		},
		current: PlayerA, chainSuit: &chainSuit, chainValue: &chainValue,
		wildFaceRaw: 9, sharedStats: SharedStats{Shift: 1, Favor: -2},
	}
	legal := g.LegalPlays(PlayerA)
	want := []game.Card{
		{Suit: game.Blade, Face: 9},
		{Suit: game.Fang, Face: 4},
		{Suit: game.Shield, Face: 4},
		{Suit: game.Shield, Face: 9},
	}
	if !reflect.DeepEqual(legal, want) {
		t.Fatalf("numeric-chain legal plays = %v, want %v", legal, want)
	}

	g.chainIsWild = true
	g.chainValue = nil
	legal = g.LegalPlays(PlayerA)
	if !reflect.DeepEqual(legal, []game.Card{{Suit: game.Blade, Face: 9}}) {
		t.Fatalf("wild-chain legal plays = %v, want only the winning-suit wild", legal)
	}
	if got := g.LegalPlays(PlayerB); got == nil || len(got) != 0 {
		t.Fatalf("empty legal-play result = %#v, want a non-nil empty slice", got)
	}
}

func TestPlayResetsChainValueAfterDominantPlay(t *testing.T) {
	chainSuit := game.Fang
	chainValue := 4
	card := game.Card{Suit: game.Blade, Face: 1}
	g := &LadderGame{
		handA: []game.Card{card}, handB: []game.Card{{Suit: game.Shield, Face: 1}},
		current: PlayerA, chainSuit: &chainSuit, chainValue: &chainValue,
		sharedStats: SharedStats{Shift: 2, Favor: -1}, wildFaceRaw: 9,
	}
	if err := g.Play(PlayerA, card); err != nil {
		t.Fatal(err)
	}
	if got := g.Winner(); got == nil || *got != PlayerA {
		t.Fatalf("winner = %v, want PlayerA", got)
	}
	if state := g.State(); state.ChainCard == nil || *state.ChainCard != card ||
		state.ChainValue == nil || *state.ChainValue != 3 ||
		state.ChainSuit == nil || *state.ChainSuit != game.Blade {
		t.Fatalf("played card did not reset to its shifted face value: %+v", state)
	}
	if err := g.Play(PlayerB, game.Card{Suit: game.Shield, Face: 1}); !errors.Is(err, ErrGameOver) {
		t.Fatalf("play after game over error = %v, want ErrGameOver", err)
	}
}

func TestDominantSuitPlayResetsTargetForNextPlayer(t *testing.T) {
	chainSuit := game.Blade
	chainValue := 9
	rockFive := game.Card{Suit: game.Shield, Face: 5}
	paperNine := game.Card{Suit: game.Fang, Face: 9}
	paperFour := game.Card{Suit: game.Fang, Face: 4}
	g := &LadderGame{
		handA: []game.Card{rockFive, paperNine}, handB: []game.Card{paperFour},
		current: PlayerA, chainSuit: &chainSuit, chainValue: &chainValue,
		wildFaceRaw: 8,
	}
	if got, want := g.LegalPlays(PlayerA), []game.Card{rockFive, paperNine}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legal responses to Scissors 9 = %v, want %v", got, want)
	}
	if err := g.Play(PlayerA, rockFive); err != nil {
		t.Fatal(err)
	}
	state := g.State()
	if state.ChainValue == nil || *state.ChainValue != 5 || state.ChainSuit == nil ||
		*state.ChainSuit != game.Shield ||
		!reflect.DeepEqual(state.PlayedCards, []game.Card{rockFive}) {
		t.Fatalf("dominant play did not reset target to face value: %+v", state)
	}
	if got, want := g.LegalPlays(PlayerB), []game.Card{paperFour}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legal response to Rock 5 = %v, want %v", got, want)
	}
	if err := g.Play(PlayerB, paperFour); err != nil {
		t.Fatal(err)
	}
	if got, want := g.State().PlayedCards, []game.Card{rockFive, paperFour}; !reflect.DeepEqual(got, want) {
		t.Fatalf("played-card history = %v, want %v", got, want)
	}
}

func TestRestoreResetsLegacyChainTargetToShiftedFaceValue(t *testing.T) {
	chainSuit := game.Blade
	legacyChainValue := 18
	restored := Restore(LadderState{
		ChainValue: &legacyChainValue, ChainSuit: &chainSuit,
	})
	state := restored.State()
	if state.ChainCard == nil || *state.ChainCard != (game.Card{Suit: game.Blade, Face: 9}) ||
		state.ChainValue == nil || *state.ChainValue != 9 ||
		!reflect.DeepEqual(state.PlayedCards, []game.Card{{Suit: game.Blade, Face: 9}}) {
		t.Fatalf("restored chain = card %v, target %v; want Scissors 9 with target 9",
			state.ChainCard, state.ChainValue)
	}
}

func TestPlayRequiresLegalCurrentHandCard(t *testing.T) {
	chainSuit := game.Fang
	chainValue := 8
	illegal := game.Card{Suit: game.Fang, Face: 1}
	g := &LadderGame{
		handA: []game.Card{illegal}, handB: []game.Card{{Suit: game.Blade, Face: 2}},
		current: PlayerA, chainSuit: &chainSuit, chainValue: &chainValue,
	}
	if err := g.Play(PlayerA, illegal); !errors.Is(err, ErrIllegalPlay) {
		t.Fatalf("illegal play error = %v, want ErrIllegalPlay", err)
	}
	if err := g.Play(PlayerB, game.Card{Suit: game.Blade, Face: 2}); !errors.Is(err, ErrWrongTurn) {
		t.Fatalf("wrong player error = %v, want ErrWrongTurn", err)
	}
	if err := g.Play(PlayerA, game.Card{Suit: game.Shield, Face: 4}); !errors.Is(err, ErrCardNotInHand) {
		t.Fatalf("missing card error = %v, want ErrCardNotInHand", err)
	}
}

func TestPassingIsForcedOnlyWhenNoLegalCardAndOpponentLeadsFresh(t *testing.T) {
	chainSuit := game.Fang
	chainValue := 20
	g := &LadderGame{
		handA: []game.Card{{Suit: game.Fang, Face: 1}},
		handB: []game.Card{
			{Suit: game.Blade, Face: 2},
			{Suit: game.Shield, Face: 3},
			{Suit: game.Fang, Face: 9},
		},
		current: PlayerA, chainSuit: &chainSuit, chainValue: &chainValue,
		wildFaceRaw: 9, sharedStats: SharedStats{Shift: 1},
	}
	if err := g.PassTurn(PlayerA); err != nil {
		t.Fatal(err)
	}
	state := g.State()
	if !state.OpeningPending || state.OpeningCard != nil || state.Current != PlayerB ||
		!reflect.DeepEqual(state.HandB, []game.Card{
			{Suit: game.Blade, Face: 2},
			{Suit: game.Shield, Face: 3},
			{Suit: game.Fang, Face: 9},
		}) {
		t.Fatalf("pass did not give the opponent a choice to open a new ladder: %+v", state)
	}
	if err := g.OpenStack(PlayerB, game.Card{Suit: game.Blade, Face: 2}); err != nil {
		t.Fatal(err)
	}
	state = g.State()
	if state.Current != PlayerA || !reflect.DeepEqual(state.HandB, []game.Card{{Suit: game.Shield, Face: 3}, {Suit: game.Fang, Face: 9}}) ||
		state.ChainValue == nil || *state.ChainValue != 3 || state.ChainSuit == nil ||
		*state.ChainSuit != game.Blade || state.ChainIsWild ||
		!reflect.DeepEqual(state.PlayedCards, []game.Card{{Suit: game.Blade, Face: 2}}) {
		t.Fatalf("chosen opening card did not establish a fresh chain and return the turn: %+v", state)
	}

	g.chainValue = intPointer(1)
	if err := g.PassTurn(PlayerA); err != nil {
		t.Fatalf("voluntary pass should open a new chain for the opponent: %v", err)
	}
	if state := g.State(); !state.OpeningPending || state.Current != PlayerB {
		t.Fatalf("voluntary pass did not give the opponent the opening: %+v", state)
	}
}

func TestPassingAWildStartsFreshLadderAndResetsPlayedCards(t *testing.T) {
	wild := game.Card{Suit: game.Fang, Face: 8}
	opening := game.Card{Suit: game.Fang, Face: 1}
	chainSuit := wild.Suit
	g := &LadderGame{
		handA: []game.Card{opening, {Suit: game.Blade, Face: 2}},
		handB: []game.Card{{Suit: game.Shield, Face: 1}},
		playedCards: []game.Card{
			{Suit: game.Blade, Face: 7},
			wild,
		},
		chainCard: &wild, chainSuit: &chainSuit, chainIsWild: true,
		current: PlayerB, wildFaceRaw: wild.Face,
	}
	if legal := g.LegalPlays(PlayerB); len(legal) != 0 {
		t.Fatalf("non-wild response to wild was legal: %v", legal)
	}
	if err := g.PassTurn(PlayerB); err != nil {
		t.Fatal(err)
	}
	state := g.State()
	if !state.OpeningPending || state.OpeningCard != nil ||
		state.OpeningPlayer != PlayerA || state.Current != PlayerA ||
		!reflect.DeepEqual(state.HandA, []game.Card{opening, {Suit: game.Blade, Face: 2}}) ||
		!reflect.DeepEqual(state.HandB, []game.Card{{Suit: game.Shield, Face: 1}}) {
		t.Fatalf("passing wild did not give the opponent choice of the next ladder: %+v", state)
	}
	g = Restore(state)
	if err := g.OpenStack(PlayerA, opening); err != nil {
		t.Fatal(err)
	}
	state = g.State()
	if state.ChainCard == nil || *state.ChainCard != opening ||
		state.ChainIsWild || state.Current != PlayerB ||
		!reflect.DeepEqual(state.PlayedCards, []game.Card{opening}) ||
		!reflect.DeepEqual(state.HandA, []game.Card{{Suit: game.Blade, Face: 2}}) ||
		!reflect.DeepEqual(state.HandB, []game.Card{{Suit: game.Shield, Face: 1}}) {
		t.Fatalf("chosen opening card did not start a fresh ladder: %+v", state)
	}
}

func TestWildChainRequiresBetterSuitedWildAndPassCanLeadWild(t *testing.T) {
	chainSuit := game.Fang
	g := &LadderGame{
		handA: []game.Card{
			{Suit: game.Blade, Face: 9},
			{Suit: game.Fang, Face: 9},
			{Suit: game.Shield, Face: 9},
		},
		handB:   []game.Card{{Suit: game.Blade, Face: 1}},
		current: PlayerA, chainSuit: &chainSuit, chainIsWild: true, wildFaceRaw: 9,
	}
	if got := g.LegalPlays(PlayerA); !reflect.DeepEqual(got, []game.Card{{Suit: game.Blade, Face: 9}}) {
		t.Fatalf("legal wild responses = %v", got)
	}
	if err := g.Play(PlayerA, game.Card{Suit: game.Blade, Face: 9}); err != nil {
		t.Fatal(err)
	}
	if state := g.State(); !state.ChainIsWild || state.ChainValue != nil ||
		state.ChainSuit == nil || *state.ChainSuit != game.Blade {
		t.Fatalf("better wild did not establish a wild chain: %+v", state)
	}
}

func TestGreedyPolicySavesWildAndOrdersCards(t *testing.T) {
	g := &LadderGame{wildFaceRaw: 4}
	legal := []game.Card{
		{Suit: game.Shield, Face: 4},
		{Suit: game.Fang, Face: 3},
		{Suit: game.Blade, Face: 3},
	}
	want := game.Card{Suit: game.Blade, Face: 3}
	if got := GreedyPolicy().ChoosePlay(g, legal); got != want {
		t.Fatalf("greedy choice = %v, want %v", got, want)
	}
}

func intPointer(value int) *int {
	return &value
}
