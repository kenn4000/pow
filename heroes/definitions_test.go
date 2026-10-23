package heroes

import (
	"math/rand"
	"testing"
)

func TestCombineAllPairsStayWithinEnvelope(t *testing.T) {
	all := All()
	pairs := 0
	for i, first := range all {
		for _, second := range all[i:] {
			pairs++
			got := Combine(first, second)
			if got.Bounty < -3 || got.Bounty > 3 || got.Shift < 0 || got.Shift > 3 ||
				got.Favored < -1 || got.Favored > 2 {
				t.Fatalf("%s + %s produced out-of-range modifiers: %+v", first.ID, second.ID, got)
			}
		}
	}
	if pairs != 66 {
		t.Fatalf("tested %d hero pairs, want 66", pairs)
	}
}

func TestRecruitMirrorMatchDefaultsToPositiveBounty(t *testing.T) {
	recruit, ok := ByID("recruit")
	if !ok {
		t.Fatal("Recruit hero is missing")
	}
	if got := Combine(recruit, recruit).Bounty; got != 1 {
		t.Fatalf("Recruit vs Recruit bounty = %d, want +1", got)
	}
	mods, err := GameModifiersFor(recruit, recruit, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if mods.Bounty != 1 {
		t.Fatalf("Recruit vs Recruit game modifier bounty = %d, want +1", mods.Bounty)
	}
}

func TestHaterFlipsOnlyOpponentBounty(t *testing.T) {
	tests := []struct {
		first, second string
		want          CombinedModifiers
	}{
		{"hunter", "hater", CombinedModifiers{Bounty: -1, Favored: 1}},
		{"hater", "warden", CombinedModifiers{Favored: 2}},
		{"hater", "hater", CombinedModifiers{Bounty: -2, Favored: 2}},
	}
	for _, test := range tests {
		first, _ := ByID(test.first)
		second, _ := ByID(test.second)
		if got := Combine(first, second); got != test.want {
			t.Errorf("Combine(%s, %s) = %+v, want %+v", test.first, test.second, got, test.want)
		}
	}
}

func TestCombineWithTuningAppliesBeforeClamp(t *testing.T) {
	herald, _ := ByID("herald")
	hunter, _ := ByID("hunter")
	got, err := CombineWithTuning(herald, hunter, Tuning{"bounty": -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounty != 3 {
		t.Fatalf("tuned bounty = %d, want 3 after sum-then-clamp", got.Bounty)
	}

	warden, _ := ByID("warden")
	ascendant, _ := ByID("ascendant")
	got, err = CombineWithTuning(warden, ascendant, Tuning{
		"bounty": 1, "shift": 1,
	}, Tuning{"favored": 1})
	if err != nil {
		t.Fatal(err)
	}
	if got != (CombinedModifiers{Bounty: 2, Shift: 3, Favored: 2}) {
		t.Fatalf("tuned modifiers = %+v", got)
	}
}

func TestValidateTuningLimitsAxesAndValues(t *testing.T) {
	for _, tuning := range []Tuning{
		{"unknown": 1},
		{"bounty": 2},
		{"bounty": 1, "shift": -1},
	} {
		if err := ValidateTuning(tuning, 1); err == nil {
			t.Errorf("invalid tuning accepted: %v", tuning)
		}
	}
	if err := ValidateTuning(Tuning{"bounty": 1, "shift": -1}, 2); err != nil {
		t.Fatalf("Conductor II tuning rejected: %v", err)
	}
}

func TestChooseForOpponentUsesUnlockedCounterPick(t *testing.T) {
	chosen, err := ChooseForOpponent([]string{"warden", "hunter", "treasurer"}, "hunter", rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if chosen.ID != "treasurer" {
		t.Fatalf("counter-pick = %q, want treasurer", chosen.ID)
	}
}

func TestValidateRejectsMalformedUnlockCost(t *testing.T) {
	hero := Hero{ID: "test", Name: "Test", Unlock: "marks:not-a-number"}
	if err := Validate(hero); err == nil {
		t.Fatal("malformed Marks unlock cost was accepted")
	}
}
