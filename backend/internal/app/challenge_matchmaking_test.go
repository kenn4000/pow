package app

import "testing"

func TestValidChallengePreference(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{value: "main:online", valid: true},
		{value: "puzzle-battle:ranked", valid: true},
		{value: "unknown:online"},
		{value: "main:friend"},
		{value: "main"},
		{value: "main:online:extra"},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := validChallengePreference(test.value); got != test.valid {
				t.Fatalf("validChallengePreference(%q) = %t, want %t", test.value, got, test.valid)
			}
		})
	}
}
