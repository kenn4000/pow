package heroes

import (
	"errors"
	"math/rand"
)

type Reckoning struct {
	Weights [3]int
}

var reckoningDistribution = []struct {
	weights [3]int
	weight  int
}{
	{weights: [3]int{1, 1, 1}, weight: 40},
	{weights: [3]int{1, 1, 2}, weight: 25},
	{weights: [3]int{1, 1, 3}, weight: 20},
	{weights: [3]int{1, 2, 3}, weight: 15},
}

func RollReckoning(rng *rand.Rand) (Reckoning, error) {
	if rng == nil {
		return Reckoning{}, errors.New("random source is required")
	}
	roll := rng.Intn(100)
	for _, outcome := range reckoningDistribution {
		if roll < outcome.weight {
			return Reckoning{Weights: outcome.weights}, nil
		}
		roll -= outcome.weight
	}
	return Reckoning{}, errors.New("reckoning distribution is invalid")
}
