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
				got.Favored < -1 || got.Favored > 2 || got.Draft < 1 || got.Draft > 5 {
				t.Fatalf("%s + %s produced out-of-range modifiers: %+v", first.ID, second.ID, got)
			}

		}
	}
	if pairs != 66 {
		t.Fatalf("tested %d hero pairs, want 66", pairs)
	}
}

func TestHeroDraftModifierRange(t *testing.T) {
	for _, draft := range []int{-3, 3} {
		if err := Validate(Hero{ID: "test", Name: "Test", Draft: draft, Unlock: "default"}); err == nil {
			t.Errorf("hero Draft modifier %d should be outside the allowed range", draft)
		}
	}
}

func TestRenamedHeroesKeepTheirExistingIDs(t *testing.T) {
	want := map[string]string{
		"ascendant": "The Gorilla", "anchor": "The Turtle",
		"ascetic": "The Bureaucrat", "herald": "The Bodybuilder",
	}
	for id, name := range want {
		hero, ok := ByID(id)
		if !ok || hero.Name != name {
			t.Errorf("hero %q name = %q, want %q", id, hero.Name, name)
		}
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
	if got := DraftMode(recruit, recruit); got != 1 {
		t.Fatalf("Recruit vs Recruit draft mode = %d, want 1", got)
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
		{"hunter", "hater", CombinedModifiers{Bounty: -1, Favored: 1, Draft: 3}},
		{"hater", "warden", CombinedModifiers{Favored: 2, Draft: 3}},
		{"hater", "hater", CombinedModifiers{Bounty: -2, Favored: 2, Draft: 3}},
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
	got, err = CombineWithTuning(warden, ascendant, Tuning{"bounty": 1}, Tuning{"favored": 1})
	if err != nil {
		t.Fatal(err)
	}
	if got != (CombinedModifiers{Bounty: 2, Shift: 2, Favored: 2, Draft: 2}) {
		t.Fatalf("tuned modifiers = %+v", got)
	}

	got, err = CombineWithTuning(warden, ascendant, Tuning{"shift": 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Shift != 3 {
		t.Fatalf("Master Conductor shift = %d, want 3 after modifier clamp", got.Shift)
	}
}

func TestDraftModeAddsAndClampsHeroAttributes(t *testing.T) {
	tests := []struct {
		first, second int
		want          int
	}{
		{first: -2, second: -2, want: 1},
		{first: -2, second: 0, want: 1},
		{first: 0, second: 0, want: 3},
		{first: 1, second: 1, want: 5},
		{first: 2, second: 2, want: 5},
	}

	for _, test := range tests {
		first, second := Hero{Draft: test.first}, Hero{Draft: test.second}
		if got := DraftMode(first, second); got != test.want {
			t.Errorf("DraftMode(%d, %d) = %d, want %d", test.first, test.second, got, test.want)
		}
	}
}

func TestRosterDraftModifiersAndHaterCancellation(t *testing.T) {
	recruit, _ := ByID("recruit")
	gorilla, _ := ByID("ascendant")
	gambler, _ := ByID("gambler")
	diplomat, _ := ByID("diplomat")
	turtle, _ := ByID("anchor")
	hater, _ := ByID("hater")
	if got := DraftMode(recruit, gorilla); got != 1 {
		t.Fatalf("minimum roster draft mode = %d, want 1", got)
	}
	if got := DraftMode(gambler, diplomat); got != 5 {
		t.Fatalf("maximum roster draft mode = %d, want 5", got)
	}
	if got := DraftMode(hater, gambler); got != 2 {
		t.Fatalf("Hater versus +1 Draft modifier mode = %d, want 2", got)
	}
	if got := DraftMode(hater, hater); got != 3 {
		t.Fatalf("Hater mirror-match mode = %d, want 3", got)
	}
	if got := DraftMode(recruit, turtle); got != 3 {
		t.Fatalf("Recruit versus Turtle mode = %d, want 3", got)
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
	if err := ValidateTuning(Tuning{"bounty": 2}, 2); err != nil {
		t.Fatalf("Master Conductor tuning rejected: %v", err)
	}
	if err := ValidateTuning(Tuning{"bounty": 1, "shift": -1}, 2); err == nil {
		t.Fatal("tuning multiple modifiers should be rejected")
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
