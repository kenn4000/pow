package game

import (
	"errors"
	"sort"
)

type Outcome struct {
	AttackerValue int
	DefenderValue int
	Winner        int // 0 attacker, 1 defender, -1 clash
	Captured      Card
}

func Resolve(attacker, defender Card) Outcome {
	result := Outcome{
		AttackerValue: EffectiveValue(attacker, defender),
		DefenderValue: EffectiveValue(defender, attacker),
		Winner:        -1,
	}
	if result.AttackerValue > result.DefenderValue {
		result.Winner, result.Captured = 0, defender
	} else if result.DefenderValue > result.AttackerValue {
		result.Winner, result.Captured = 1, attacker
	}
	return result
}

type ScoreBreakdown struct {
	BySuit        [3]int
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
			result.BySuit[suit] = counts[suit] * highest[suit]
			result.Total += result.BySuit[suit]
		}
	}
	result.Total += result.BountyBonus
	return result
}

func ScoreDelta(jail []Card, card Card, bounty Suit) int {
	before := Score(jail, bounty).Total
	after := append(append([]Card(nil), jail...), card)
	return Score(after, bounty).Total - before
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
	outcome := Resolve(attackCard, defendCard)
	switch outcome.Winner {
	case 0:
		attacker.Jail = append(attacker.Jail, defendCard)
		state.Discard = append(state.Discard, attackCard)
	case 1:
		defender.Jail = append(defender.Jail, attackCard)
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
