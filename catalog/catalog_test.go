package catalog

import (
	"testing"
	"time"

	"prisonersofwar/profile"
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
	if err := BuyChroma(p, "crimson"); err != nil {
		t.Fatal(err)
	}
	if err := BuyChroma(p, "crimson"); err == nil {
		t.Fatal("already-owned chroma should not be charged twice")
	}
	if err := MarkChromaSeen(p, "crimson"); err != nil {
		t.Fatal(err)
	}
	if !p.SeenChromas["crimson"] {
		t.Fatal("viewing the catalog did not mark the owned chroma as seen")
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
