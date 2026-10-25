package app

import "testing"

func TestValidSocialHandle(t *testing.T) {
	tests := []struct {
		handle string
		valid  bool
	}{
		{"Ace", true},
		{"Player_007", true},
		{"a12", true},
		{"ab", false},
		{"1player", false},
		{"player-name", false},
		{"has space", false},
		{"way_too_long_handle_name", false},
	}
	for _, test := range tests {
		t.Run(test.handle, func(t *testing.T) {
			if got := validSocialHandle(test.handle); got != test.valid {
				t.Errorf("validSocialHandle(%q) = %t, want %t", test.handle, got, test.valid)
			}
		})
	}
}
