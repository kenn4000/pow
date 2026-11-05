package ranked

import "testing"

func TestSeriesEndsAtTwoWinsAndTotalsScores(t *testing.T) {
	series := NewSeries()
	if err := series.RecordGame(0, [2]int{12, 8}); err != nil {
		t.Fatal(err)
	}
	if err := series.RecordGame(1, [2]int{7, 11}); err != nil {
		t.Fatal(err)
	}
	if err := series.RecordGame(0, [2]int{14, 9}); err != nil {
		t.Fatal(err)
	}
	if !series.Complete || series.Winner != 0 || series.GamesPlayed != 3 ||
		series.Wins != [2]int{2, 1} || series.Scores != [2]int{33, 28} {
		t.Fatalf("series = %+v", series)
	}
	if err := series.RecordGame(1, [2]int{1, 1}); err == nil {
		t.Fatal("completed series accepted another game")
	}
}

func TestSeriesCanEndEarlyAndTiesAfterThree(t *testing.T) {
	early := NewSeries()
	for _, winner := range []int{1, 1} {
		if err := early.RecordGame(winner, [2]int{10, 10}); err != nil {
			t.Fatal(err)
		}
	}
	if !early.Complete || early.GamesPlayed != 2 || early.Winner != 1 {
		t.Fatalf("early series = %+v", early)
	}

	tied := NewSeries()
	for _, winner := range []int{0, 1, -1} {
		if err := tied.RecordGame(winner, [2]int{10, 10}); err != nil {
			t.Fatal(err)
		}
	}
	if !tied.Complete || !tied.Draw || tied.Winner != -1 {
		t.Fatalf("tied series = %+v", tied)
	}
}

func TestUpdatedRatingsUsesStandardKFactor(t *testing.T) {
	a, b, err := UpdatedRatings(1000, 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a != 1016 || b != 984 {
		t.Fatalf("ratings after win = %d, %d; want 1016, 984", a, b)
	}
	a, b, err = UpdatedRatings(a, b, -1)
	if err != nil {
		t.Fatal(err)
	}
	if a != 1015 || b != 985 {
		t.Fatalf("ratings after standard Elo draw = %d, %d; want 1015, 985", a, b)
	}
}
