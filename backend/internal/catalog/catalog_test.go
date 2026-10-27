package catalog

import (
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestShopPurchasesBundlesAndColorAssignments(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	p.Crowns = 100
	p.Marks = 250
	if err := BuyBundle(p, "woodland"); err != nil {
		t.Fatal(err)
	}
	remaining := p.Crowns
	if !p.OwnedChromas["emerald"] || !p.OwnedChromas["walnut"] {
		t.Fatal("bundle did not grant its chromas")
	}
	if p.TotalChromasPurchased != 2 || p.RareChromasPurchased != 1 {
		t.Fatalf("bundle purchase counters = total %d, rare %d",
			p.TotalChromasPurchased, p.RareChromasPurchased)
	}
	if err := BuyBundle(p, "woodland"); err != nil {
		t.Fatalf("repeated bundle purchase should be idempotent: %v", err)
	}
	if p.Crowns != remaining {
		t.Fatalf("repeated bundle purchase charged Crowns: %d -> %d", remaining, p.Crowns)
	}
	if err := AssignChroma(p, "B", "emerald"); err != nil {
		t.Fatal(err)
	}
	if err := AssignChroma(p, "F", "emerald"); err == nil {
		t.Fatal("same color should not be assigned to multiple suits")
	}
	p.Marks = 100
	if err := BuyChroma(p, "conq-pink"); err != nil {
		t.Fatal(err)
	}
	if p.Marks != 0 {
		t.Fatalf("Conquistador purchase left %d Marks, want 0", p.Marks)
	}
	if err := BuyChroma(p, "conq-pink"); err == nil {
		t.Fatal("already-owned chroma should not be charged twice")
	}
	if err := MarkChromaSeen(p, "conq-pink"); err != nil {
		t.Fatal(err)
	}
	if !p.SeenChromas["conq-pink"] {
		t.Fatal("viewing the catalog did not mark the owned chroma as seen")
	}
}

func TestTribalAndConquistadorChromaNames(t *testing.T) {
	want := map[string]string{
		"crimson": "Tribal (red)", "cobalt": "Tribal (blue)", "sunflower": "Tribal (yellow)",
		"emerald": "Tribal (green)", "tangerine": "Tribal (orange)", "amethyst": "Tribal (purple)",
		"conq-lime": "Conquistador (lime green)", "conq-gold": "Conquistador (gold)",
		"conq-pink": "Conquistador (pink)", "conq-blue": "Conquistador (sky blue)",
	}
	for id, name := range want {
		chroma, ok := FindChroma(id)
		if !ok || chroma.Name != name {
			t.Errorf("chroma %q name = %q, want %q", id, chroma.Name, name)
		}
	}
}

func TestConquistadorChromasCost100Marks(t *testing.T) {
	for _, id := range []string{"conq-lime", "conq-gold", "conq-pink", "conq-blue"} {
		chroma, ok := FindChroma(id)
		if !ok || chroma.Cost != 100 || chroma.Currency != string(Marks) {
			t.Errorf("chroma %q = %+v, want 100 Marks", id, chroma)
		}
	}
}

func TestNewSkinChromasUseTheirFolderColorsAndArePurchasable(t *testing.T) {
	want := map[string]struct {
		name  string
		color string
		skin  string
	}{
		"alien-yellow": {"Alien (yellow)", "yellow", "shop/alien/yellow"},
		"amazon-green": {"Amazon (green)", "green", "shop/amazon/green"},
		"bee-silver":   {"Bee (silver)", "silver", "shop/bee/silver"},
		"cat-brown":    {"Cat (brown)", "brown", "shop/cat/brown"},
		"dog-gray":     {"Dog (gray)", "gray", "shop/dog/gray"},
		"dragon-ruby":  {"Dragon (ruby)", "ruby", "shop/dragon/ruby"},
		"dwarf-purple": {"Dwarf (purple)", "purple", "shop/dwarf/purple"},
		"orc-white":    {"Orc (white)", "white", "shop/orc/white"},
		"reptile-pink": {"Reptile (pink)", "pink", "shop/reptile/pink"},
		"zulu-orange":  {"Zulu (orange)", "orange", "shop/zulu/orange"},
	}
	for id, expected := range want {
		chroma, ok := FindChroma(id)
		if !ok {
			t.Errorf("skin chroma %q is missing from the catalog", id)
			continue
		}
		if chroma.Name != expected.name || chroma.Color != expected.color || chroma.Skin != expected.skin || chroma.HexColor == "" {
			t.Errorf("chroma %q = %+v, want name %q, color %q, skin %q",
				id, chroma, expected.name, expected.color, expected.skin)
		}
		if chroma.Cost != 250 || chroma.Currency != string(Marks) {
			t.Errorf("chroma %q costs %d %s, want 250 Marks", id, chroma.Cost, chroma.Currency)
		}
	}

	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 250
	if err := BuyChroma(p, "bee-silver"); err != nil {
		t.Fatalf("buy new skin chroma: %v", err)
	}
	if p.Marks != 0 || !p.OwnedChromas["bee-silver"] {
		t.Fatalf("skin chroma purchase left %d Marks and owned=%t", p.Marks, p.OwnedChromas["bee-silver"])
	}
	if err := AssignChroma(p, "B", "bee-silver"); err != nil {
		t.Fatalf("assign new skin chroma: %v", err)
	}
	p.OwnedChromas["silver"] = true
	if err := AssignChroma(p, "F", "silver"); err == nil {
		t.Fatal("chromas with the same folder color should not be assigned to different suits")
	}
}

func TestRedeemChromaPickConsumesTokenAndGrantsUnownedChroma(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := RedeemChromaPick(p, "aurora"); err == nil {
		t.Fatal("redemption without a token should fail")
	}
	p.ChromaPickTokens = 1
	if err := RedeemChromaPick(p, "not-a-chroma"); err == nil {
		t.Fatal("unknown chroma should fail")
	}
	if p.ChromaPickTokens != 1 {
		t.Fatal("invalid chroma consumed the token")
	}
	if err := RedeemChromaPick(p, "aurora"); err != nil {
		t.Fatal(err)
	}
	if p.ChromaPickTokens != 0 || !p.OwnedChromas["aurora"] {
		t.Fatalf("redemption state = token %d, owned %t", p.ChromaPickTokens, p.OwnedChromas["aurora"])
	}
	p.ChromaPickTokens = 1
	if err := RedeemChromaPick(p, "aurora"); err == nil || p.ChromaPickTokens != 1 {
		t.Fatal("duplicate chroma redemption should fail without consuming a token")
	}
}

func TestSeasonalChromaIsGrantedButNotDirectlyPurchasable(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := BuyChroma(p, "ember-dog"); err == nil {
		t.Fatal("seasonal chroma should not be directly purchasable")
	}
	p.ChromaPickTokens = 1
	if err := RedeemChromaPick(p, "ember-dog"); err != nil {
		t.Fatal(err)
	}
	if !p.OwnedChromas["ember-dog"] || p.ChromaPickTokens != 0 {
		t.Fatal("Pick Any Chroma did not grant the seasonal chroma")
	}
}
