package game

import "testing"

func TestNewRummyParametersClampsHeroModifiers(t *testing.T) {
	got := NewRummyParameters(5, -2, 8, -5)
	want := RummyParameters{J: 8, K: 0, B: 3, D: -1, HandSize: 5}
	if got != want {
		t.Fatalf("parameters = %+v, want %+v", got, want)
	}

	got = NewRummyParameters(-5, 8, -8, 5)
	want = RummyParameters{J: 4, K: 3, B: -2, D: 1, HandSize: 7}
	if got != want {
		t.Fatalf("parameters = %+v, want %+v", got, want)
	}
}

func TestBestRummyPartitionScoresSetsRunsAndDeadwood(t *testing.T) {
	params := RummyParameters{J: 6, K: 1, B: 2}
	hand := []Card{
		{Suit: Blade, Face: 2}, {Suit: Blade, Face: 3}, {Suit: Blade, Face: 4},
		{Suit: Blade, Face: 7}, {Suit: Fang, Face: 7}, {Suit: Shield, Face: 7},
		{Suit: Shield, Face: 9},
	}

	got := BestRummyPartition(hand, params)
	if len(got.Melds) != 2 || got.MeldPoints != 24 {
		t.Fatalf("partition melds=%+v points=%d, want two melds worth 24",
			got.Melds, got.MeldPoints)
	}
	if got.DeadwoodPenalty != 10 || got.Total != 14 || len(got.Deadwood) != 1 ||
		got.Deadwood[0] != (Card{Suit: Shield, Face: 9}) {
		t.Fatalf("partition deadwood=%v penalty=%d total=%d, want Shield 9, 10, 14",
			got.Deadwood, got.DeadwoodPenalty, got.Total)
	}
}

func TestShouldTakeRummyDiscardAndMeldProximity(t *testing.T) {
	params := RummyParameters{J: 6}
	hand := []Card{{Suit: Blade, Face: 2}, {Suit: Blade, Face: 3}}
	if !ShouldTakeRummyDiscard(hand, Card{Suit: Blade, Face: 4}, params) {
		t.Fatal("should take a discard that completes a run")
	}
	if ShouldTakeRummyDiscard(hand, Card{Suit: Shield, Face: 9}, params) {
		t.Fatal("should not take a discard that adds no meld points")
	}

	proximity := RummyMeldProximity(Card{Suit: Blade, Face: 4}, []Card{
		{Suit: Blade, Face: 2}, {Suit: Blade, Face: 3}, {Suit: Fang, Face: 4},
	})
	if proximity != 6 {
		t.Fatalf("meld proximity = %d, want 6", proximity)
	}
}

func TestRummyHandQualityIncludesDeadwoodPotential(t *testing.T) {
	hand := []Card{
		{Suit: Blade, Face: 1},
		{Suit: Blade, Face: 3},
		{Suit: Fang, Face: 1},
	}
	if got := RummyHandQuality(hand, RummyParameters{}); got < -3.41 || got > -3.39 {
		t.Fatalf("hand quality = %.1f, want -3.4", got)
	}
}
