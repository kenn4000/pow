package game

import (
	"math/bits"
	"sort"
)

type RummyParameters struct {
	J        int `json:"j"`
	K        int `json:"k"`
	B        int `json:"b"`
	D        int `json:"d"`
	HandSize int `json:"handSize"`
}

type RummyMeld struct {
	Kind   string `json:"kind"`
	Cards  []Card `json:"cards"`
	Points int    `json:"points"`
}

type RummyPartition struct {
	Melds           []RummyMeld `json:"melds"`
	Deadwood        []Card      `json:"deadwood"`
	MeldPoints      int         `json:"meldPoints"`
	DeadwoodPenalty int         `json:"deadwoodPenalty"`
	Total           int         `json:"total"`
}

func NewRummyParameters(favor, shift, bounty, draft int) RummyParameters {
	d := clampInt(draft, -1, 1)
	return RummyParameters{
		J:        6 + clampInt(favor, -2, 2),
		K:        clampInt(shift, 0, 3),
		B:        clampInt(bounty, -2, 3),
		D:        d,
		HandSize: 6 + d,
	}
}

func BestRummyPartition(hand []Card, params RummyParameters) RummyPartition {
	cards := append([]Card(nil), hand...)
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Suit != cards[j].Suit {
			return cards[i].Suit < cards[j].Suit
		}
		return cards[i].Face < cards[j].Face
	})
	if len(cards) == 0 {
		return RummyPartition{Melds: []RummyMeld{}, Deadwood: []Card{}}
	}
	if len(cards) > 63 {
		return scoreRummyHandWithoutMelds(cards, params)
	}

	candidates := makeRummyCandidates(cards, params)
	byCard := make([][]int, len(cards))
	for candidateIndex, candidate := range candidates {
		for cardIndex := range cards {
			if candidate.mask&(uint64(1)<<cardIndex) != 0 {
				byCard[cardIndex] = append(byCard[cardIndex], candidateIndex)
			}
		}
	}

	type solution struct {
		meldPoints int
		total      int
		melds      []int
	}
	memo := make(map[uint64]solution)
	var solve func(uint64) solution
	solve = func(remaining uint64) solution {
		if remaining == 0 {
			return solution{}
		}
		if cached, ok := memo[remaining]; ok {
			return cached
		}
		cardIndex := bits.TrailingZeros64(remaining)
		rest := remaining &^ (uint64(1) << cardIndex)
		bestRest := solve(rest)
		best := solution{
			meldPoints: bestRest.meldPoints,
			total:      bestRest.total - cards[cardIndex].Face - params.K,
			melds:      bestRest.melds,
		}
		for _, candidateIndex := range byCard[cardIndex] {
			candidate := candidates[candidateIndex]
			if candidate.mask&remaining != candidate.mask {
				continue
			}
			next := solve(remaining ^ candidate.mask)
			points := candidate.meld.Points + next.meldPoints
			total := candidate.meld.Points + next.total
			if points > best.meldPoints || points == best.meldPoints && total > best.total {
				best = solution{
					meldPoints: points,
					total:      total,
					melds:      append([]int{candidateIndex}, next.melds...),
				}
			}
		}
		memo[remaining] = best
		return best
	}

	allCards := (uint64(1) << len(cards)) - 1
	best := solve(allCards)
	partition := RummyPartition{
		Melds:      make([]RummyMeld, 0, len(best.melds)),
		Deadwood:   make([]Card, 0, len(cards)),
		MeldPoints: best.meldPoints,
		Total:      best.total,
	}
	used := uint64(0)
	for _, candidateIndex := range best.melds {
		candidate := candidates[candidateIndex]
		partition.Melds = append(partition.Melds, candidate.meld)
		used |= candidate.mask
	}
	for cardIndex, card := range cards {
		if used&(uint64(1)<<cardIndex) == 0 {
			partition.Deadwood = append(partition.Deadwood, card)
			partition.DeadwoodPenalty += card.Face + params.K
		}
	}
	return partition
}

func RummyHandQuality(hand []Card, params RummyParameters) float64 {
	partition := BestRummyPartition(hand, params)
	quality := float64(partition.MeldPoints - partition.DeadwoodPenalty)
	for _, card := range partition.Deadwood {
		for _, other := range partition.Deadwood {
			if card == other {
				continue
			}
			if card.Face == other.Face && card.Suit != other.Suit {
				quality += 0.5
			}
			if card.Suit == other.Suit && absInt(card.Face-other.Face) <= 2 {
				quality += 0.3
			}
		}
	}
	return quality
}

func ShouldTakeRummyDiscard(hand []Card, discard Card, params RummyParameters) bool {
	if !discard.Valid() {
		return false
	}
	before := BestRummyPartition(hand, params).MeldPoints
	afterHand := append(append([]Card(nil), hand...), discard)
	after := BestRummyPartition(afterHand, params).MeldPoints
	return after > before
}

func RummyMeldProximity(card Card, hand []Card) int {
	score := 0
	for _, other := range hand {
		if other == card {
			continue
		}
		switch {
		case other.Face == card.Face && other.Suit != card.Suit:
			score += 3
		case other.Suit == card.Suit && absInt(other.Face-card.Face) == 1:
			score += 2
		case other.Suit == card.Suit && absInt(other.Face-card.Face) == 2:
			score++
		}
	}
	return score
}

type rummyCandidate struct {
	mask uint64
	meld RummyMeld
}

func makeRummyCandidates(cards []Card, params RummyParameters) []rummyCandidate {
	candidates := make([]rummyCandidate, 0)
	add := func(indices []int, kind string, points int) {
		mask := uint64(0)
		meldCards := make([]Card, 0, len(indices))
		for _, index := range indices {
			mask |= uint64(1) << index
			meldCards = append(meldCards, cards[index])
		}
		candidates = append(candidates, rummyCandidate{
			mask: mask,
			meld: RummyMeld{Kind: kind, Cards: meldCards, Points: points},
		})
	}

	for first := 0; first < len(cards); first++ {
		for second := first + 1; second < len(cards); second++ {
			if cards[first].Face != cards[second].Face {
				continue
			}
			for third := second + 1; third < len(cards); third++ {
				if cards[third].Face == cards[first].Face &&
					cards[first].Suit != cards[second].Suit &&
					cards[first].Suit != cards[third].Suit &&
					cards[second].Suit != cards[third].Suit {
					add([]int{first, second, third}, "set",
						params.J+cards[first].Face+params.K)
				}
			}
		}
	}

	for first := 0; first < len(cards); first++ {
		if !cards[first].Valid() {
			continue
		}
		for last := first + 1; last < len(cards); last++ {
			if cards[last].Suit != cards[first].Suit {
				if cards[last].Suit > cards[first].Suit {
					break
				}
				continue
			}
			validRun := true
			for index := first + 1; index <= last; index++ {
				if cards[index].Suit != cards[first].Suit ||
					cards[index].Face != cards[index-1].Face+1 {
					validRun = false
					break
				}
			}
			if !validRun || last-first+1 < 3 {
				continue
			}
			indices := make([]int, 0, last-first+1)
			for index := first; index <= last; index++ {
				indices = append(indices, index)
			}
			length := last - first + 1
			add(indices, "run", cards[last].Face+params.K+length+params.B)
		}
	}
	return candidates
}

func scoreRummyHandWithoutMelds(cards []Card, params RummyParameters) RummyPartition {
	result := RummyPartition{Melds: []RummyMeld{}, Deadwood: append([]Card(nil), cards...)}
	for _, card := range cards {
		result.DeadwoodPenalty += card.Face + params.K
	}
	result.Total = -result.DeadwoodPenalty
	return result
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
