package app

import (
	"testing"

	"prisonersofwar/internal/catalog"
)

func TestSuccessMessageNamesUsePlayerFacingLabels(t *testing.T) {
	if got := skillName("quartermaster"); got != "Quartermaster" {
		t.Fatalf("skillName(quartermaster) = %q", got)
	}
	for code, want := range map[string]string{"B": "Scissors", "F": "Paper", "S": "Rock", "x": "x"} {
		if got := suitDisplayName(code); got != want {
			t.Fatalf("suitDisplayName(%q) = %q, want %q", code, got, want)
		}
	}
	chroma := catalog.AllChromas()[0]
	if got := chromaName(chroma.ID); got != chroma.Name {
		t.Fatalf("chromaName(%q) = %q, want %q", chroma.ID, got, chroma.Name)
	}
	if bundles := catalog.AllBundles(); len(bundles) > 0 && bundleName(bundles[0].ID) != bundles[0].Name {
		t.Fatalf("bundleName(%q) = %q", bundles[0].ID, bundleName(bundles[0].ID))
	}
	if got := skillName("unknown-node"); got != "unknown-node" {
		t.Fatalf("unknown IDs must fall back to themselves, got %q", got)
	}
}
