package ui

import (
	"bytes"
	"strings"
	"testing"

	"prisonersofwar/internal/modifiers"
)

func TestRenderModifierHUD(t *testing.T) {
	var output bytes.Buffer
	mods := modifiers.GameModifiers{Bounty: 2, Shift: 1, Favored: -1, Reckoning: [3]int{1, 1, 2}}
	if err := RenderModifierHUD(&output, mods, "Rock", false); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Bounty: +2 (Rock cards score +2 extra)", "Shift: +1", "Favored: -1", "Round weights: 1 / 1 / 2", "Bounty suit: Rock"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("HUD missing %q:\n%s", expected, output.String())
		}
	}
}

func TestRenderModifierHUDHidesNeutralModifiers(t *testing.T) {
	var output bytes.Buffer
	mods := modifiers.GameModifiers{Reckoning: [3]int{1, 1, 1}}
	if err := RenderModifierHUD(&output, mods, "Scissors", false); err != nil {
		t.Fatal(err)
	}
	for _, unexpected := range []string{"Bounty:", "Shift:", "Favored:"} {
		if strings.Contains(output.String(), unexpected) {
			t.Errorf("neutral HUD unexpectedly includes %q", unexpected)
		}
	}
	if !strings.Contains(output.String(), "Round weights: even (no reckoning)") {
		t.Fatalf("HUD missing even-weight explanation: %s", output.String())
	}
}
