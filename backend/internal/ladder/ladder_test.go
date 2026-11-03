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
	if !state.FallbackNoInitiativeSuit || state.Current != PlayerB || state.ChainValue == nil ||
		*state.ChainValue != 1 || state.ChainSuit == nil || *state.ChainSuit != game.Shield {
		t.Fatalf("unexpected fallback opening state: %+v", state)
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
		containsCard(state.HandB, game.Card{Suit: game.Shield, Face: 1}) ||
		containsCard(state.HandB, game.Card{Suit: game.Fang, Face: 1}) {
		t.Fatalf("draft did not exchange each player's lowest card: hands=%v/%v", state.HandA, state.HandB)
	}
	if state.InitiativeSuit != game.Blade || state.FallbackNoInitiativeSuit ||
		state.Current != PlayerA || state.ChainSuit == nil || *state.ChainSuit != game.Shield {
		t.Fatalf("leader was not selected from the drafted initiative-suit cards: %+v", state)
	}
}

func TestStateReturnsDeepCopies(t *testing.T) {
	value := 5
	suit := game.Blade
	winner := PlayerA
	g := &LadderGame{
		handA:      []game.Card{{Suit: game.Blade, Face: 1}},
		handB:      []game.Card{{Suit: game.Fang, Face: 2}},
		chainValue: &value, chainSuit: &suit, winner: &winner,
	}
	state := g.State()
	state.HandA[0].Face = 9
	*state.ChainValue = 1
	*state.ChainSuit = game.Shield
	*state.Winner = PlayerB
	if g.handA[0].Face != 1 || *g.chainValue != 5 || *g.chainSuit != game.Blade ||
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

func TestPlayUsesPreviousChainSuitAndEndsOnEmptyHand(t *testing.T) {
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
	if state := g.State(); state.ChainValue == nil || *state.ChainValue != 5 ||
		state.ChainSuit == nil || *state.ChainSuit != game.Blade {
		t.Fatalf("played card did not freeze its effective value: %+v", state)
	}
	if err := g.Play(PlayerB, game.Card{Suit: game.Shield, Face: 1}); !errors.Is(err, ErrGameOver) {
		t.Fatalf("play after game over error = %v, want ErrGameOver", err)
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
	if state.Current != PlayerA || !reflect.DeepEqual(state.HandB, []game.Card{{Suit: game.Shield, Face: 3}, {Suit: game.Fang, Face: 9}}) ||
		state.ChainValue == nil || *state.ChainValue != 3 || state.ChainSuit == nil ||
		*state.ChainSuit != game.Blade || state.ChainIsWild {
		t.Fatalf("pass did not establish a fresh chain and return the turn: %+v", state)
	}

	g.chainValue = intPointer(1)
	if err := g.PassTurn(PlayerA); !errors.Is(err, ErrPassNotAllowed) {
		t.Fatalf("voluntary pass error = %v, want ErrPassNotAllowed", err)
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
