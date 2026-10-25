package ai

import (
	"errors"
	"math/rand"

	"prisonersofwar/internal/game"
)

type Difficulty int

const (
	Easy Difficulty = iota
	Normal
	Hard
	Greedy
)

const sampleCount = 8

func AIDraftPick(pool, aiHand []game.Card, bountySuit game.Suit, difficulty Difficulty, rng *rand.Rand) (game.Card, error) {
	if len(pool) == 0 {
		return game.Card{}, errors.New("draft pool is empty")
	}
	if rng == nil {
		return game.Card{}, errors.New("random source is required")
	}
	if difficulty == Easy || difficulty == Normal && rng.Intn(2) == 1 {
		return pool[rng.Intn(len(pool))], nil
	}
	bestIndex, bestScore := 0, -1
	for index, card := range pool {
		suitCount, highestFace := 0, 0
		for _, held := range aiHand {
			if held.Suit == card.Suit {
				suitCount++
				if held.Face > highestFace {
					highestFace = held.Face
				}
			}
		}
		score := int(card.Face) + suitCount*2
		if card.Suit == bountySuit {
			score += 2
		}
		if int(card.Face) > highestFace {
			score += 3
		}
		if score > bestScore {
			bestIndex, bestScore = index, score
		}
	}
	return pool[bestIndex], nil
}

func AIDraftPickMedium(pool, aiHand []game.Card) (game.Card, error) {
	if len(pool) == 0 {
		return game.Card{}, errors.New("draft pool is empty")
	}
	suitCounts := [3]int{}
	for _, card := range aiHand {
		if card.Suit.Valid() {
			suitCounts[card.Suit]++
		}
	}
	best := pool[0]
	for _, card := range pool[1:] {
		if card.Face > best.Face ||
			card.Face == best.Face && suitCounts[card.Suit] < suitCounts[best.Suit] {
			best = card
		}
	}
	return best, nil
}

func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "Easy"
	case Normal:
		return "Normal"
	case Hard:
		return "Hard"
	case Greedy:
		return "Greedy"
	default:
		return "Unknown"
	}
}

func ChooseCommit(state game.GameState, playerIndex int, difficulty Difficulty, rng *rand.Rand) ([]int, error) {
	player, err := state.Player(playerIndex)
	if err != nil {
		return nil, err
	}
	count := len(player.Hand)
	if count > 3 {
		count = 3
	}
	if count == 0 {
		return nil, nil
	}
	combos := combinations(len(player.Hand), count)
	if difficulty == Easy {
		return combos[rng.Intn(len(combos))], nil
	}
	if difficulty == Normal || difficulty == Greedy {
		best, bestValue := combos[0], -1<<30
		for _, combo := range combos {
			value := 0
			for _, index := range combo {
				delta, err := game.ScoreDeltaWithModifiers(*player, player.Hand[index], state.Bounty.Suit, state.Modifiers, state.Round)
				if err != nil {
					return nil, err
				}
				value += delta
			}
			if value > bestValue {
				best, bestValue = combo, value
			}
		}
		return best, nil
	}

	unseen := unseenCards(state, playerIndex)
	best, bestValue := combos[0], -1<<30
	for _, combo := range combos {
		ownPool := cardsAt(player.Hand, combo)
		value := 0
		for sample := 0; sample < sampleCount; sample++ {
			oppPool := sampleCards(unseen, 3, rng)
			roundValue, err := evaluateRound(state, playerIndex, ownPool, oppPool, player.Seat)
			if err != nil {
				return nil, err
			}
			value += roundValue
		}
		if value > bestValue {
			best, bestValue = combo, value
		}
	}
	return best, nil
}

func ChoosePoolIndex(state game.GameState, playerIndex int, attacking bool, revealedAttack *game.Card, difficulty Difficulty, rng *rand.Rand) (int, error) {
	player, err := state.Player(playerIndex)
	if err != nil {
		return 0, err
	}
	if len(player.Pool) == 0 {
		return 0, errors.New("no cards remain in pool")
	}
	if difficulty == Easy || (attacking && difficulty == Normal) {
		return rng.Intn(len(player.Pool)), nil
	}
	opponent, _ := state.Player(1 - playerIndex)
	bestIndex, bestValue := 0, -1<<30
	if !attacking {
		return chooseDefenseIndex(state, player, opponent, revealedAttack)
	}

	candidates := unseenCards(state, playerIndex)
	if len(candidates) == 0 {
		return 0, errors.New("no unseen cards available for attack evaluation")
	}
	if difficulty == Greedy {
		return chooseGreedyAttack(state, player, opponent, candidates)
	}
	for index, card := range player.Pool {
		worst := 1 << 30
		for sample := 0; sample < sampleCount; sample++ {
			oppCard := candidates[rng.Intn(len(candidates))]
			outcome := game.ResolveWithModifiers(card, oppCard, state.Modifiers)
			value := 0
			switch outcome.Winner {
			case 0:
				value, err = game.ScoreDeltaWithModifiers(*player, outcome.Captured, state.Bounty.Suit, state.Modifiers, state.Round)
			case 1:
				value, err = game.ScoreDeltaWithModifiers(*opponent, outcome.Captured, state.Bounty.Suit, state.Modifiers, state.Round)
				value = -value
			}
			if err != nil {
				return 0, err
			}
			if value < worst {
				worst = value
			}
		}
		if worst > bestValue || (worst == bestValue && card.Face > player.Pool[bestIndex].Face) {
			bestIndex, bestValue = index, worst
		}
	}
	return bestIndex, nil
}

func chooseDefenseIndex(state game.GameState, player, opponent *game.Player, revealedAttack *game.Card) (int, error) {
	if revealedAttack == nil {
		return 0, errors.New("defense requires the revealed attack card")
	}
	bestIndex, bestValue := 0, -1<<30
	for i, card := range player.Pool {
		outcome := game.ResolveWithModifiers(*revealedAttack, card, state.Modifiers)
		value, err := poolOutcomeValue(state, player, opponent, outcome, false)
		if err != nil {
			return 0, err
		}
		if value > bestValue || (value == bestValue && card.Face < player.Pool[bestIndex].Face) {
			bestIndex, bestValue = i, value
		}
	}
	return bestIndex, nil
}

func chooseGreedyAttack(state game.GameState, player, opponent *game.Player, candidates []game.Card) (int, error) {
	bestIndex, bestValue := 0, -1<<30
	for index, card := range player.Pool {
		expected := 0
		for _, oppCard := range candidates {
			outcome := game.ResolveWithModifiers(card, oppCard, state.Modifiers)
			value, err := poolOutcomeValue(state, player, opponent, outcome, true)
			if err != nil {
				return 0, err
			}
			expected += value
		}
		if expected > bestValue || (expected == bestValue && card.Face > player.Pool[bestIndex].Face) {
			bestIndex, bestValue = index, expected
		}
	}
	return bestIndex, nil
}

func poolOutcomeValue(state game.GameState, player, opponent *game.Player, outcome game.Outcome, attacking bool) (int, error) {
	if outcome.Winner == -1 {
		return 0, nil
	}
	won := (attacking && outcome.Winner == 0) || (!attacking && outcome.Winner == 1)
	if won {
		return game.ScoreDeltaWithModifiers(*player, outcome.Captured, state.Bounty.Suit, state.Modifiers, state.Round)
	}
	value, err := game.ScoreDeltaWithModifiers(*opponent, outcome.Captured, state.Bounty.Suit, state.Modifiers, state.Round)
	return -value, err
}

func ChooseSeat(state game.GameState, playerIndex int, difficulty Difficulty, rng *rand.Rand) (game.Seat, error) {
	player, err := state.Player(playerIndex)
	if err != nil {
		return game.SeatX, err
	}
	if difficulty != Hard {
		return game.Seat(rng.Intn(2)), nil
	}
	bestSeat, bestValue := player.Seat, -1<<30
	opponent, _ := state.Player(1 - playerIndex)
	ownCount := min(len(player.Hand), 3)
	oppCount := min(len(opponent.Hand), 3)
	ownCombos := combinations(len(player.Hand), ownCount)
	unseen := unseenCards(state, playerIndex)
	for _, seat := range []game.Seat{game.SeatX, game.SeatY} {
		seatValue := 0
		for _, combo := range ownCombos {
			ownPool := cardsAt(player.Hand, combo)
			for sample := 0; sample < sampleCount; sample++ {
				oppPool := sampleCards(unseen, oppCount, rng)
				roundValue, err := evaluateRound(state, playerIndex, ownPool, oppPool, seat)
				if err != nil {
					return game.SeatX, err
				}
				seatValue += roundValue
			}
		}
		if seatValue > bestValue {
			bestSeat, bestValue = seat, seatValue
		}
	}
	return bestSeat, nil
}

func unseenCards(state game.GameState, playerIndex int) []game.Card {
	own, _ := state.Player(playerIndex)
	opponent, _ := state.Player(1 - playerIndex)
	known := make(map[game.Card]bool)
	for _, cards := range [][]game.Card{
		own.Hand, own.Pool, own.Jail, opponent.Jail, state.Discard, {state.Bounty},
	} {
		for _, card := range cards {
			known[card] = true
		}
	}
	cards := make([]game.Card, 0, 27-len(known))
	for _, card := range game.NewDeck() {
		if !known[card] {
			cards = append(cards, card)
		}
	}
	return cards
}

func sampleCards(cards []game.Card, count int, rng *rand.Rand) []game.Card {
	if count > len(cards) {
		count = len(cards)
	}
	copyCards := append([]game.Card(nil), cards...)
	rng.Shuffle(len(copyCards), func(i, j int) { copyCards[i], copyCards[j] = copyCards[j], copyCards[i] })
	return copyCards[:count]
}

func evaluateRound(state game.GameState, ownIndex int, ownPool, opponentPool []game.Card, ownSeat game.Seat) (int, error) {
	own, _ := state.Player(ownIndex)
	opponent, _ := state.Player(1 - ownIndex)
	ownPlayer, opponentPlayer := *own, *opponent
	ownPlayer.Jail = append([]game.Card(nil), own.Jail...)
	opponentPlayer.Jail = append([]game.Card(nil), opponent.Jail...)
	value := 0
	for i := 0; i < min(len(ownPool), len(opponentPool)); i++ {
		ownAttacks := ownSeat == game.SeatX
		if i == 1 {
			ownAttacks = !ownAttacks
		}
		attacker, defender := ownPool[i], opponentPool[i]
		if !ownAttacks {
			attacker, defender = opponentPool[i], ownPool[i]
		}
		result := game.ResolveWithModifiers(attacker, defender, state.Modifiers)
		if result.Winner == -1 {
			continue
		}
		ownWins := (result.Winner == 0 && ownAttacks) || (result.Winner == 1 && !ownAttacks)
		if ownWins {
			delta, err := game.ScoreDeltaWithModifiers(ownPlayer, result.Captured, state.Bounty.Suit, state.Modifiers, state.Round)
			if err != nil {
				return 0, err
			}
			value += delta
			ownPlayer.Jail = append(ownPlayer.Jail, result.Captured)
			ownPlayer.CapturedByRound[state.Round-1][int(result.Captured.Suit)]++
		} else {
			delta, err := game.ScoreDeltaWithModifiers(opponentPlayer, result.Captured, state.Bounty.Suit, state.Modifiers, state.Round)
			if err != nil {
				return 0, err
			}
			value -= delta
			opponentPlayer.Jail = append(opponentPlayer.Jail, result.Captured)
			opponentPlayer.CapturedByRound[state.Round-1][int(result.Captured.Suit)]++
		}
	}
	return value, nil
}

func cardsAt(cards []game.Card, indices []int) []game.Card {
	result := make([]game.Card, 0, len(indices))
	for _, index := range indices {
		result = append(result, cards[index])
	}
	return result
}

func combinations(n, k int) [][]int {
	result := make([][]int, 0)
	var build func(start int, current []int)
	build = func(start int, current []int) {
		if len(current) == k {
			result = append(result, append([]int(nil), current...))
			return
		}
		for i := start; i <= n-(k-len(current)); i++ {
			build(i+1, append(current, i))
		}
	}
	build(0, nil)
	return result
}
