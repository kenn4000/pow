package modifiers

import "testing"

func TestEffectiveCombatValue(t *testing.T) {
	tests := []struct {
		name    string
		face    int
		beats   bool
		shift   int
		favored int
		want    int
	}{
		{"shifted cycle win", 5, true, 2, 0, 14},
		{"negative favored cycle win", 5, true, 2, -1, 13},
		{"positive favored cycle win", 5, true, 1, 2, 14},
		{"non-cycle uses shifted face", 5, false, 2, 2, 7},
		{"cycle floor", 1, true, 0, -1, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CombatValue(test.face, test.beats, test.shift, test.favored); got != test.want {
				t.Fatalf("CombatValue() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestScoringFaceUsesBountyAndShift(t *testing.T) {
	tests := []struct {
		name   string
		face   int
		mods   GameModifiers
		bounty bool
		want   int
	}{
		{"positive bounty", 7, GameModifiers{Bounty: 2, Reckoning: [3]int{1, 1, 1}}, true, 9},
		{"negative bounty", 7, GameModifiers{Bounty: -1, Reckoning: [3]int{1, 1, 1}}, true, 6},
		{"shift", 7, GameModifiers{Shift: 2, Reckoning: [3]int{1, 1, 1}}, false, 9},
		{"floor", 1, GameModifiers{Bounty: -3, Reckoning: [3]int{1, 1, 1}}, true, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ScoringFace(test.face, test.bounty, test.mods); got != test.want {
				t.Fatalf("ScoringFace() = %d, want %d", got, test.want)
			}
		})
	}
}
