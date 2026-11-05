package ranked

import (
	"errors"
	"math"
)

const (
	SeriesLength   = 3
	WinsToEnd      = 2
	StartingRating = 1000
	KFactor        = 32
)

type Series struct {
	GamesPlayed int    `json:"gamesPlayed"`
	Wins        [2]int `json:"wins"`
	Scores      [2]int `json:"scores"`
	Winner      int    `json:"winner"`
	Draw        bool   `json:"draw"`
	Complete    bool   `json:"complete"`
}

func NewSeries() *Series {
	return &Series{Winner: -1}
}

func (s *Series) RecordGame(winner int, scores [2]int) error {
	if s == nil {
		return errors.New("ranked series is required")
	}
	if s.Complete || s.GamesPlayed >= SeriesLength {
		return errors.New("ranked series is already complete")
	}
	if winner < -1 || winner > 1 {
		return errors.New("ranked series winner must be -1, 0, or 1")
	}
	if scores[0] < 0 || scores[1] < 0 {
		return errors.New("ranked series scores cannot be negative")
	}
	s.GamesPlayed++
	s.Scores[0] += scores[0]
	s.Scores[1] += scores[1]
	if winner >= 0 {
		s.Wins[winner]++
	}
	if s.Wins[0] < WinsToEnd && s.Wins[1] < WinsToEnd && s.GamesPlayed < SeriesLength {
		return nil
	}
	s.Complete = true
	switch {
	case s.Wins[0] > s.Wins[1]:
		s.Winner = 0
	case s.Wins[1] > s.Wins[0]:
		s.Winner = 1
	default:
		s.Winner = -1
		s.Draw = true
	}
	return nil
}

func UpdatedRatings(ratingA, ratingB, winner int) (int, int, error) {
	if ratingA < 0 || ratingB < 0 {
		return 0, 0, errors.New("ratings cannot be negative")
	}
	if winner < -1 || winner > 1 {
		return 0, 0, errors.New("winner must be -1, 0, or 1")
	}
	actualA := 0.5
	if winner == 0 {
		actualA = 1
	} else if winner == 1 {
		actualA = 0
	}
	expectedA := 1 / (1 + math.Pow(10, float64(ratingB-ratingA)/400))
	delta := int(math.Round(KFactor * (actualA - expectedA)))
	return ratingA + delta, ratingB - delta, nil
}
