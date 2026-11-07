package app

import (
	"math/rand"
	"testing"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

func TestNewAbandonDuelGameCombinesModifiersAndDealsMatchingPiles(t *testing.T) {
	first := heroes.Hero{Bounty: 4, Shift: 3, Favored: 2, Draft: 1}
	second := heroes.Hero{Bounty: 1, Shift: 3, Favored: -1, Draft: 1}
	duel, err := newAbandonDuelGame(first, second, rand.New(rand.NewSource(21)))
	if err != nil {
		t.Fatal(err)
	}
	if duel.Modifiers != (abandonDuelModifiers{B: 3, S: 4, F: 1, D: 1}) {
		t.Fatalf("combined modifiers = %+v", duel.Modifiers)
	}
	if len(duel.Board) != abandonDuelBoardSize ||
		len(duel.Players[0].PersonalPile) != 5 || len(duel.Players[1].PersonalPile) != 5 {
		t.Fatalf("board/piles = %d/%d/%d", len(duel.Board), len(duel.Players[0].PersonalPile), len(duel.Players[1].PersonalPile))
	}
	if err := duel.validateState(); err != nil {
		t.Fatal(err)
	}
	foundSuits := [3]bool{}
	for index, card := range duel.Players[0].PersonalPile {
		foundSuits[int(card.Suit)] = true
		if duel.Players[1].PersonalPile[index] != card {
			t.Fatalf("private piles differ at index %d", index)
		}
		if duel.Players[0].PileRevealed[index] != (index < 4) {
			t.Fatalf("private card %d revealed = %v, want %v", index, duel.Players[0].PileRevealed[index], index < 4)
		}
	}
	if !foundSuits[0] || !foundSuits[1] || !foundSuits[2] {
		t.Fatalf("private pile does not contain every suit: %v", foundSuits)
	}
	highestBounty := -1
	for index, card := range duel.Board {
		if card.Suit == duel.Bounty.Suit && (highestBounty < 0 || card.Face > duel.Board[highestBounty].Face) {
			highestBounty = index
		}
	}
	if highestBounty < 0 || duel.BoardRevealed[highestBounty] {
		t.Fatalf("highest bounty card index %d should be hidden", highestBounty)
	}
	for index, card := range duel.Board {
		if card.Suit == duel.Bounty.Suit && index != highestBounty && !duel.BoardRevealed[index] {
			t.Fatalf("non-highest bounty card %d is hidden", index)
		}
	}
}

func TestAbandonDuelSnakeOrderAndSwappedSeats(t *testing.T) {
	firstRound := []int{0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0}
	secondRound := []int{1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1}
	for turn := 0; turn < abandonDuelTurnCount; turn++ {
		if got := abandonDuelSnakeOwner(turn, 0); got != firstRound[turn] {
			t.Errorf("round 1 turn %d owner = %d, want %d", turn, got, firstRound[turn])
		}
		if got := abandonDuelSnakeOwner(turn, 1); got != secondRound[turn] {
			t.Errorf("round 2 turn %d owner = %d, want %d", turn, got, secondRound[turn])
		}
	}
}

func TestAbandonDuelResolveUsesDominanceAndFavor(t *testing.T) {
	captured, discarded := abandonDuelResolve(
		game.Card{Suit: game.Blade, Face: 3}, game.Card{Suit: game.Blade, Face: 5}, 0, 0,
	)
	if captured == nil || *captured != (game.Card{Suit: game.Blade, Face: 3}) ||
		discarded == nil || *discarded != (game.Card{Suit: game.Blade, Face: 5}) {
		t.Fatalf("same-suit resolve = %v, %v", captured, discarded)
	}
	captured, discarded = abandonDuelResolve(
		game.Card{Suit: game.Blade, Face: 4}, game.Card{Suit: game.Blade, Face: 4}, 0, 0,
	)
	if captured != nil || discarded != nil {
		t.Fatalf("equal same-suit cards should tie, got %v, %v", captured, discarded)
	}
	captured, discarded = abandonDuelResolve(
		game.Card{Suit: game.Blade, Face: 5}, game.Card{Suit: game.Fang, Face: 9}, 1, 0,
	)
	if captured == nil || *captured != (game.Card{Suit: game.Fang, Face: 9}) ||
		discarded == nil || *discarded != (game.Card{Suit: game.Blade, Face: 5}) {
		t.Fatalf("dominant resolve = %v, %v", captured, discarded)
	}
	captured, discarded = abandonDuelResolve(
		game.Card{Suit: game.Blade, Face: 4}, game.Card{Suit: game.Fang, Face: 6}, 0, -2,
	)
	if captured != nil || discarded != nil {
		t.Fatalf("favor-adjusted equal values should tie, got %v, %v", captured, discarded)
	}
}

func TestAbandonDuelScoreUsesHighCardPerSuitAndAbandonPenalty(t *testing.T) {
	prisoners := []game.Card{
		{Suit: game.Blade, Face: 3},
		{Suit: game.Blade, Face: 7},
		{Suit: game.Fang, Face: 4},
	}
	personal := []game.Card{
		{Suit: game.Blade, Face: 1},
		{Suit: game.Shield, Face: 5},
	}
	score := abandonDuelScore(prisoners, personal, []bool{false, false}, game.Blade, abandonDuelModifiers{B: 1, S: 2})
	if score != 16 {
		t.Fatalf("score = %d, want 16", score)
	}
	if got := abandonDuelJailValue(game.Card{Suit: game.Blade, Face: 3}, game.Blade, 1, 2); got != 7 {
		t.Fatalf("jail value = %d, want 7", got)
	}
}

func TestAbandonDuelCPUUniformlyMaterializesChosenHiddenSlot(t *testing.T) {
	known := game.Card{Suit: game.Blade, Face: 9}
	hidden := []game.Card{
		{Suit: game.Fang, Face: 1},
		{Suit: game.Fang, Face: 2},
	}
	hero := abandonDuelHeroParams{}
	for selectedIndex := range hidden {
		move := chooseAbandonDuelCPUMove(
			[]game.Card{known}, nil, hidden, nil, hero, game.Shield, true,
			func(n int) int {
				if n != len(hidden) {
					t.Fatalf("hidden choice count = %d, want %d", n, len(hidden))
				}
				return selectedIndex
			},
		)
		if move.first != known || move.second != hidden[selectedIndex] {
			t.Fatalf("hidden index %d materialized move %+v", selectedIndex, move)
		}
	}
}

func TestAbandonDuelCPUSelectsTwoDistinctHiddenCards(t *testing.T) {
	hidden := []game.Card{
		{Suit: game.Blade, Face: 3},
		{Suit: game.Fang, Face: 4},
	}
	calls := 0
	move := chooseAbandonDuelCPUMove(
		nil, nil, nil, hidden, abandonDuelHeroParams{}, game.Shield, true,
		func(n int) int {
			calls++
			if calls == 1 {
				return 1
			}
			return 0
		},
	)
	if move.first == move.second || !containsAbandonDuelCard(hidden, move.first) ||
		!containsAbandonDuelCard(hidden, move.second) || calls != 2 {
		t.Fatalf("two-hidden move = %+v, random calls = %d", move, calls)
	}
}

func TestOnlineAbandonDuelViewUsesViewerRelativePlayers(t *testing.T) {
	view := &webAbandonDuelState{
		ActivePlayer: 1, StartingPlayer: 0, Winner: 1,
		Players:     [2]webAbandonDuelPlayer{{HeroID: "first"}, {HeroID: "second"}},
		TotalScores: [2]int{12, 24},
		RoundScores: [2][2]int{{5, 7}, {7, 17}},
		RoundPairCounts: [2][2]int{
			{6, 6}, {6, 6},
		},
		Pairings: []abandonDuelPairing{{Player: 0}, {Player: 1}},
	}

	orientAbandonDuelView(view, true)

	if view.ActivePlayer != 0 || view.StartingPlayer != 1 || view.Winner != 0 {
		t.Fatalf("viewer-relative player indexes = active %d, starter %d, winner %d",
			view.ActivePlayer, view.StartingPlayer, view.Winner)
	}
	if view.Players[0].HeroID != "second" || view.Players[1].HeroID != "first" {
		t.Fatalf("viewer-relative players = %q, %q", view.Players[0].HeroID, view.Players[1].HeroID)
	}
	if view.TotalScores != [2]int{24, 12} ||
		view.RoundScores != [2][2]int{{7, 5}, {17, 7}} ||
		view.RoundPairCounts != [2][2]int{{6, 6}, {6, 6}} {
		t.Fatalf("viewer-relative scores = total %v, rounds %v, pairs %v",
			view.TotalScores, view.RoundScores, view.RoundPairCounts)
	}
	if view.Pairings[0].Player != 1 || view.Pairings[1].Player != 0 {
		t.Fatalf("viewer-relative pairing owners = %d, %d", view.Pairings[0].Player, view.Pairings[1].Player)
	}
}

func TestAbandonDuelCompletesBothSixPairRoundsAndSwapsStarter(t *testing.T) {
	hero := heroes.Hero{}
	rng := rand.New(rand.NewSource(8))
	duel, err := newAbandonDuelGame(hero, hero, rng)
	if err != nil {
		t.Fatal(err)
	}
	for !duel.GameOver {
		round := duel.Round
		if duel.phase() == "abandon-duel-reveal" {
			hidden := duel.hiddenBoardIndices()
			if len(hidden) == 0 {
				t.Fatal("reveal phase has no hidden board cards")
			}
			if err := duel.flipBoardCard(hidden[0]); err != nil {
				t.Fatal(err)
			}
		}
		player := duel.ActivePlayer
		pileIndex := -1
		for index, used := range duel.Players[player].PileUsed {
			if !used {
				pileIndex = index
				break
			}
		}
		boardIndices := make([]int, 0, len(duel.Board))
		for index, used := range duel.BoardUsed {
			if !used {
				boardIndices = append(boardIndices, index)
			}
		}
		if pileIndex >= 0 {
			if len(boardIndices) == 0 {
				t.Fatal("board ran out before each player used their five private cards")
			}
			if _, err := duel.selectPairCard(abandonPileSource, pileIndex, rng); err != nil {
				t.Fatal(err)
			}
			if _, err := duel.selectPairCard(abandonBoardSource, boardIndices[0], rng); err != nil {
				t.Fatal(err)
			}
		} else {
			if len(boardIndices) < 2 {
				t.Fatalf("player %d has fewer than two legal cards", player)
			}
			if _, err := duel.selectPairCard(abandonBoardSource, boardIndices[0], rng); err != nil {
				t.Fatal(err)
			}
			if _, err := duel.selectPairCard(abandonBoardSource, boardIndices[1], rng); err != nil {
				t.Fatal(err)
			}
		}
		if err := duel.validateState(); err != nil {
			t.Fatal(err)
		}
		if round == 1 && duel.Round == 2 {
			for player, state := range duel.Players {
				if len(state.Prisoners) != 0 {
					t.Fatalf("player %d carried %d prisoners into round 2",
						player, len(state.Prisoners))
				}
			}
		}
	}
	if duel.RoundPairCounts != [2][2]int{{6, 6}, {6, 6}} {
		t.Fatalf("pair counts by round = %v", duel.RoundPairCounts)
	}
	if duel.Winner < -1 || duel.Winner > 1 {
		t.Fatalf("invalid winner %d", duel.Winner)
	}
	for player := range duel.TotalScores {
		if duel.TotalScores[player] != duel.RoundScores[0][player]+duel.RoundScores[1][player] {
			t.Fatalf("player %d total score does not equal both rounds", player)
		}
	}
}
