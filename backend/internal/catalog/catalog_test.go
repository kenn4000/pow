package catalog

import (
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestShopPurchasesChromasAndColorAssignments(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	p.Marks = 250
	if err := BuyChroma(p, "emerald"); err != nil {
		t.Fatal(err)
	}
	if p.Marks != 0 || !p.OwnedChromas["emerald"] || p.TotalChromasPurchased != 1 {
		t.Fatalf("chroma purchase left %d Marks, owned=%t, total purchases=%d",
			p.Marks, p.OwnedChromas["emerald"], p.TotalChromasPurchased)
	}
	if err := AssignChroma(p, "B", "emerald"); err != nil {
		t.Fatal(err)
	}
	paperAppearance := p.AssignedChromas["F"]
	if err := AssignChroma(p, "F", "emerald"); err != nil {
		t.Fatalf("assign existing color to another suit: %v", err)
	}
	if p.AssignedChromas["F"] != "emerald" || p.AssignedChromas["B"] != paperAppearance {
		t.Fatalf("duplicate color assignment did not swap appearances: %+v", p.AssignedChromas)
	}
	p.Marks = 600
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

func TestConquistadorChromasCost600Marks(t *testing.T) {
	for _, id := range []string{"conq-lime", "conq-gold", "conq-pink", "conq-blue", "conq-skyblue"} {
		chroma, ok := FindChroma(id)
		if !ok || chroma.Cost != 600 || chroma.Currency != string(Marks) {
			t.Errorf("chroma %q = %+v, want 600 Marks", id, chroma)
		}
	}
}

func TestTribalAndDragonFamilyPricesUseOneCurrency(t *testing.T) {
	for _, id := range []string{
		"cobalt", "crimson", "sunflower", "emerald", "tangerine", "amethyst",
	} {
		chroma, ok := FindChroma(id)
		if !ok || chroma.Cost != 250 || chroma.Currency != string(Marks) {
			t.Errorf("Tribal chroma %q = %+v, want 250 Marks", id, chroma)
		}
	}
	for _, id := range []string{"dragon-emerald", "dragon-ruby", "dragon-sapphire"} {
		chroma, ok := FindChroma(id)
		if !ok || chroma.Cost != 800 || chroma.Currency != string(Crowns) {
			t.Errorf("Dragon chroma %q = %+v, want 800 Crowns", id, chroma)
		}
	}
}

func TestChromaFamilyPricesUseHighestMemberPrice(t *testing.T) {
	byFamily := make(map[string][]Chroma)
	for _, chroma := range AllChromas() {
		family := chromaFamilyKey(chroma)
		byFamily[family] = append(byFamily[family], chroma)
	}

	for family, chromas := range byFamily {
		if len(chromas) < 2 {
			continue
		}
		highestPrice := chromas[0].Cost
		currency := chromas[0].Currency
		for _, chroma := range chromas[1:] {
			if chroma.Cost > highestPrice {
				highestPrice = chroma.Cost
			}
			if chroma.Currency != currency {
				t.Errorf("family %q mixes currencies: %s and %s", family, currency, chroma.Currency)
			}
		}
		for _, chroma := range chromas {
			if chroma.Cost != highestPrice {
				t.Errorf("family %q chroma %q costs %d; want family maximum %d",
					family, chroma.ID, chroma.Cost, highestPrice)
			}
		}
	}

	for family, want := range map[string]int64{
		"alien": 1200, "amazon": 1200, "bee": 1200, "cat": 900,
		"conq": 600, "dog": 600, "dwarf": 1000,
	} {
		chromas := byFamily[family]
		if len(chromas) < 2 {
			t.Errorf("family %q has only %d chromas", family, len(chromas))
			continue
		}
		for _, chroma := range chromas {
			if chroma.Cost != want {
				t.Errorf("family %q chroma %q costs %d, want %d", family, chroma.ID, chroma.Cost, want)
			}
		}
	}
}

func TestBuyChromaFamilyDiscountsFullSetPrice(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var fullPrice int64
	var familyChromas []Chroma
	for _, chroma := range AllChromas() {
		if chromaFamilyKey(chroma) == "alien" {
			fullPrice += chroma.Cost
			familyChromas = append(familyChromas, chroma)
		}
	}
	if len(familyChromas) != 3 || fullPrice != 3600 {
		t.Fatalf("Alien family has %d chromas at %d Marks; want 3 at 3600 Marks total",
			len(familyChromas), fullPrice)
	}
	wantCost := (fullPrice*4 + 2) / 5
	p.Marks = wantCost - 1
	if _, _, err := BuyChromaFamily(p, "alien"); err == nil {
		t.Fatal("family purchase should fail when the discounted price is unaffordable")
	}
	if p.Marks != wantCost-1 || p.TotalChromasPurchased != 0 ||
		len(p.OwnedChromas) != 3 || !p.OwnedChromas["cobalt"] {
		t.Fatalf("failed family purchase changed profile: marks=%d purchased=%d owned=%v",
			p.Marks, p.TotalChromasPurchased, p.OwnedChromas)
	}

	p.Marks = wantCost
	cost, currency, err := BuyChromaFamily(p, "ALIEN")
	if err != nil {
		t.Fatalf("buy Alien chroma family: %v", err)
	}
	if cost != wantCost || currency != Marks || p.Marks != 0 || wantCost != 2880 {
		t.Fatalf("family purchase charged %d %s, leaving %d Marks; want 2880 Marks, 0 left",
			cost, currency, p.Marks)
	}
	for _, chroma := range familyChromas {
		if !p.OwnedChromas[chroma.ID] {
			t.Errorf("family purchase did not grant %q", chroma.ID)
		}
	}
	if p.TotalChromasPurchased != len(familyChromas) {
		t.Fatalf("family purchase recorded %d purchases, want %d chromas",
			p.TotalChromasPurchased, len(familyChromas))
	}
}

func TestBuyChromaFamilyDiscountsTheFullPriceOfEveryThreeChromaSet(t *testing.T) {
	byFamily := make(map[string][]Chroma)
	for _, chroma := range AllChromas() {
		family := chromaFamilyKey(chroma)
		byFamily[family] = append(byFamily[family], chroma)
	}

	familiesTested := 0
	for family, chromas := range byFamily {
		if len(chromas) != 3 {
			continue
		}
		familiesTested++
		unitPrice := chromas[0].Cost
		currency := Currency(chromas[0].Currency)
		var fullPrice int64
		for _, chroma := range chromas {
			if chroma.Cost != unitPrice || Currency(chroma.Currency) != currency {
				t.Errorf("family %q does not have a uniform price and currency", family)
			}
			fullPrice += chroma.Cost
		}

		p, err := profile.New(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		p.OwnedChromas = make(map[string]bool)
		p.Marks = 100000
		p.Crowns = 100000
		cost, gotCurrency, err := BuyChromaFamily(p, family)
		if err != nil {
			t.Errorf("Buy All for %q: %v", family, err)
			continue
		}
		wantCost := (fullPrice*4 + 2) / 5
		if cost != wantCost || gotCurrency != currency {
			t.Errorf("Buy All for %q costs %d %s; want 20%% off all three (%d %s)",
				family, cost, gotCurrency, wantCost, currency)
		}
		if cost <= 2*unitPrice {
			t.Errorf("Buy All for %q costs %d, no more than two full-price chromas (%d)",
				family, cost, 2*unitPrice)
		}
	}
	if familiesTested == 0 {
		t.Fatal("catalog has no three-chroma families")
	}
}

func TestBuyChromaFamilyRejectsPartiallyOwnedFamily(t *testing.T) {
	var familyChromas []Chroma
	for _, chroma := range AllChromas() {
		if chromaFamilyKey(chroma) == "alien" {
			familyChromas = append(familyChromas, chroma)
		}
	}
	if len(familyChromas) != 3 {
		t.Fatalf("Alien family has %d chromas, want 3", len(familyChromas))
	}

	for ownedCount := 1; ownedCount < len(familyChromas); ownedCount++ {
		p, err := profile.New(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		p.Marks = 5000
		for _, chroma := range familyChromas[:ownedCount] {
			p.OwnedChromas[chroma.ID] = true
		}

		if _, _, err := BuyChromaFamily(p, "alien"); err == nil {
			t.Errorf("Buy All should fail with %d already-owned chromas", ownedCount)
		}
		if p.Marks != 5000 || p.TotalChromasPurchased != 0 {
			t.Errorf("rejected partial family purchase changed profile: marks=%d purchases=%d",
				p.Marks, p.TotalChromasPurchased)
		}
		for index, chroma := range familyChromas {
			if got, want := p.OwnedChromas[chroma.ID], index < ownedCount; got != want {
				t.Errorf("owned state for %q = %t, want %t", chroma.ID, got, want)
			}
		}

		nextChroma := familyChromas[ownedCount]
		if err := BuyChroma(p, nextChroma.ID); err != nil {
			t.Errorf("buy remaining chroma %q at full price: %v", nextChroma.ID, err)
		} else if wantMarks := int64(5000) - nextChroma.Cost; p.Marks != wantMarks {
			t.Errorf("remaining chroma purchase leaves %d Marks, want %d", p.Marks, wantMarks)
		}
	}
}

func TestBuyChromaFamilyRejectsSingletonAndDiscountsCrownsFamily(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Crowns = 2000
	if _, _, err := BuyChromaFamily(p, "angel"); err == nil {
		t.Fatal("single-chroma family should not offer Buy All")
	}
	if p.Crowns != 2000 || p.OwnedChromas["angel-prismatic"] {
		t.Fatal("rejected singleton purchase changed profile")
	}

	cost, currency, err := BuyChromaFamily(p, "dragon")
	if err != nil {
		t.Fatalf("buy Dragon chroma family: %v", err)
	}
	if cost != 1920 || currency != Crowns || p.Crowns != 80 {
		t.Fatalf("Dragon family purchase charged %d %s, leaving %d Crowns; want 1920 Crowns, 80 left",
			cost, currency, p.Crowns)
	}
	for _, id := range []string{"dragon-emerald", "dragon-ruby", "dragon-sapphire"} {
		if !p.OwnedChromas[id] {
			t.Errorf("Dragon family purchase did not grant %q", id)
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
	wantPrices := map[string]struct {
		cost     int64
		currency Currency
	}{
		"alien-yellow": {1200, Marks},
		"amazon-green": {1200, Marks},
		"bee-silver":   {1200, Marks},
		"cat-brown":    {900, Marks},
		"dog-gray":     {600, Marks},
		"dragon-ruby":  {800, Crowns},
		"dwarf-purple": {1000, Marks},
		"orc-white":    {250, Marks},
		"reptile-pink": {250, Marks},
		"zulu-orange":  {250, Marks},
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
		expectedPrice := wantPrices[id]
		if chroma.Cost != expectedPrice.cost || chroma.Currency != string(expectedPrice.currency) {
			t.Errorf("chroma %q costs %d %s, want %d %s",
				id, chroma.Cost, chroma.Currency, expectedPrice.cost, expectedPrice.currency)
		}
	}

	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 1200
	if err := BuyChroma(p, "bee-silver"); err != nil {
		t.Fatalf("buy new skin chroma: %v", err)
	}
	if p.Marks != 0 || !p.OwnedChromas["bee-silver"] {
		t.Fatalf("skin chroma purchase left %d Marks and owned=%t", p.Marks, p.OwnedChromas["bee-silver"])
	}
	if err := AssignChroma(p, "B", "bee-silver"); err != nil {
		t.Fatalf("assign new skin chroma: %v", err)
	}
	paperAppearance := p.AssignedChromas["F"]
	if err := AssignChroma(p, "F", "bee-silver"); err != nil {
		t.Fatalf("assign chroma with a color already used by another suit: %v", err)
	}
	if p.AssignedChromas["F"] != "bee-silver" || p.AssignedChromas["B"] != paperAppearance {
		t.Fatalf("same-color chromas did not swap appearances: %+v", p.AssignedChromas)
	}
}

func TestAssignChromaSwapsSuitUsingTheSameColor(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.OwnedChromas["conq-lime"] = true
	p.AssignedChromas["S"] = "lime"
	previousBladeAppearance := p.AssignedChromas["B"]

	if err := AssignChroma(p, "B", "conq-lime"); err != nil {
		t.Fatalf("assign owned chroma with a color already in use: %v", err)
	}
	if p.AssignedChromas["B"] != "conq-lime" || p.AssignedChromas["S"] != previousBladeAppearance {
		t.Fatalf("same-color assignment did not swap appearances: %+v", p.AssignedChromas)
	}
}

func TestRedeemChromaPickConsumesTokenAndGrantsUnownedChroma(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := RedeemChromaPick(p, "bee-silver"); err == nil {
		t.Fatal("redemption without a token should fail")
	}
	p.ChromaPickTokens = 1
	if err := RedeemChromaPick(p, "not-a-chroma"); err == nil {
		t.Fatal("unknown chroma should fail")
	}
	if p.ChromaPickTokens != 1 {
		t.Fatal("invalid chroma consumed the token")
	}
	if err := RedeemChromaPick(p, "bee-silver"); err != nil {
		t.Fatal(err)
	}
	if p.ChromaPickTokens != 0 || !p.OwnedChromas["bee-silver"] {
		t.Fatalf("redemption state = token %d, owned %t", p.ChromaPickTokens, p.OwnedChromas["bee-silver"])
	}
	p.ChromaPickTokens = 1
	if err := RedeemChromaPick(p, "bee-silver"); err == nil || p.ChromaPickTokens != 1 {
		t.Fatal("duplicate chroma redemption should fail without consuming a token")
	}
}
