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
