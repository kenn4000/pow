package main

import (
	"bufio"
	"math/rand"
	"strings"
	"testing"

	"prisonersofwar/game"
)

func TestScriptedTerminalGamePreservesAllCards(t *testing.T) {
	var input strings.Builder
	for round := 1; round <= 3; round++ {
		if round == 3 {
			input.WriteString("X\n")
		}
		for player := 0; player < 2; player++ {
			input.WriteString("\n1 2 3\ny\n")
		}
		for exchange := 0; exchange < 3; exchange++ {
			input.WriteString("1\n1\n")
		}
	}
	gameApp := app{
		scanner: bufio.NewScanner(strings.NewReader(input.String())),
		rng:     rand.New(rand.NewSource(41)),
		mode:    humanVsHuman,
	}
	state, err := gameApp.playGame()
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Validate(); err != nil {
		t.Fatalf("final game state is invalid: %v", err)
	}
	if prisoners := len(state.P1.Jail) + len(state.P2.Jail); prisoners > 9 {
		t.Fatalf("captured %d prisoners, maximum is 9", prisoners)
	}
	if len(state.P1.Pool)+len(state.P2.Pool) != 0 {
		t.Fatal("cards remained in pools after the final round")
	}
	finalCards := allCards(state)
	if len(finalCards) != 27 {
		t.Fatalf("final state accounts for %d cards, want 27", len(finalCards))
	}
	for _, card := range game.NewDeck() {
		if !containsCard(finalCards, card) {
			t.Fatalf("final state lost card %s", card)
		}
	}
}

func allCards(state game.GameState) []game.Card {
	cards := []game.Card{state.Bounty}
	cards = append(cards, state.Deck...)
	cards = append(cards, state.Discard...)
	for _, player := range state.Players() {
		cards = append(cards, player.Hand...)
		cards = append(cards, player.Pool...)
		cards = append(cards, player.Jail...)
	}
	return cards
}

func containsCard(cards []game.Card, target game.Card) bool {
	for _, card := range cards {
		if card == target {
			return true
		}
	}
	return false
}
