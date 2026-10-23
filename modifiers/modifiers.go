package modifiers

import (
	"fmt"
)

type GameModifiers struct {
	Bounty    int    `json:"bounty"`
	Shift     int    `json:"shift"`
	Favored   int    `json:"favored"`
	Reckoning [3]int `json:"reckoning"`
}

var DefaultReckoning = [3]int{1, 1, 1}

func (m GameModifiers) Validate() error {
	if m.Bounty < -3 || m.Bounty > 3 {
		return fmt.Errorf("bounty modifier %d is outside -3..3", m.Bounty)
	}
	if m.Shift < 0 || m.Shift > 3 {
		return fmt.Errorf("shift modifier %d is outside 0..3", m.Shift)
	}
	if m.Favored < -1 || m.Favored > 2 {
		return fmt.Errorf("favored modifier %d is outside -1..2", m.Favored)
	}
	switch m.Reckoning {
	case [3]int{1, 1, 1}, [3]int{1, 1, 2}, [3]int{1, 1, 3}, [3]int{1, 2, 3}:
		return nil
	default:
		return fmt.Errorf("invalid reckoning weights %v", m.Reckoning)
	}
}

func CombatValue(face int, beats bool, shift, favored int) int {
	base := face + shift
	if base < 1 {
		base = 1
	}
	if beats {
		base = base*2 + favored
		if base < 1 {
			return 1
		}
	}
	return base
}

func ScoringFace(face int, isBountySuit bool, mods GameModifiers) int {
	value := face + mods.Shift
	if isBountySuit {
		value += mods.Bounty
	}
	if value < 1 {
		return 1
	}
	return value
}
