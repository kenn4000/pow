package game

import (
	"math/rand"
	"testing"
)

func TestSuitBeatsCycle(t *testing.T) {
	tests := []struct {
		name  string
		suit  Suit
		other Suit
		want  bool
	}{
		{"Blade beats Fang", Blade, Fang, true},
		{"Fang beats Shield", Fang, Shield, true},
		{"Shield beats Blade", Shield, Blade, true},
		{"same suit", Blade, Blade, false},
		{"reverse cycle", Fang, Blade, false},
		{"invalid suit", Suit(3), Blade, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.suit.Beats(test.other); got != test.want {
				t.Fatalf("%s.Beats(%s) = %t, want %t", test.suit, test.other, got, test.want)
			}
		})
	}
}

func TestEffectiveValue(t *testing.T) {
	tests := []struct {
		name     string
		card     Card
		opponent Card
		want     int
	}{
		{"same suit does not double", Card{Blade, 7}, Card{Blade, 3}, 7},
		{"beating suit doubles", Card{Blade, 4}, Card{Fang, 6}, 8},
		{"losing suit does not double", Card{Fang, 6}, Card{Blade, 4}, 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EffectiveValue(test.card, test.opponent); got != test.want {
				t.Fatalf("EffectiveValue() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestResolveAttribution(t *testing.T) {
	tests := []struct {
		name     string
		attacker Card
		defender Card
		winner   int
		captured Card
		attValue int
		defValue int
	}{
		{"attacker wins", Card{Blade, 4}, Card{Fang, 6}, 0, Card{Fang, 6}, 8, 6},
		{"defender wins", Card{Fang, 6}, Card{Blade, 4}, 1, Card{Fang, 6}, 6, 8},
		{"clash", Card{Blade, 2}, Card{Fang, 4}, -1, Card{}, 4, 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Resolve(test.attacker, test.defender)
			if got.Winner != test.winner || got.Captured != test.captured ||
				got.AttackerValue != test.attValue || got.DefenderValue != test.defValue {
				t.Fatalf("Resolve() = %+v", got)
			}
		})
	}
}

func TestScoring(t *testing.T) {
	tests := []struct {
		name   string
		jail   []Card
		bounty Suit
		want   int
	}{
		{"empty", nil, Blade, 0},
		{"one suit count times maximum", []Card{{Blade, 3}, {Blade, 8}}, Fang, 16},
		{"multiple suits", []Card{{Blade, 3}, {Blade, 8}, {Fang, 4}, {Shield, 2}}, Blade, 24},
		{"bounty bonus for every matching card", []Card{{Fang, 2}, {Fang, 9}}, Fang, 20},
		{"worked example", []Card{{Blade, 7}, {Blade, 7}, {Fang, 9}, {Shield, 2}}, Fang, 26},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Score(test.jail, test.bounty).Total; got != test.want {
				t.Fatalf("Score() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestRefillDrawsAndDiscardsLowest(t *testing.T) {
	state, err := NewGame(rand.New(rand.NewSource(21)))
	if err != nil {
		t.Fatal(err)
	}
	playerIndex, err := state.PlayerAtSeat(SeatX)
	if err != nil {
		t.Fatal(err)
	}
	player, _ := state.Player(playerIndex)
	player.Hand = append(player.Hand, state.Deck[len(state.Deck)-1], state.Deck[len(state.Deck)-2])
	state.Deck = state.Deck[:len(state.Deck)-2]
	beforeDeck := len(state.Deck)
	discarded, err := Refill(&state, playerIndex)
	if err != nil {
		t.Fatal(err)
	}
	if len(player.Hand) != 6 || len(discarded) != 2 {
		t.Fatalf("hand/discard lengths = %d/%d, want 6/2", len(player.Hand), len(discarded))
	}
	if discarded[0].Face > discarded[1].Face {
		t.Fatalf("discarded cards are not in lowest-first order: %v", discarded)
	}

	removed := append([]Card(nil), player.Hand[4:]...)
	player.Hand = player.Hand[:4]
	state.Discard = append(state.Discard, removed...)
	if _, err := Refill(&state, playerIndex); err != nil {
		t.Fatal(err)
	}
	if len(player.Hand) != 6 || len(state.Deck) != beforeDeck-2 {
		t.Fatalf("refill did not draw to target: hand=%d deck=%d", len(player.Hand), len(state.Deck))
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSeatSwapHappensBeforeRefill(t *testing.T) {
	state, err := NewGame(rand.New(rand.NewSource(22)))
	if err != nil {
		t.Fatal(err)
	}
	oldX, _ := state.PlayerAtSeat(SeatX)
	oldY := 1 - oldX
	if err := EndRound(&state, true, true); err != nil {
		t.Fatal(err)
	}
	newX, _ := state.PlayerAtSeat(SeatX)
	newY := 1 - newX
	xPlayer, _ := state.Player(newX)
	yPlayer, _ := state.Player(newY)
	if newX != oldY || newY != oldX {
		t.Fatal("seats were not swapped")
	}
	if len(xPlayer.Hand) != 6 || len(yPlayer.Hand) != 5 {
		t.Fatalf("refill used old seat sizes: X=%d Y=%d", len(xPlayer.Hand), len(yPlayer.Hand))
	}
}

func TestCommitRequiresThreeDistinctCardsAndLocksPool(t *testing.T) {
	state, err := NewGame(rand.New(rand.NewSource(23)))
	if err != nil {
		t.Fatal(err)
	}
	initialCards := len(state.P1.Hand)
	if err := Commit(&state, 0, []int{0, 0, 1}); err == nil {
		t.Fatal("duplicate commit card was accepted")
	}
	if err := Commit(&state, 0, []int{0, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := Commit(&state, 0, []int{0, 1, 2}); err == nil {
		t.Fatal("second commit was accepted")
	}
	if len(state.P1.Hand)+len(state.P1.Pool) != initialCards {
		t.Fatal("commit changed the player's total cards")
	}
}

func TestFullGameMaintainsCardAccounting(t *testing.T) {
	rng := rand.New(rand.NewSource(24))
	state, err := NewGame(rng)
	if err != nil {
		t.Fatal(err)
	}
	for round := 1; round <= 3; round++ {
		state.Round = round
		if round == 3 {
			state.P1.Seat, state.P2.Seat = SeatX, SeatY
		}
		for player := 0; player < 2; player++ {
			if err := Commit(&state, player, []int{0, 1, 2}); err != nil {
				t.Fatal(err)
			}
			if err := state.Validate(); err != nil {
				t.Fatal(err)
			}
		}
		for exchange := 0; exchange < 3; exchange++ {
			attackerSeat := SeatX
			if exchange == 1 {
				attackerSeat = SeatY
			}
			attacker, _ := state.PlayerAtSeat(attackerSeat)
			if _, err := PlayExchange(&state, attacker, 0, 1-attacker, 0); err != nil {
				t.Fatal(err)
			}
			if err := state.Validate(); err != nil {
				t.Fatal(err)
			}
		}
		DiscardPools(&state)
		if round < 3 {
			if err := EndRound(&state, round == 1, true); err != nil {
				t.Fatal(err)
			}
		}
		if err := state.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
