package catalog

import (
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestClownPurpleUsesItsOwnSpriteSheet(t *testing.T) {
	clown, ok := FindChroma("clown-purple")
	if !ok {
		t.Fatal("Clown purple chroma is missing")
	}
	tribal, ok := FindChroma("tribal-purple")
	if !ok {
		t.Fatal("Tribal purple chroma is missing")
	}
	if clown.Skin != "shop/clown/purple" || tribal.Skin != "shop/tribal/purple" {
		t.Fatalf("purple chroma artwork paths = clown:%q tribal:%q", clown.Skin, tribal.Skin)
	}
}

func TestShopPurchasesChromasAndColorAssignments(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	chroma, ok := FindChroma("tribal-green")
	if !ok {
		t.Fatal("Tribal green chroma is missing")
	}
	p.Marks = chroma.Cost
	if err := BuyChroma(p, chroma.ID); err != nil {
		t.Fatal(err)
	}
	if p.Marks != 0 || !p.OwnedChromas[chroma.ID] || p.TotalChromasPurchased != 1 {
		t.Fatalf("chroma purchase left %d Marks, owned=%t, total purchases=%d",
			p.Marks, p.OwnedChromas[chroma.ID], p.TotalChromasPurchased)
	}
	if err := AssignChroma(p, "B", chroma.ID); err != nil {
		t.Fatal(err)
	}
	paperAppearance := p.AssignedChromas["F"]
	if err := AssignChroma(p, "F", chroma.ID); err != nil {
		t.Fatalf("assign existing color to another suit: %v", err)
	}
	if p.AssignedChromas["F"] != chroma.ID || p.AssignedChromas["B"] != paperAppearance {
		t.Fatalf("duplicate color assignment did not swap appearances: %+v", p.AssignedChromas)
	}
	conqPink, ok := FindChroma("conq-pink")
	if !ok {
		t.Fatal("Conquistador pink chroma is missing")
	}
	p.Marks = conqPink.Cost
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
		"tribal-red": "Tribal (red)", "tribal-blue": "Tribal (blue)", "tribal-yellow": "Tribal (yellow)",
		"tribal-green": "Tribal (green)", "tribal-orange": "Tribal (orange)", "tribal-purple": "Tribal (purple)",
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

func TestSkinRarityRanksUseFiveLevelScale(t *testing.T) {
	want := map[string]int{
		"tribal-red": 0, "tribal-green": 1, "conq-gold": 0, "dragon-ruby": 3,
	}
	rarities := []string{"common", "uncommon", "rare", "epic", "seasonal"}
	for _, chroma := range AllChromas() {
		if chroma.RarityRank < 0 || chroma.RarityRank > 4 {
			t.Errorf("chroma %q has unsupported rarity rank %d", chroma.ID, chroma.RarityRank)
		}
		if chroma.RarityRank >= 0 && chroma.RarityRank < len(rarities) &&
			chroma.Rarity != rarities[chroma.RarityRank] {
			t.Errorf("chroma %q rarity = %q for rank %d, want %q",
				chroma.ID, chroma.Rarity, chroma.RarityRank, rarities[chroma.RarityRank])
		}
		if expected, ok := want[chroma.ID]; ok && chroma.RarityRank != expected {
			t.Errorf("chroma %q rarity rank = %d, want %d", chroma.ID, chroma.RarityRank, expected)
		}
	}
}

func TestConquistadorChromasUseSheetPrices(t *testing.T) {
	for _, id := range []string{"conq-lime", "conq-gold", "conq-pink", "conq-blue", "conq-skyblue"} {
		chroma, ok := FindChroma(id)
		if !ok {
			t.Errorf("chroma %q is missing", id)
			continue
		}
		if chroma.Cost <= 0 || (chroma.Currency != string(Marks) && chroma.Currency != string(Crowns)) {
			t.Errorf("chroma %q has invalid sheet price: %+v", id, chroma)
		}
	}
}

func TestTribalAndDragonPricesUseSheetMetadata(t *testing.T) {
	for _, id := range []string{
		"tribal-blue", "tribal-red", "tribal-yellow", "tribal-green", "tribal-orange", "tribal-purple",
	} {
		chroma, ok := FindChroma(id)
		if !ok || chroma.Cost <= 0 || chroma.Currency != string(Marks) {
			t.Errorf("Tribal chroma %q = %+v, want a positive Marks price from its sheet", id, chroma)
		}
	}
	for _, id := range []string{"dragon-emerald", "dragon-ruby", "dragon-sapphire"} {
		chroma, ok := FindChroma(id)
		if !ok || chroma.Cost <= 0 || chroma.Currency != string(Crowns) {
			t.Errorf("Dragon chroma %q = %+v, want a positive Crowns price from its sheet", id, chroma)
		}
	}
}

func TestChromaFamilyPricesFollowSheetMetadata(t *testing.T) {
	byFamily := make(map[string][]Chroma)
	for _, chroma := range AllChromas() {
		family := chromaFamilyKey(chroma)
		byFamily[family] = append(byFamily[family], chroma)
	}

	for family, chromas := range byFamily {
		if len(chromas) < 2 {
			continue
		}
		currency := chromas[0].Currency
		for _, chroma := range chromas {
			if chroma.Currency != currency {
				t.Errorf("family %q mixes currencies: %s and %s", family, currency, chroma.Currency)
			}
			if chroma.Cost <= 0 {
				t.Errorf("family %q chroma %q has invalid price %d", family, chroma.ID, chroma.Cost)
			}
		}
	}
}

func TestBuyChromaFamilyDiscountsFullSetPrice(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var familyChromas []Chroma
	for _, chroma := range AllChromas() {
		if chromaFamilyKey(chroma) == "alien" {
			familyChromas = append(familyChromas, chroma)
		}
	}
	familyChromas = uniqueChromaAppearances(familyChromas)
	if len(familyChromas) < 2 {
		t.Fatalf("Alien family has %d purchasable chromas, want at least 2", len(familyChromas))
	}
	var fullPrice int64
	for _, chroma := range familyChromas {
		fullPrice += chroma.Cost
	}
	wantCost := (fullPrice*4 + 2) / 5
	p.Marks = wantCost - 1
	if _, _, err := BuyChromaFamily(p, "alien"); err == nil {
		t.Fatal("family purchase should fail when the discounted price is unaffordable")
	}
	if p.Marks != wantCost-1 || p.TotalChromasPurchased != 0 {
		t.Fatalf("failed family purchase changed profile: marks=%d purchased=%d owned=%v",
			p.Marks, p.TotalChromasPurchased, p.OwnedChromas)
	}

	p.Marks = wantCost
	cost, currency, err := BuyChromaFamily(p, "ALIEN")
	if err != nil {
		t.Fatalf("buy Alien chroma family: %v", err)
	}
	if cost != wantCost || currency != Marks || p.Marks != 0 {
		t.Fatalf("family purchase charged %d %s, leaving %d Marks; want %d Marks, 0 left",
			cost, currency, p.Marks, wantCost)
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

func TestConquistadorFamilyBuyAllSkipsDuplicateSkyBlue(t *testing.T) {
	var familyChromas []Chroma
	for _, chroma := range AllChromas() {
		if chromaFamilyKey(chroma) == "conq" {
			familyChromas = append(familyChromas, chroma)
		}
	}
	unique := uniqueChromaAppearances(familyChromas)
	if len(unique) != 4 {
		t.Fatalf("Conquistador family has %d unique appearances, want 4", len(unique))
	}
	ids := make(map[string]bool, len(unique))
	var fullPrice int64
	for _, chroma := range unique {
		ids[chroma.ID] = true
		fullPrice += chroma.Cost
	}
	if !ids["conq-skyblue"] || ids["conq-blue"] {
		t.Fatalf("duplicate sky-blue representative IDs = %v", ids)
	}

	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = fullPrice
	if _, _, err := BuyChromaFamily(p, "conq"); err != nil {
		t.Fatal(err)
	}
	if p.OwnedChromas["conq-blue"] || !p.OwnedChromas["conq-skyblue"] ||
		p.TotalChromasPurchased != len(unique) {
		t.Fatalf("family purchase did not collapse duplicate appearance: owned=%v purchases=%d",
			p.OwnedChromas, p.TotalChromasPurchased)
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
		currency := Currency(chromas[0].Currency)
		var fullPrice int64
		for _, chroma := range chromas {
			if Currency(chroma.Currency) != currency {
				t.Errorf("family %q does not have a uniform currency", family)
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
	familyChromas = uniqueChromaAppearances(familyChromas)
	if len(familyChromas) < 2 {
		t.Fatalf("Alien family has %d purchasable chromas, want at least 2", len(familyChromas))
	}

	for ownedCount := 1; ownedCount < len(familyChromas); ownedCount++ {
		p, err := profile.New(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		p.Marks = 100000
		for _, chroma := range familyChromas[:ownedCount] {
			p.OwnedChromas[chroma.ID] = true
		}

		if _, _, err := BuyChromaFamily(p, "alien"); err == nil {
			t.Errorf("Buy All should fail with %d already-owned chromas", ownedCount)
		}
		if p.Marks != 100000 || p.TotalChromasPurchased != 0 {
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
		} else if wantMarks := int64(100000) - nextChroma.Cost; p.Marks != wantMarks {
			t.Errorf("remaining chroma purchase leaves %d Marks, want %d", p.Marks, wantMarks)
		}
	}
}

func TestBuyChromaFamilyRejectsUnknownAndDiscountsCrownsFamily(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := BuyChromaFamily(p, "unknown-family"); err == nil {
		t.Fatal("unknown family should not offer Buy All")
	}
	var fullPrice int64
	for _, chroma := range AllChromas() {
		if chromaFamilyKey(chroma) == "dragon" {
			fullPrice += chroma.Cost
		}
	}
	p.Crowns = fullPrice

	cost, currency, err := BuyChromaFamily(p, "dragon")
	if err != nil {
		t.Fatalf("buy Dragon chroma family: %v", err)
	}
	wantCost := (fullPrice*4 + 2) / 5
	if cost != wantCost || currency != Crowns || p.Crowns != fullPrice-wantCost {
		t.Fatalf("Dragon family purchase charged %d %s, leaving %d Crowns; want %d Crowns, %d left",
			cost, currency, p.Crowns, wantCost, fullPrice-wantCost)
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
		if chroma.Cost <= 0 || (chroma.Currency != string(Marks) && chroma.Currency != string(Crowns)) {
			t.Errorf("chroma %q has an invalid price: %+v", id, chroma)
		}
	}

	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	bee, ok := FindChroma("bee-silver")
	if !ok {
		t.Fatal("Bee silver chroma is missing")
	}
	if bee.Currency == string(Marks) {
		p.Marks = bee.Cost
	} else {
		p.Crowns = bee.Cost
	}
	if err := BuyChroma(p, "bee-silver"); err != nil {
		t.Fatalf("buy new skin chroma: %v", err)
	}
	if (bee.Currency == string(Marks) && p.Marks != 0) ||
		(bee.Currency == string(Crowns) && p.Crowns != 0) || !p.OwnedChromas["bee-silver"] {
		t.Fatalf("skin chroma purchase left %d Marks and %d Crowns, owned=%t",
			p.Marks, p.Crowns, p.OwnedChromas["bee-silver"])
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
