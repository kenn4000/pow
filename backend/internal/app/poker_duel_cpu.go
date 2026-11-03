package app

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"prisonersofwar/internal/game"
)

type pokerDuelCPUProfile struct {
	useFullScoring  bool
	redrawThreshold float64
	errorRate       float64
	minThink        time.Duration
	maxThink        time.Duration
}

type pokerDuelCPUAction struct {
	request actionRequest
	score   float64
	redraw  bool
}

func pokerDuelCPUProfileForMode(mode string) (pokerDuelCPUProfile, error) {
	switch mode {
	case "easy":
		return pokerDuelCPUProfile{
			redrawThreshold: 5, errorRate: 0.15,
			minThink: 5 * time.Second, maxThink: 12 * time.Second,
		}, nil
	case "normal":
		return pokerDuelCPUProfile{
			useFullScoring: true, redrawThreshold: 10, errorRate: 0.05,
			minThink: 4 * time.Second, maxThink: 8 * time.Second,
		}, nil
	case "hard":
		return pokerDuelCPUProfile{
			useFullScoring: true, redrawThreshold: 15,
			minThink: 2 * time.Second, maxThink: 6 * time.Second,
		}, nil
	default:
		return pokerDuelCPUProfile{}, fmt.Errorf("unknown Poker CPU difficulty %q", mode)
	}
}

func pokerDuelCPUThinkingDuration(mode string, rng *rand.Rand) (time.Duration, error) {
	profile, err := pokerDuelCPUProfileForMode(mode)
	if err != nil {
		return 0, err
	}
	if rng == nil {
		return 0, errors.New("Poker CPU requires a random source")
	}
	duration := profile.minThink + time.Duration(rng.Float64()*float64(profile.maxThink-profile.minThink))
	return minDuration(duration, 15*time.Second), nil
}

func minDuration(first, second time.Duration) time.Duration {
	if first < second {
		return first
	}
	return second
}

func choosePokerDuelCPUAction(duel *pokerDuelGame, mode string, rng *rand.Rand) (actionRequest, bool, error) {
	if duel == nil {
		return actionRequest{}, false, errors.New("Poker duel state is unavailable")
	}
	if rng == nil {
		return actionRequest{}, false, errors.New("Poker CPU requires a random source")
	}
	profile, err := pokerDuelCPUProfileForMode(mode)
	if err != nil {
		return actionRequest{}, false, err
	}
	if duel.ActivePlayer < 0 || duel.ActivePlayer >= len(duel.Players) {
		return actionRequest{}, false, errors.New("Poker duel has an invalid active player")
	}
	player := &duel.Players[duel.ActivePlayer]
	if player.Done {
		return actionRequest{}, false, nil
	}

	actions := make([]pokerDuelCPUAction, 0, len(player.Hand)*4)
	for cardIndex, card := range player.Hand {
		flexibility := pokerDuelCPUCardFlexibility(card, player.Hand, cardIndex, duel.BountySuit)
		for slotIndex := range player.Table {
			if len(player.Table[slotIndex]) >= pokerSlotCapacity ||
				!game.CanPlacePokerCard(player.Table, slotIndex) {
				continue
			}
			hand := append(append([]game.Card(nil), player.Table[slotIndex]...), card)
			immediate, err := pokerDuelCPUHandValue(duel, hand, slotIndex)
			if err != nil {
				return actionRequest{}, false, err
			}
			score := float64(immediate)
			if profile.useFullScoring {
				future := float64(immediate) / float64(pokerSlotCapacity-len(player.Table[slotIndex]))
				rank, err := pokerDuelCPURankBonus(duel, slotIndex, immediate)
				if err != nil {
					return actionRequest{}, false, err
				}
				score += 0.5*future + rank - float64(flexibility)/5
			}
			actions = append(actions, pokerDuelCPUAction{
				request: actionRequest{Action: "poker-duel-place", Index: cardIndex, Slot: slotIndex},
				score:   score,
			})
		}
		if len(player.DrawPile) > 0 {
			actions = append(actions, pokerDuelCPUAction{
				request: actionRequest{Action: "poker-duel-redraw", Index: cardIndex},
				score:   -float64(flexibility) / 5,
				redraw:  true,
			})
		}
	}
	if len(actions) == 0 {
		return actionRequest{}, false, nil
	}

	bestPlacement, bestRedraw := -1, -1
	bestAction := 0
	for index, action := range actions {
		if action.score > actions[bestAction].score {
			bestAction = index
		}
		if action.redraw {
			if bestRedraw < 0 || action.score > actions[bestRedraw].score {
				bestRedraw = index
			}
		} else if bestPlacement < 0 || action.score > actions[bestPlacement].score {
			bestPlacement = index
		}
	}
	if bestPlacement < 0 {
		bestAction = bestRedraw
	} else if bestRedraw >= 0 && actions[bestPlacement].score < profile.redrawThreshold {
		bestAction = bestRedraw
	}
	if rng.Float64() < profile.errorRate {
		bestAction = rng.Intn(len(actions))
	}
	return actions[bestAction].request, true, nil
}

func pokerDuelCPUHandValue(duel *pokerDuelGame, hand []game.Card, slot int) (int, error) {
	if len(hand) == pokerSlotCapacity {
		var hands [3][]game.Card
		hands[slot] = hand
		score, err := game.ScorePokerTable(
			hands, duel.sharedHeroParameters(), duel.BountySuit, duel.Rules,
		)
		if err != nil {
			return 0, err
		}
		return score.Hands[slot].Total, nil
	}
	return pokerDuelCPUPartialValue(hand, duel.BountySuit), nil
}

func pokerDuelCPUPartialValue(hand []game.Card, bountySuit game.Suit) int {
	rankCounts := make(map[int]int, len(hand))
	suitCounts := make(map[game.Suit]int, len(hand))
	ranks := make([]int, 0, len(hand))
	ninePresent := false
	bountyCount := 0
	for _, card := range hand {
		rankCounts[card.Face]++
		suitCounts[card.Suit]++
		ranks = append(ranks, card.Face)
		ninePresent = ninePresent || card.Face == 9
		if card.Suit == bountySuit {
			bountyCount++
		}
	}
	pairValue := 0
	for _, count := range rankCounts {
		pairValue += count * (count - 1) / 2
	}
	maxSuitCount := 0
	for _, count := range suitCounts {
		if count > maxSuitCount {
			maxSuitCount = count
		}
	}
	sort.Ints(ranks)
	longestRun, currentRun := 0, 0
	previous := -1
	for _, rank := range ranks {
		if rank == previous {
			continue
		}
		if rank == previous+1 {
			currentRun++
		} else {
			currentRun = 1
		}
		if currentRun > longestRun {
			longestRun = currentRun
		}
		previous = rank
	}
	nineValue := 0
	if ninePresent {
		nineValue = 3
	}
	return pairValue*8 + maxSuitCount*5 + longestRun*4 + nineValue + bountyCount*2
}

func pokerDuelCPUCardFlexibility(card game.Card, hand []game.Card, cardIndex int, bountySuit game.Suit) int {
	score := 0
	if card.Face == 9 {
		score += 3
	}
	if card.Suit == bountySuit {
		score += 2
	}
	formsPair, joinsRun := false, false
	for index, other := range hand {
		if index == cardIndex {
			continue
		}
		if card.Face == other.Face {
			formsPair = true
		}
		distance := card.Face - other.Face
		if distance < 0 {
			distance = -distance
		}
		if card.Suit == other.Suit && distance <= 1 {
			joinsRun = true
		}
	}
	if formsPair {
		score++
	}
	if joinsRun {
		score++
	}
	return score
}

func pokerDuelCPURankBonus(duel *pokerDuelGame, slot, afterValue int) (float64, error) {
	current := [3]int{}
	for index, hand := range duel.Players[duel.ActivePlayer].Table {
		value, err := pokerDuelCPUHandValue(duel, hand, index)
		if err != nil {
			return 0, err
		}
		current[index] = value
	}
	beforeOrder, beforeStrict := pokerDuelStrictRankOrder(current)
	current[slot] = afterValue
	afterOrder, afterStrict := pokerDuelStrictRankOrder(current)
	if !beforeStrict || !afterStrict {
		return 0, nil
	}
	if beforeOrder == afterOrder {
		return 2, nil
	}
	return -2, nil
}

func pokerDuelStrictRankOrder(scores [3]int) ([3]int, bool) {
	if scores[0] == scores[1] || scores[0] == scores[2] || scores[1] == scores[2] {
		return [3]int{}, false
	}
	order := [3]int{0, 1, 2}
	sort.Slice(order[:], func(first, second int) bool {
		return scores[order[first]] > scores[order[second]]
	})
	return order, true
}

func (s *webServer) schedulePokerDuelCPU() error {
	session := s.session
	if session == nil || session.variant != "poker-duel" || session.pokerDuel == nil ||
		session.mode == "hvh" || session.pokerDuel.GameOver {
		return nil
	}
	if session.pokerDuel.ActivePlayer != 1 {
		session.pokerDuelCPUReadyAt = time.Time{}
		return nil
	}
	if session.phase == "poker-duel-thinking" && !session.pokerDuelCPUReadyAt.IsZero() {
		return nil
	}
	delay, err := pokerDuelCPUThinkingDuration(session.mode, s.rng)
	if err != nil {
		return err
	}
	session.actor = 1
	session.phase = "poker-duel-thinking"
	session.pokerDuelCPUReadyAt = time.Now().Add(delay)
	session.message = s.playerName(1) + " is thinking."
	return nil
}

func (s *webServer) applyPokerDuelCPUAction() error {
	session := s.session
	if session == nil || session.variant != "poker-duel" || session.pokerDuel == nil ||
		session.mode == "hvh" || session.phase != "poker-duel-thinking" ||
		session.actor != 1 || session.pokerDuel.ActivePlayer != 1 {
		return errors.New("there is no pending Poker CPU turn")
	}
	readyAt := session.pokerDuelCPUReadyAt
	if readyAt.IsZero() {
		return errors.New("Poker CPU turn is not scheduled")
	}
	now := time.Now()
	if now.Before(readyAt) {
		return errors.New("Poker CPU is still thinking")
	}
	duel := session.pokerDuel
	action, hasAction, err := choosePokerDuelCPUAction(duel, session.mode, s.rng)
	if err != nil {
		return err
	}
	if !hasAction {
		duel.Players[1].Done = true
		session.pokerDuelCPUReadyAt = time.Time{}
		if err := duel.completeTurn(now); err != nil {
			return err
		}
		s.finishPokerDuelTurn(1, now, "pass", false)
		return nil
	}

	originalTurnStarted := duel.TurnStarted
	thinkTime := readyAt.Sub(originalTurnStarted)
	if thinkTime < 0 {
		thinkTime = 0
	}
	if thinkTime > time.Duration(pokerDuelTurnSeconds)*time.Second {
		thinkTime = time.Duration(pokerDuelTurnSeconds) * time.Second
	}
	duel.TurnStarted = now.Add(-thinkTime)
	session.pokerDuelCPUReadyAt = time.Time{}
	session.phase = "poker-duel-play"
	if err := s.applyPokerDuelTurn(action, now); err != nil {
		duel.TurnStarted = originalTurnStarted
		session.pokerDuelCPUReadyAt = readyAt
		session.phase = "poker-duel-thinking"
		return err
	}
	return nil
}

func pokerDuelCPUPlayer(mode string) int {
	if mode == "hvh" {
		return -1
	}
	return 1
}
