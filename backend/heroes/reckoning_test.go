package heroes

import (
	"math/rand"
	"testing"
)

func TestRollReckoningDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(731))
	counts := make(map[[3]int]int)
	const samples = 10000
	for i := 0; i < samples; i++ {
		result, err := RollReckoning(rng)
		if err != nil {
			t.Fatal(err)
		}
		counts[result.Weights]++
	}
	for weights, expectedPercent := range map[[3]int]float64{
		{1, 1, 1}: 40,
		{1, 1, 2}: 25,
		{1, 1, 3}: 20,
		{1, 2, 3}: 15,
	} {
		actual := float64(counts[weights]) / samples * 100
		if actual < expectedPercent-2 || actual > expectedPercent+2 {
			t.Errorf("weights %v appeared %.2f%% of the time, want %.0f%% ±2%%", weights, actual, expectedPercent)
		}
	}
}
