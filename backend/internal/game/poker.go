package game

import (
	"errors"
	"fmt"
)

type PokerHandType string

const (
	PokerStraightFlush PokerHandType = "straight_flush"
	PokerThreeOfAKind  PokerHandType = "three_of_a_kind"
	PokerStraight      PokerHandType = "straight"
	PokerFlush         PokerHandType = "flush"
	PokerPair          PokerHandType = "pair"
	PokerHighCard      PokerHandType = "high_card"
)

const PokerHandCapacity = 5

type PokerMissionConstraint string

const (
	PokerNoStraightsOrFlushes PokerMissionConstraint = "no_straights_or_flushes"
	PokerNineInEveryHand      PokerMissionConstraint = "nine_in_every_hand"
	PokerNoNines              PokerMissionConstraint = "no_nines"
	PokerSameBountySuitEvery  PokerMissionConstraint = "same_bounty_suit_every_hand"
)

type PokerGhostStrategy string

const (
	PokerGhostBounty PokerGhostStrategy = "bounty"
	PokerGhostNine   PokerGhostStrategy = "nine"
	PokerGhostPairs  PokerGhostStrategy = "pairs"
	PokerGhostMedium PokerGhostStrategy = "balanced"
)

type PokerGhostRules struct {
	Strategy     PokerGhostStrategy
	StartingHand []Card
	HandSize     int
	TeachingNote string
}

type PokerMissionRules struct {
	NineBase          int
	BountyBase        int
	Par               int
	Constraint        PokerMissionConstraint
	StreakCount       int
	StreakScore       int
	ForcedBountySuit  *Suit
	StartingHand      []Card
	HandSize          int
	Lesson            string
	RecommendedHeroes []string
	Ghost             *PokerGhostRules
}

type PokerHeroParameters struct {
	Bounty  int
	Favored int
	Shift   int
	Draft   int
}

type PokerHandScore struct {
	Type           PokerHandType `json:"type"`
	Cards          int           `json:"cards"`
	Base           int           `json:"base"`
	BaseSuppressed bool          `json:"baseSuppressed,omitempty"`
	NineBonus      int           `json:"nineBonus"`
	BountyBonus    int           `json:"bountyBonus"`
	BustPenalty    int           `json:"bustPenalty"`
	Total          int           `json:"total"`
	Void           bool          `json:"void,omitempty"`
}

type PokerTableScore struct {
	Hands             [3]PokerHandScore `json:"hands"`
	Total             int               `json:"total"`
	TableBonus        int               `json:"tableBonus"`
	HandAHighBonus    int               `json:"handAHighBonus,omitempty"`
	HandCLowBonus     int               `json:"handCLowBonus,omitempty"`
	ConstraintPassed  bool              `json:"constraintPassed"`
	ConstraintFailure string            `json:"constraintFailure,omitempty"`
}

func PokerHandsBalanced(hands [3][]Card) bool {
	minCount, maxCount := len(hands[0]), len(hands[0])
	for _, hand := range hands[1:] {
		if len(hand) < minCount {
			minCount = len(hand)
		}
		if len(hand) > maxCount {
			maxCount = len(hand)
		}
	}
	return maxCount-minCount <= 1
}

func CanPlacePokerCard(hands [3][]Card, slot int) bool {
	if slot < 0 || slot >= len(hands) || len(hands[slot]) >= PokerHandCapacity ||
		!PokerHandsBalanced(hands) {
		return false
	}
	if len(hands[0])+len(hands[1])+len(hands[2]) == 0 && slot != 0 {
		return false
	}
	next := [3]int{len(hands[0]), len(hands[1]), len(hands[2])}
	next[slot]++
	minCount, maxCount := next[0], next[0]
	for _, count := range next[1:] {
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount-minCount <= 1
}

func ScorePokerTable(hands [3][]Card, hero PokerHeroParameters, bountySuit Suit, rules PokerMissionRules) (PokerTableScore, error) {
	if !bountySuit.Valid() {
		return PokerTableScore{}, errors.New("invalid poker bounty suit")
	}
	seen := make(map[Card]bool, 15)
	for _, hand := range hands {
		if len(hand) > PokerHandCapacity {
			return PokerTableScore{}, errors.New("poker hands cannot contain more than five cards")
		}
		for _, card := range hand {
			if !card.Valid() {
				return PokerTableScore{}, fmt.Errorf("invalid poker card %v", card)
			}
			if seen[card] {
				return PokerTableScore{}, fmt.Errorf("duplicate poker card %s", card)
			}
			seen[card] = true
		}
	}
	failure, err := pokerConstraintFailure(hands, bountySuit, rules.Constraint)
	if err != nil {
		return PokerTableScore{}, err
	}
	if failure != "" {
		return PokerTableScore{ConstraintFailure: failure}, nil
	}

	result := PokerTableScore{ConstraintPassed: true}
	allPairOrBetter := true
	allHandsComplete := true
	for index, hand := range hands {
		scored := PokerHandScore{Cards: len(hand)}
		if len(hand) != 5 {
			scored.Void = true
			allPairOrBetter = false
			allHandsComplete = false
			result.Hands[index] = scored
			continue
		}

		scored.Type, scored.Base = evaluatePokerHand(hand)
		if rules.Constraint == PokerNoStraightsOrFlushes &&
			(scored.Type == PokerStraightFlush || scored.Type == PokerStraight || scored.Type == PokerFlush) {
			scored.Base = 0
			scored.BaseSuppressed = true
		}
		if scored.Base < 20 {
			allPairOrBetter = false
		}

		containsNine, bountyCount := false, 0
		for _, card := range hand {
			containsNine = containsNine || card.Face == 9
			if card.Suit == bountySuit {
				bountyCount++
			}
		}
		if containsNine {
			scored.NineBonus = rules.NineBase + 2*hero.Shift
		}
		if bountyCount >= 2-hero.Favored {
			scored.BountyBonus = max(0, rules.BountyBase+hero.Bounty)
		}
		if scored.Base == 0 {
			scored.BustPenalty = -10
		}
		scored.Total = scored.Base + scored.NineBonus + scored.BountyBonus + scored.BustPenalty
		result.Total += scored.Total
		result.Hands[index] = scored
	}
	if allPairOrBetter {
		result.TableBonus = 15
		result.Total += result.TableBonus
	}
	if allHandsComplete {
		if result.Hands[0].Total > result.Hands[1].Total &&
			result.Hands[0].Total > result.Hands[2].Total {
			result.HandAHighBonus = 15
			result.Total += result.HandAHighBonus
		}
		if result.Hands[2].Total < result.Hands[0].Total &&
			result.Hands[2].Total < result.Hands[1].Total {
			result.HandCLowBonus = 5
			result.Total += result.HandCLowBonus
		}
	}
	return result, nil
}

func pokerConstraintFailure(hands [3][]Card, bountySuit Suit, constraint PokerMissionConstraint) (string, error) {
	switch constraint {
	case "":
		return "", nil
	case PokerNoStraightsOrFlushes:
		return "", nil
	case PokerNineInEveryHand:
		for index, hand := range hands {
			if len(hand) != 5 || !pokerHandContains(hand, func(card Card) bool { return card.Face == 9 }) {
				return fmt.Sprintf("Hand %c must contain a 9.", 'A'+index), nil
			}
		}
	case PokerNoNines:
		for index, hand := range hands {
			if len(hand) == 5 && pokerHandContains(hand, func(card Card) bool { return card.Face == 9 }) {
				return fmt.Sprintf("Hand %c cannot contain a 9.", 'A'+index), nil
			}
		}
	case PokerSameBountySuitEvery:
		for index, hand := range hands {
			if len(hand) != 5 || !pokerHandContains(hand, func(card Card) bool { return card.Suit == bountySuit }) {
				return fmt.Sprintf("Hand %c must contain a bounty-suit card.", 'A'+index), nil
			}
		}
	default:
		return "", fmt.Errorf("unsupported poker mission constraint %q", constraint)
	}
	return "", nil
}

func pokerHandContains(hand []Card, predicate func(Card) bool) bool {
	for _, card := range hand {
		if predicate(card) {
			return true
		}
	}
	return false
}

func evaluatePokerHand(hand []Card) (PokerHandType, int) {
	counts := make(map[int]int, 5)
	minFace, maxFace := 10, 0
	flush := true
	firstSuit := hand[0].Suit
	for _, card := range hand {
		counts[card.Face]++
		if card.Face < minFace {
			minFace = card.Face
		}
		if card.Face > maxFace {
			maxFace = card.Face
		}
		flush = flush && card.Suit == firstSuit
	}
	straight := len(counts) == 5 && maxFace-minFace == 4
	if straight && flush {
		return PokerStraightFlush, 100
	}
	for _, count := range counts {
		if count == 3 {
			return PokerThreeOfAKind, 60
		}
	}
	if straight {
		return PokerStraight, 50
	}
	if flush {
		return PokerFlush, 40
	}
	for _, count := range counts {
		if count >= 2 {
			return PokerPair, 20
		}
	}
	return PokerHighCard, 0
}
