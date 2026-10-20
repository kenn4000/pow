package ai

import (
	"errors"
	"math/rand"

	"prisonersofwar/game"
)

type Difficulty int

const (
	Easy Difficulty = iota
	Normal
	Hard
)

const sampleCount = 8

func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "Easy"
	case Normal:
		return "Normal"
	case Hard:
		return "Hard"
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
	if difficulty == Normal {
		best, bestValue := combos[0], -1<<30
		for _, combo := range combos {
			value := 0
			for _, index := range combo {
				value += game.ScoreDelta(player.Jail, player.Hand[index], state.Bounty.Suit)
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
			value += evaluateRound(state, playerIndex, ownPool, oppPool, player.Seat)
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
		if revealedAttack == nil {
			return 0, errors.New("defense requires the revealed attack card")
		}
		for i, card := range player.Pool {
			outcome := game.Resolve(*revealedAttack, card)
			value := 0
			switch outcome.Winner {
			case 1:
				value = game.ScoreDelta(player.Jail, outcome.Captured, state.Bounty.Suit)
			case 0:
				value = -game.ScoreDelta(opponent.Jail, outcome.Captured, state.Bounty.Suit)
			}
			if value > bestValue || (value == bestValue && card.Face < player.Pool[bestIndex].Face) {
				bestIndex, bestValue = i, value
			}
		}
		return bestIndex, nil
	}

	candidates := unseenCards(state, playerIndex)
	if len(candidates) == 0 {
		return 0, errors.New("no unseen cards available for attack evaluation")
	}
	for index, card := range player.Pool {
		worst := 1 << 30
		for sample := 0; sample < sampleCount; sample++ {
			oppCard := candidates[rng.Intn(len(candidates))]
			outcome := game.Resolve(card, oppCard)
			value := 0
			switch outcome.Winner {
			case 0:
				value = game.ScoreDelta(player.Jail, outcome.Captured, state.Bounty.Suit)
			case 1:
				value = -game.ScoreDelta(opponent.Jail, outcome.Captured, state.Bounty.Suit)
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
				seatValue += evaluateRound(state, playerIndex, ownPool, oppPool, seat)
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

func evaluateRound(state game.GameState, ownIndex int, ownPool, opponentPool []game.Card, ownSeat game.Seat) int {
	own, _ := state.Player(ownIndex)
	opponent, _ := state.Player(1 - ownIndex)
	ownJail := append([]game.Card(nil), own.Jail...)
	opponentJail := append([]game.Card(nil), opponent.Jail...)
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
		result := game.Resolve(attacker, defender)
		if result.Winner == -1 {
			continue
		}
		ownWins := (result.Winner == 0 && ownAttacks) || (result.Winner == 1 && !ownAttacks)
		if ownWins {
			delta := game.ScoreDelta(ownJail, result.Captured, state.Bounty.Suit)
			value += delta
			ownJail = append(ownJail, result.Captured)
		} else {
			delta := game.ScoreDelta(opponentJail, result.Captured, state.Bounty.Suit)
			value -= delta
			opponentJail = append(opponentJail, result.Captured)
		}
	}
	return value
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
