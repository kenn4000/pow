package ai

import (
	"math/rand"
	"testing"

	"prisonersofwar/internal/game"
)

func TestHardAIChoicesAreLegal(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	state, err := game.NewGame(rng)
	if err != nil {
		t.Fatal(err)
	}
	indices, err := ChooseCommit(state, 0, Hard, rng)
	if err != nil {
		t.Fatal(err)
	}
	if len(indices) != 3 {
		t.Fatalf("hard AI chose %d commit cards, want 3", len(indices))
	}
	if err := game.Commit(&state, 0, indices); err != nil {
		t.Fatal(err)
	}
	opponentIndices, err := ChooseCommit(state, 1, Hard, rng)
	if err != nil {
		t.Fatal(err)
	}
	if err := game.Commit(&state, 1, opponentIndices); err != nil {
		t.Fatal(err)
	}
	attackIndex, err := ChoosePoolIndex(state, 0, true, nil, Hard, rng)
	if err != nil {
		t.Fatal(err)
	}
	attackCard := state.P1.Pool[attackIndex]
	defendIndex, err := ChoosePoolIndex(state, 1, false, &attackCard, Hard, rng)
	if err != nil {
		t.Fatal(err)
	}
	if defendIndex < 0 || defendIndex >= len(state.P2.Pool) {
		t.Fatalf("hard AI chose invalid defense index %d", defendIndex)
	}
}

func TestAIDraftPickUsesSuitBountyAndHandSynergy(t *testing.T) {
	pool := []game.Card{
		{Suit: game.Blade, Face: 7},
		{Suit: game.Fang, Face: 6},
		{Suit: game.Shield, Face: 9},
	}
	hand := []game.Card{{Suit: game.Blade, Face: 3}, {Suit: game.Blade, Face: 5}}
	got, err := AIDraftPick(pool, hand, game.Blade, Hard, rand.New(rand.NewSource(4)))
	if err != nil {
		t.Fatal(err)
	}
	if got != pool[0] {
		t.Fatalf("greedy draft picked %v, want bounty/synergy card %v", got, pool[0])
	}
}

func TestAIDraftPickMediumUsesFaceThenFewestSuitCards(t *testing.T) {
	pool := []game.Card{
		{Suit: game.Blade, Face: 9},
		{Suit: game.Fang, Face: 9},
		{Suit: game.Shield, Face: 8},
	}
	hand := []game.Card{{Suit: game.Blade, Face: 2}, {Suit: game.Blade, Face: 3}}
	got, err := AIDraftPickMedium(pool, hand)
	if err != nil {
		t.Fatal(err)
	}
	if got != pool[1] {
		t.Fatalf("medium draft pick = %v, want %v (highest face and fewer cards of that suit)", got, pool[1])
	}
	if _, err := AIDraftPickMedium(nil, hand); err == nil {
		t.Fatal("empty draft pool should fail")
	}
}

func TestHardVersusHardBalanceAndCardAccounting(t *testing.T) {
	if testing.Short() {
		t.Skip("10,000-game balance simulation")
	}
	rng := rand.New(rand.NewSource(32))
	const games = 10000
	var scores [2]int
	prisoners := 0
	for i := 0; i < games; i++ {
		state, err := game.NewGame(rng)
		if err != nil {
			t.Fatal(err)
		}
		for state.Round <= 3 {
			roundPlayed := state.Round
			if roundPlayed == 3 {
				s1 := game.Score(state.P1.Jail, state.Bounty.Suit).Total
				s2 := game.Score(state.P2.Jail, state.Bounty.Suit).Total
				chooser := 0
				if s2 < s1 || (s1 == s2 && rng.Intn(2) == 1) {
					chooser = 1
				}
				seat, err := ChooseSeat(state, chooser, Hard, rng)
				if err != nil {
					t.Fatal(err)
				}
				player, _ := state.Player(chooser)
				opponent, _ := state.Player(1 - chooser)
				player.Seat, opponent.Seat = seat, otherSeat(seat)
			}
			for playerIndex := 0; playerIndex < 2; playerIndex++ {
				indices, err := ChooseCommit(state, playerIndex, Hard, rng)
				if err != nil {
					t.Fatal(err)
				}
				if err := game.Commit(&state, playerIndex, indices); err != nil {
					t.Fatal(err)
				}
			}
			for exchange := 0; exchange < 3; exchange++ {
				seat := game.SeatX
				if exchange == 1 {
					seat = game.SeatY
				}
				attacker, _ := state.PlayerAtSeat(seat)
				defender := 1 - attacker
				attackPlayer, _ := state.Player(attacker)
				defendPlayer, _ := state.Player(defender)
				if len(attackPlayer.Pool) == 0 || len(defendPlayer.Pool) == 0 {
					continue
				}
				attackIndex, err := ChoosePoolIndex(state, attacker, true, nil, Hard, rng)
				if err != nil {
					t.Fatal(err)
				}
				attack := attackPlayer.Pool[attackIndex]
				defendIndex, err := ChoosePoolIndex(state, defender, false, &attack, Hard, rng)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := game.PlayExchange(&state, attacker, attackIndex, defender, defendIndex); err != nil {
					t.Fatal(err)
				}
			}
			game.DiscardPools(&state)
			if roundPlayed < 3 {
				if err := game.EndRound(&state, roundPlayed == 1, true); err != nil {
					t.Fatal(err)
				}
				state.Round++
			}
			if err := state.Validate(); err != nil {
				t.Fatal(err)
			}
			if roundPlayed == 3 {
				break
			}
		}
		scores[0] += game.Score(state.P1.Jail, state.Bounty.Suit).Total
		scores[1] += game.Score(state.P2.Jail, state.Bounty.Suit).Total
		gamePrisoners := len(state.P1.Jail) + len(state.P2.Jail)
		if gamePrisoners > 9 {
			t.Fatalf("game %d captured %d prisoners, maximum is 9", i, gamePrisoners)
		}
		prisoners += gamePrisoners
	}
	meanPrisoners := float64(prisoners) / games
	if meanPrisoners < 8 || meanPrisoners > 9 {
		t.Errorf("mean prisoners/game = %.3f, want 8.0–9.0", meanPrisoners)
	}
	mean1, mean2 := float64(scores[0])/games, float64(scores[1])/games
	mean := (mean1 + mean2) / 2
	if diff := mean1 - mean2; diff < -mean*0.05 || diff > mean*0.05 {
		t.Errorf("mean scores are not seat-balanced: P1 %.3f, P2 %.3f", mean1, mean2)
	}
}

func otherSeat(seat game.Seat) game.Seat {
	if seat == game.SeatX {
		return game.SeatY
	}
	return game.SeatX
}
