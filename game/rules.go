package game

import (
	"errors"
	"fmt"
	"sort"

	"prisonersofwar/modifiers"
)

type Outcome struct {
	AttackerValue int
	DefenderValue int
	Winner        int // 0 attacker, 1 defender, -1 clash
	Captured      Card
}

func Resolve(attacker, defender Card) Outcome {
	return ResolveWithModifiers(attacker, defender, GameModifiers{Reckoning: modifiers.DefaultReckoning})
}

func ResolveWithModifiers(attacker, defender Card, gameModifiers GameModifiers) Outcome {
	result := Outcome{
		AttackerValue: EffectiveValueWithModifiers(attacker, defender, gameModifiers),
		DefenderValue: EffectiveValueWithModifiers(defender, attacker, gameModifiers),
		Winner:        -1,
	}
	if result.AttackerValue > result.DefenderValue {
		result.Winner, result.Captured = 0, defender
	} else if result.DefenderValue > result.AttackerValue {
		result.Winner, result.Captured = 1, attacker
	}
	return result
}

func EffectiveValueWithModifiers(card, opponent Card, gameModifiers GameModifiers) int {
	return modifiers.CombatValue(card.Face, card.Suit.Beats(opponent.Suit), gameModifiers.Shift, gameModifiers.Favored)
}

type ScoreBreakdown struct {
	BySuit        [3]int
	WeightedCount [3]int
	BountyBonus   int
	Total         int
	DistinctSuits int
	HighestCard   int
}

func Score(jail []Card, bounty Suit) ScoreBreakdown {
	var result ScoreBreakdown
	var counts, highest [3]int
	for _, card := range jail {
		suit := int(card.Suit)
		if suit < 0 || suit >= len(counts) || card.Face < 1 || card.Face > 9 {
			continue
		}
		counts[suit]++
		if card.Face > highest[suit] {
			highest[suit] = card.Face
		}
		if card.Face > result.HighestCard {
			result.HighestCard = card.Face
		}
		if card.Suit == bounty {
			result.BountyBonus++
		}
	}
	for suit := range counts {
		if counts[suit] > 0 {
			result.DistinctSuits++
			result.WeightedCount[suit] = counts[suit]
			result.BySuit[suit] = counts[suit] * highest[suit]
			result.Total += result.BySuit[suit]
		}
	}
	result.Total += result.BountyBonus
	return result
}

func ScoreWithModifiers(jail []Card, capturedByRound [3][3]int, bounty Suit, gameModifiers GameModifiers) ScoreBreakdown {
	var result ScoreBreakdown
	if gameModifiers.Reckoning == [3]int{} {
		// Zero-value modifiers in older summaries mean the original even scoring.
		gameModifiers.Reckoning = modifiers.DefaultReckoning
	}
	roundCounts := capturedByRound
	if total := totalCaptureCount(roundCounts); total == 0 && len(jail) > 0 {
		// Legacy match summaries did not record capture rounds; treat them as round one.
		for _, card := range jail {
			if card.Valid() {
				roundCounts[0][int(card.Suit)]++
			}
		}
	}

	var weightsBySuit, highestBySuit, baseHighestBySuit [3]int
	for round, counts := range roundCounts {
		for suit, count := range counts {
			if count <= 0 {
				continue
			}
			weightsBySuit[suit] += count * gameModifiers.Reckoning[round]
		}
	}
	for _, card := range jail {
		if !card.Valid() {
			continue
		}
		suit := int(card.Suit)
		face := modifiers.ScoringFace(card.Face, card.Suit == bounty, gameModifiers)
		baseFace := modifiers.ScoringFace(card.Face, false, GameModifiers{Shift: gameModifiers.Shift})
		if face > highestBySuit[suit] {
			highestBySuit[suit] = face
		}
		if baseFace > baseHighestBySuit[suit] {
			baseHighestBySuit[suit] = baseFace
		}
		if face > result.HighestCard {
			result.HighestCard = face
		}
	}
	for suit, weightedCount := range weightsBySuit {
		if weightedCount == 0 {
			continue
		}
		result.DistinctSuits++
		result.WeightedCount[suit] = weightedCount
		result.BySuit[suit] = weightedCount * highestBySuit[suit]
		baseScore := weightedCount * baseHighestBySuit[suit]
		result.Total += result.BySuit[suit]
		result.BountyBonus += result.BySuit[suit] - baseScore
	}
	return result
}

func ScorePlayer(player Player, bounty Suit, gameModifiers GameModifiers) ScoreBreakdown {
	return ScoreWithModifiers(player.Jail, player.CapturedByRound, bounty, gameModifiers)
}

func totalCaptureCount(captures [3][3]int) int {
	total := 0
	for _, round := range captures {
		for _, count := range round {
			total += count
		}
	}
	return total
}

func ScoreDelta(jail []Card, card Card, bounty Suit) int {
	before := Score(jail, bounty).Total
	after := append(append([]Card(nil), jail...), card)
	return Score(after, bounty).Total - before
}

func ScoreDeltaWithModifiers(player Player, card Card, bounty Suit, gameModifiers GameModifiers, round int) (int, error) {
	if round < 1 || round > len(gameModifiers.Reckoning) {
		return 0, fmt.Errorf("invalid capture round %d", round)
	}
	if !card.Valid() {
		return 0, fmt.Errorf("invalid captured card %v", card)
	}
	before := ScoreWithModifiers(player.Jail, player.CapturedByRound, bounty, gameModifiers).Total
	jail := append(append([]Card(nil), player.Jail...), card)
	captures := player.CapturedByRound
	captures[round-1][int(card.Suit)]++
	after := ScoreWithModifiers(jail, captures, bounty, gameModifiers).Total
	return after - before, nil
}

func Refill(state *GameState, playerIndex int) ([]Card, error) {
	player, err := state.Player(playerIndex)
	if err != nil {
		return nil, err
	}
	target := targetForSeat(player.Seat)
	discarded := make([]Card, 0)
	if len(player.Hand) > target {
		sort.SliceStable(player.Hand, func(i, j int) bool {
			if player.Hand[i].Face != player.Hand[j].Face {
				return player.Hand[i].Face < player.Hand[j].Face
			}
			return player.Hand[i].Suit < player.Hand[j].Suit
		})
		excess := len(player.Hand) - target
		discarded = append(discarded, player.Hand[:excess]...)
		state.Discard = append(state.Discard, discarded...)
		player.Hand = append([]Card(nil), player.Hand[excess:]...)
	}
	draw(state, &player.Hand, target)
	return discarded, nil
}

// EndRound swaps before refilling to keep seat-based hand sizes consistent.
func EndRound(state *GameState, swapSeats, refill bool) error {
	DiscardPools(state)
	if swapSeats {
		state.P1.Seat, state.P2.Seat = state.P2.Seat, state.P1.Seat
	}
	if refill {
		if _, err := Refill(state, 0); err != nil {
			return err
		}
		if _, err := Refill(state, 1); err != nil {
			return err
		}
	}
	if err := state.Validate(); err != nil {
		return err
	}
	return nil
}

func Commit(state *GameState, playerIndex int, handIndices []int) error {
	player, err := state.Player(playerIndex)
	if err != nil {
		return err
	}
	if len(player.Pool) > 0 {
		return ErrInvalidCommit
	}
	required := 3
	if len(player.Hand) < required {
		required = len(player.Hand)
	}
	if len(handIndices) != required {
		return ErrInvalidCommit
	}
	seen := make(map[int]bool, len(handIndices))
	for _, index := range handIndices {
		if index < 0 || index >= len(player.Hand) || seen[index] {
			return ErrInvalidCommit
		}
		seen[index] = true
	}
	indices := append([]int(nil), handIndices...)
	sort.Sort(sort.Reverse(sort.IntSlice(indices)))
	for _, index := range indices {
		player.Pool = append(player.Pool, player.Hand[index])
		player.Hand = append(player.Hand[:index], player.Hand[index+1:]...)
	}
	return nil
}

func PlayExchange(state *GameState, attackerIndex, attackPoolIndex, defenderIndex, defendPoolIndex int) (Outcome, error) {
	if state.Round < 1 || state.Round > 3 {
		return Outcome{}, fmt.Errorf("invalid capture round %d", state.Round)
	}
	if attackerIndex == defenderIndex || (attackerIndex != 0 && attackerIndex != 1) ||
		(defenderIndex != 0 && defenderIndex != 1) {
		return Outcome{}, errors.New("invalid attacker or defender")
	}
	attacker, _ := state.Player(attackerIndex)
	defender, _ := state.Player(defenderIndex)
	if attackPoolIndex < 0 || attackPoolIndex >= len(attacker.Pool) ||
		defendPoolIndex < 0 || defendPoolIndex >= len(defender.Pool) {
		return Outcome{}, ErrInvalidPoolCard
	}
	attackCard := take(&attacker.Pool, attackPoolIndex)
	defendCard := take(&defender.Pool, defendPoolIndex)
	outcome := ResolveWithModifiers(attackCard, defendCard, state.Modifiers)
	switch outcome.Winner {
	case 0:
		attacker.Jail = append(attacker.Jail, defendCard)
		attacker.CapturedByRound[state.Round-1][int(defendCard.Suit)]++
		state.Discard = append(state.Discard, attackCard)
	case 1:
		defender.Jail = append(defender.Jail, attackCard)
		defender.CapturedByRound[state.Round-1][int(attackCard.Suit)]++
		state.Discard = append(state.Discard, defendCard)
	default:
		state.Discard = append(state.Discard, attackCard, defendCard)
		state.Clashes++
	}
	return outcome, state.Validate()
}

func DiscardPools(state *GameState) {
	state.Discard = append(state.Discard, state.P1.Pool...)
	state.Discard = append(state.Discard, state.P2.Pool...)
	state.P1.Pool, state.P2.Pool = nil, nil
}

func draw(state *GameState, hand *[]Card, target int) {
	for len(*hand) < target && len(state.Deck) > 0 {
		last := len(state.Deck) - 1
		*hand = append(*hand, state.Deck[last])
		state.Deck = state.Deck[:last]
	}
}

func take(cards *[]Card, index int) Card {
	card := (*cards)[index]
	*cards = append((*cards)[:index], (*cards)[index+1:]...)
	return card
}
