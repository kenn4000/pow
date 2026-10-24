package app

import (
	"testing"
	"time"

	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
)

// Camp Draw pages carry the full chroma record for a chroma reward so the
// browser can render its artwork, and nothing for currency rewards.
func TestCampDrawPageIncludesRewardChroma(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	chroma := catalog.AllChromas()[0]
	web := &webServer{lastCampDraw: &lootbox.CampDrawResult{BoxType: "diamond", Reward: lootbox.BoxOpenResult{BoxType: "diamond", ChromaID: chroma.ID}}}
	page := web.campDrawPage(p, false, &lootbox.BoxOpenResult{BoxType: "gold", Marks: 500})
	if page.RewardChroma == nil || page.RewardChroma.ID != chroma.ID {
		t.Fatalf("RewardChroma = %+v, want %q", page.RewardChroma, chroma.ID)
	}
	if page.BoxChroma != nil {
		t.Fatalf("BoxChroma = %+v for a Marks reward, want nil", page.BoxChroma)
	}
	if chromaPointer("not-a-chroma") != nil || chromaPointer("") != nil {
		t.Fatal("unknown or empty chroma IDs must not produce a catalog entry")
	}
}
