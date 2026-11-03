package game

import "testing"

func TestEvaluatePokerHandTypes(t *testing.T) {
	tests := []struct {
		name string
		hand []Card
		want PokerHandType
	}{
		{"straight flush", []Card{{Blade, 1}, {Blade, 2}, {Blade, 3}, {Blade, 4}, {Blade, 5}}, PokerStraightFlush},
		{"three of a kind", []Card{{Blade, 7}, {Fang, 7}, {Shield, 7}, {Blade, 2}, {Fang, 4}}, PokerThreeOfAKind},
		{"full house uses three of a kind score", []Card{{Blade, 7}, {Fang, 7}, {Shield, 7}, {Blade, 2}, {Fang, 2}}, PokerThreeOfAKind},
		{"straight", []Card{{Blade, 1}, {Fang, 2}, {Shield, 3}, {Blade, 4}, {Fang, 5}}, PokerStraight},
		{"flush", []Card{{Blade, 1}, {Blade, 3}, {Blade, 5}, {Blade, 7}, {Blade, 9}}, PokerFlush},
		{"two pair", []Card{{Blade, 1}, {Fang, 1}, {Shield, 3}, {Blade, 3}, {Fang, 7}}, PokerPair},
		{"high card", []Card{{Blade, 1}, {Fang, 3}, {Shield, 5}, {Blade, 7}, {Fang, 9}}, PokerHighCard},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := evaluatePokerHand(test.hand)
			if got != test.want {
				t.Fatalf("evaluatePokerHand() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestScorePokerTableBonusesAndFullTable(t *testing.T) {
	hands := [3][]Card{
		{{Blade, 1}, {Fang, 1}, {Shield, 7}, {Blade, 8}, {Fang, 9}},
		{{Blade, 2}, {Fang, 3}, {Shield, 4}, {Blade, 5}, {Fang, 6}},
		{{Shield, 1}, {Shield, 2}, {Shield, 3}, {Shield, 5}, {Shield, 6}},
	}
	score, err := ScorePokerTable(hands, PokerHeroParameters{Bounty: 1}, Blade, PokerMissionRules{
		NineBase: 12, BountyBase: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !score.ConstraintPassed || score.Total != 141 || score.TableBonus != 15 {
		t.Fatalf("table score = %+v", score)
	}
	if score.Hands[0].Base != 20 || score.Hands[0].NineBonus != 12 ||
		score.Hands[0].BountyBonus != 2 || score.Hands[0].Total != 34 {
		t.Fatalf("first hand score = %+v", score.Hands[0])
	}
}

func TestScorePokerTableAwardsOrderBonusesOnlyForCompleteHands(t *testing.T) {
	hands := [3][]Card{
		{{Blade, 7}, {Fang, 7}, {Shield, 7}, {Blade, 1}, {Fang, 2}},
		{{Blade, 4}, {Fang, 4}, {Shield, 1}, {Blade, 2}, {Fang, 3}},
		{{Shield, 5}, {Blade, 6}, {Fang, 8}, {Shield, 2}, {Blade, 3}},
	}
	score, err := ScorePokerTable(hands, PokerHeroParameters{}, Fang, PokerMissionRules{
		NineBase: 12, BountyBase: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.HandAHighBonus != 15 || score.HandCLowBonus != 5 ||
		score.Total != score.Hands[0].Total+score.Hands[1].Total+score.Hands[2].Total+20 {
		t.Fatalf("complete-hand order bonuses = %+v", score)
	}

	hands[2] = nil
	score, err = ScorePokerTable(hands, PokerHeroParameters{}, Fang, PokerMissionRules{
		NineBase: 12, BountyBase: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.HandAHighBonus != 0 || score.HandCLowBonus != 0 {
		t.Fatalf("void hand received order bonuses: %+v", score)
	}
}

func TestPokerHandPlacementKeepsHandSizesWithinOne(t *testing.T) {
	hands := [3][]Card{
		make([]Card, 1), make([]Card, 2), make([]Card, 3),
	}
	if PokerHandsBalanced(hands) || CanPlacePokerCard(hands, 0) ||
		CanPlacePokerCard(hands, 1) || CanPlacePokerCard(hands, 2) {
		t.Fatal("imbalanced hands should not be considered legal")
	}

	hands = [3][]Card{make([]Card, 1), make([]Card, 1), make([]Card, 2)}
	if !PokerHandsBalanced(hands) || !CanPlacePokerCard(hands, 0) ||
		!CanPlacePokerCard(hands, 1) || CanPlacePokerCard(hands, 2) {
		t.Fatal("placement legality does not preserve the one-card balance rule")
	}
}

func TestScorePokerTableAppliesConstraintBeforeBonuses(t *testing.T) {
	hands := [3][]Card{
		{{Blade, 1}, {Blade, 2}, {Blade, 3}, {Blade, 4}, {Blade, 5}},
		nil,
		nil,
	}
	score, err := ScorePokerTable(hands, PokerHeroParameters{}, Shield, PokerMissionRules{
		NineBase: 12, BountyBase: 1, Constraint: PokerNoStraightsOrFlushes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Hands[0].Base != 0 || !score.Hands[0].BaseSuppressed ||
		score.Hands[0].BustPenalty != -10 || score.Total != -10 {
		t.Fatalf("constrained table score = %+v", score)
	}
}

func TestPokerConstraintFailureDoesNotReturnScores(t *testing.T) {
	score, err := ScorePokerTable([3][]Card{}, PokerHeroParameters{}, Blade, PokerMissionRules{
		NineBase: 12, BountyBase: 1, Constraint: PokerNineInEveryHand,
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.ConstraintPassed || score.ConstraintFailure == "" || score.Total != 0 ||
		score.Hands != [3]PokerHandScore{} {
		t.Fatalf("constraint failure exposed score data: %+v", score)
	}
}
