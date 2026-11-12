package app

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"prisonersofwar/internal/game"
)

const abandonDuelCPUALPHA = 0.3

type abandonDuelCPUMove struct {
	first        game.Card
	second       game.Card
	firstHidden  bool
	secondHidden bool
}

func CPUMove(
	knownBoard []game.Card,
	knownPrivate []game.Card,
	hiddenPrivate []game.Card,
	hiddenBoard []game.Card,
	hero abandonDuelHeroParams,
	bountySuit game.Suit,
	firstTurn bool,
) (game.Card, game.Card) {
	move := chooseAbandonDuelCPUMove(
		knownBoard, knownPrivate, hiddenPrivate, hiddenBoard,
		hero, bountySuit, firstTurn, rand.Intn,
	)
	return move.first, move.second
}

func chooseAbandonDuelCPUMove(
	knownBoard []game.Card,
	knownPrivate []game.Card,
	hiddenPrivate []game.Card,
	hiddenBoard []game.Card,
	hero abandonDuelHeroParams,
	bountySuit game.Suit,
	firstTurn bool,
	randomIntn func(int) int,
) abandonDuelCPUMove {
	return chooseAbandonDuelCPUMoveForPolicy(
		knownBoard, knownPrivate, hiddenPrivate, hiddenBoard, hero, bountySuit,
		firstTurn, randomIntn, "SMART", nil, nil,
	)
}

func chooseAbandonDuelCPUMoveForPolicy(
	knownBoard []game.Card,
	knownPrivate []game.Card,
	hiddenPrivate []game.Card,
	hiddenBoard []game.Card,
	hero abandonDuelHeroParams,
	bountySuit game.Suit,
	firstTurn bool,
	randomIntn func(int) int,
	policy string,
	prisoners []game.Card,
	opponentPrisoners []game.Card,
) abandonDuelCPUMove {
	known := append(append([]game.Card(nil), knownBoard...), knownPrivate...)
	hidden := append(append([]game.Card(nil), hiddenPrivate...), hiddenBoard...)
	alpha := abandonDuelCPUALPHA
	switch policy {
	case "GREEDY", "BOUNTY_FOCUSED":
		alpha = 0
	case "SMART_HIGH":
		alpha = 0.5
	}
	bestScore := math.Inf(-1)
	var best abandonDuelCPUMove
	foundMove := false

	for firstIndex := 0; firstIndex < len(known); firstIndex++ {
		for secondIndex := firstIndex + 1; secondIndex < len(known); secondIndex++ {
			captured, _ := abandonDuelResolve(known[firstIndex], known[secondIndex], hero.S, hero.F)
			if captured == nil {
				continue
			}
			score := abandonDuelPolicyValue(
				*captured, bountySuit, hero, policy, prisoners, opponentPrisoners,
			)
			if !firstTurn {
				if containsAbandonDuelCard(knownPrivate, known[firstIndex]) {
					score += 0.5 * float64(abandonDuelPenalty(known[firstIndex], bountySuit, hero.B, hero.S))
				}
				if containsAbandonDuelCard(knownPrivate, known[secondIndex]) {
					score += 0.5 * float64(abandonDuelPenalty(known[secondIndex], bountySuit, hero.B, hero.S))
				}
			}
			score += alpha * float64(abandonDuelLookahead(
				removeAbandonDuelCards(known, firstIndex, secondIndex),
				bountySuit, hero.B, hero.S, hero.F,
			))
			if !foundMove || score > bestScore {
				bestScore = score
				best = abandonDuelCPUMove{first: known[firstIndex], second: known[secondIndex]}
				foundMove = true
			}
		}
	}

	if len(hidden) > 0 {
		for knownIndex, knownCard := range known {
			expectation := 0.0
			captureCount := 0
			for _, hiddenCard := range hidden {
				captured, _ := abandonDuelResolve(hiddenCard, knownCard, hero.S, hero.F)
				if captured == nil {
					continue
				}
				expectation += abandonDuelPolicyValue(
					*captured, bountySuit, hero, policy, prisoners, opponentPrisoners,
				)
				captureCount++
			}
			if captureCount == 0 {
				continue
			}
			score := expectation / float64(captureCount)
			if !firstTurn && containsAbandonDuelCard(knownPrivate, knownCard) {
				score += 0.5 * float64(abandonDuelPenalty(knownCard, bountySuit, hero.B, hero.S))
			}
			score += alpha * float64(abandonDuelLookahead(
				removeAbandonDuelCard(known, knownIndex),
				bountySuit, hero.B, hero.S, hero.F,
			))
			if !foundMove || score > bestScore {
				bestScore = score
				best = abandonDuelCPUMove{first: knownCard, secondHidden: true}
				foundMove = true
			}
		}
	}

	if len(hidden) >= 2 {
		expectation := 0.0
		captureCount := 0
		for firstIndex := 0; firstIndex < len(hidden); firstIndex++ {
			for secondIndex := firstIndex + 1; secondIndex < len(hidden); secondIndex++ {
				captured, _ := abandonDuelResolve(hidden[firstIndex], hidden[secondIndex], hero.S, hero.F)
				if captured == nil {
					continue
				}
				expectation += abandonDuelPolicyValue(
					*captured, bountySuit, hero, policy, prisoners, opponentPrisoners,
				)
				captureCount++
			}
		}
		if captureCount > 0 {
			score := expectation / float64(captureCount)
			score += alpha * float64(abandonDuelLookahead(
				known, bountySuit, hero.B, hero.S, hero.F,
			))
			if !foundMove || score > bestScore {
				bestScore = score
				best = abandonDuelCPUMove{firstHidden: true, secondHidden: true}
				foundMove = true
			}
		}
	}

	if !foundMove {
		pool := append(append([]game.Card(nil), known...), hidden...)
		if len(pool) < 2 {
			return abandonDuelCPUMove{}
		}
		return abandonDuelCPUMove{first: pool[0], second: pool[1]}
	}

	if best.firstHidden {
		firstIndex := randomIntn(len(hidden))
		best.first = hidden[firstIndex]
		if best.secondHidden {
			secondIndex := randomIntn(len(hidden) - 1)
			if secondIndex >= firstIndex {
				secondIndex++
			}
			best.second = hidden[secondIndex]
		}
	} else if best.secondHidden {
		best.second = hidden[randomIntn(len(hidden))]
	}
	return best
}

func abandonDuelPolicyValue(
	card game.Card,
	bountySuit game.Suit,
	hero abandonDuelHeroParams,
	policy string,
	prisoners, opponentPrisoners []game.Card,
) float64 {
	value := float64(abandonDuelJailValue(card, bountySuit, hero.B, hero.S))
	switch policy {
	case "BOUNTY_FOCUSED":
		if card.Suit == bountySuit {
			value += 100
		}
	case "ANTI_CONCENTRATE":
		for _, prisoner := range prisoners {
			if prisoner.Suit == card.Suit {
				value -= 5
			}
		}
	case "ANTI_SPREAD":
		hasSuit := false
		for _, prisoner := range opponentPrisoners {
			hasSuit = hasSuit || prisoner.Suit == card.Suit
		}
		if !hasSuit {
			value += 10
		}
	}
	return value
}

func abandonDuelResolve(first, second game.Card, shift, favor int) (*game.Card, *game.Card) {
	if first.Suit == second.Suit {
		switch {
		case first.Face < second.Face:
			return &first, &second
		case second.Face < first.Face:
			return &second, &first
		default:
			return nil, nil
		}
	}
	dominant, ok := abandonDuelDominantSuit(first.Suit, second.Suit)
	if !ok {
		return nil, nil
	}
	dominantCard, subordinateCard := first, second
	if second.Suit == dominant {
		dominantCard, subordinateCard = second, first
	}
	dominantValue := 2*(dominantCard.Face+shift) + favor
	subordinateValue := subordinateCard.Face + shift
	switch {
	case dominantValue == subordinateValue:
		return nil, nil
	case dominantValue > subordinateValue:
		return &subordinateCard, &dominantCard
	default:
		return &dominantCard, &subordinateCard
	}
}

func abandonDuelDominantSuit(first, second game.Suit) (game.Suit, bool) {
	if first == second {
		return 0, false
	}
	if first.Beats(second) {
		return first, true
	}
	if second.Beats(first) {
		return second, true
	}
	return 0, false
}

func abandonDuelJailValue(card game.Card, bounty game.Suit, bountyBonus, shift int) int {
	value := card.Face + shift
	if card.Suit == bounty {
		value += 1 + bountyBonus
	}
	return value
}

func abandonDuelLookahead(pool []game.Card, bounty game.Suit, bountyBonus, shift, favor int) int {
	best := 0
	for firstIndex := 0; firstIndex < len(pool); firstIndex++ {
		for secondIndex := firstIndex + 1; secondIndex < len(pool); secondIndex++ {
			captured, _ := abandonDuelResolve(pool[firstIndex], pool[secondIndex], shift, favor)
			if captured == nil {
				continue
			}
			value := abandonDuelJailValue(*captured, bounty, bountyBonus, shift)
			if value > best {
				best = value
			}
		}
	}
	return best
}

func containsAbandonDuelCard(cards []game.Card, target game.Card) bool {
	for _, card := range cards {
		if card == target {
			return true
		}
	}
	return false
}

func removeAbandonDuelCard(cards []game.Card, index int) []game.Card {
	result := make([]game.Card, 0, len(cards)-1)
	result = append(result, cards[:index]...)
	result = append(result, cards[index+1:]...)
	return result
}

func removeAbandonDuelCards(cards []game.Card, firstIndex, secondIndex int) []game.Card {
	result := make([]game.Card, 0, len(cards)-2)
	for index, card := range cards {
		if index != firstIndex && index != secondIndex {
			result = append(result, card)
		}
	}
	return result
}

func (s *webServer) advanceAbandonDuelAI() error {
	session := s.session
	if session == nil || session.abandonDuel == nil {
		return errors.New("Abandon Duel state is unavailable")
	}
	if session.mode == "hvh" || s.onlineViewer {
		return nil
	}
	if s.rng == nil {
		return errors.New("Abandon Duel CPU requires a random source")
	}
	duel := session.abandonDuel
	for !duel.GameOver && isAI(session.mode, duel.ActivePlayer) {
		player := duel.ActivePlayer
		if !duel.FlippedThisTurn {
			hiddenBoard := duel.hiddenBoardIndices()
			if len(hiddenBoard) > 0 {
				if err := duel.flipBoardCard(hiddenBoard[s.rng.Intn(len(hiddenBoard))]); err != nil {
					return err
				}
			}
		}
		knownBoard := make([]game.Card, 0, len(duel.Board))
		hiddenBoard := make([]game.Card, 0, len(duel.Board))
		for index, card := range duel.Board {
			if duel.BoardUsed[index] {
				continue
			}
			if duel.BoardRevealed[index] {
				knownBoard = append(knownBoard, card)
			} else {
				hiddenBoard = append(hiddenBoard, card)
			}
		}
		knownPrivate := make([]game.Card, 0, len(duel.Players[player].PersonalPile))
		hiddenPrivate := make([]game.Card, 0, len(duel.Players[player].PersonalPile))
		for index, card := range duel.Players[player].PersonalPile {
			if duel.Players[player].PileUsed[index] {
				continue
			}
			if duel.Players[player].PileRevealed[index] {
				knownPrivate = append(knownPrivate, card)
			} else {
				hiddenPrivate = append(hiddenPrivate, card)
			}
		}
		hero := abandonDuelHeroParams{
			B: duel.Modifiers.B, S: duel.Modifiers.S,
			F: duel.Modifiers.F, D: duel.Modifiers.D,
		}
		move := chooseAbandonDuelCPUMoveForPolicy(
			knownBoard, knownPrivate, hiddenPrivate, hiddenBoard, hero,
			duel.Bounty.Suit, duel.Players[player].TurnsTaken == 1, s.rng.Intn,
			duel.Setup.OpponentAlgo, duel.Players[player].Prisoners,
			duel.Players[1-player].Prisoners,
		)
		firstSource, firstIndex, err := duel.sourceForCard(player, move.first)
		if err != nil {
			return err
		}
		secondSource, secondIndex, err := duel.sourceForCard(player, move.second)
		if err != nil {
			return err
		}
		if _, err := duel.selectPairCard(firstSource, firstIndex, s.rng); err != nil {
			return err
		}
		if _, err := duel.selectPairCard(secondSource, secondIndex, s.rng); err != nil {
			return err
		}
		session.actor = duel.ActivePlayer
		session.phase = duel.phase()
		if duel.GameOver {
			session.winner = duel.Winner
			session.message = abandonDuelResultMessage(duel, s.playerName)
			s.finishAbandonDuel()
		} else if duel.Round == 2 && duel.RoundTurn == 0 && duel.Players[duel.ActivePlayer].TurnsTaken == 1 {
			session.message = "Round 1 complete. Seats swapped; Round 2 begins."
		} else {
			session.message = fmt.Sprintf("%s completed a pair.", s.playerName(player))
		}
	}
	return nil
}

func (g *abandonDuelGame) sourceForCard(player int, target game.Card) (string, int, error) {
	for index, card := range g.Board {
		if !g.BoardUsed[index] && card == target {
			return abandonBoardSource, index, nil
		}
	}
	for index, card := range g.Players[player].PersonalPile {
		if !g.Players[player].PileUsed[index] && card == target {
			return abandonPileSource, index, nil
		}
	}
	return "", -1, fmt.Errorf("Abandon Duel CPU selected unavailable card %s", target)
}

func abandonDuelResultMessage(duel *abandonDuelGame, playerName func(int) string) string {
	switch duel.Winner {
	case -1:
		return fmt.Sprintf(
			"Abandon Duel ends in a tie: %s %d, %s %d.",
			playerName(0), duel.TotalScores[0], playerName(1), duel.TotalScores[1],
		)
	default:
		return fmt.Sprintf(
			"%s wins Abandon Duel: %d to %d.",
			playerName(duel.Winner), duel.TotalScores[duel.Winner], duel.TotalScores[1-duel.Winner],
		)
	}
}
