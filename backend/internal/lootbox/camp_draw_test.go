package lootbox

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

type scriptedCampDrawUI struct {
	choose func(exchange int, defending bool, cpuCard *game.Card, playerPool []game.Card) int
}

func (ui *scriptedCampDrawUI) ShowStartingHand([]game.Card)        {}
func (ui *scriptedCampDrawUI) ShowExchangeResult(ExchangeResult)   {}
func (ui *scriptedCampDrawUI) ShowBoxResult(string, BoxOpenResult) {}
func (ui *scriptedCampDrawUI) ChooseCardForExchange(i int, d bool, c *game.Card, p []game.Card) int {
	return ui.choose(i, d, c, p)
}

func newTestProfile(t *testing.T) *profile.Profile {
	t.Helper()
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCampDrawWinCountsSelectCorrectBox(t *testing.T) {
	ui := &scriptedCampDrawUI{choose: func(_ int, _ bool, _ *game.Card, _ []game.Card) int {
		return 0
	}}
	seen := make(map[int]bool)
	for seed := int64(0); seed < 5000 && len(seen) < 4; seed++ {
		p := newTestProfile(t)
		p.DrawsAvailable = 1
		result, err := OpenCampDraw(p, rand.New(rand.NewSource(seed)), ui)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		seen[result.PlayerWins] = true
		if result.BoxType != boxTypeForWins(result.PlayerWins) || len(result.Exchanges) != 3 {
			t.Fatalf("seed %d result = %+v", seed, result)
		}
		if p.DrawsAvailable != 0 {
			t.Fatalf("seed %d left %d draws", seed, p.DrawsAvailable)
		}
	}
	for wins := 0; wins <= 3; wins++ {
		if !seen[wins] {
			t.Fatalf("did not exercise exactly %d player wins (seen %v)", wins, seen)
		}
	}
}

func TestCampDrawSessionResolvesPlayerSelections(t *testing.T) {
	p := newTestProfile(t)
	p.DrawsAvailable = 1
	rng := rand.New(rand.NewSource(19))
	session, err := BeginCampDraw(p, rng)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.StartingHand()) != 3 {
		t.Fatalf("starting hand has %d cards, want 3", len(session.StartingHand()))
	}
	if _, err := session.Finish(p, rng, nil); err == nil {
		t.Fatal("unfinished Camp Draw should not be rewarded")
	}
	for exchangeIndex := 0; exchangeIndex < 3; exchangeIndex++ {
		prompt, ok := session.Prompt()
		if !ok || prompt.ExchangeIndex != exchangeIndex || len(prompt.PlayerPool) != 3-exchangeIndex {
			t.Fatalf("exchange %d prompt = %+v, available=%t", exchangeIndex, prompt, ok)
		}
		if prompt.IsDefending != (exchangeIndex != 1) {
			t.Fatalf("exchange %d defending = %t", exchangeIndex, prompt.IsDefending)
		}
		if prompt.IsDefending && (prompt.CPUCard == nil || !prompt.CPUCard.Valid()) {
			t.Fatalf("defense prompt omitted the revealed CPU attack: %+v", prompt)
		}
		if _, err := session.Play(0, rng); err != nil {
			t.Fatalf("exchange %d selection: %v", exchangeIndex, err)
		}
	}
	if _, ok := session.Prompt(); ok {
		t.Fatal("completed Camp Draw should have no pending prompt")
	}
	result, err := session.Finish(p, rng, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Exchanges) != 3 || p.DrawsAvailable != 0 {
		t.Fatalf("finished result=%+v; draws=%d", result, p.DrawsAvailable)
	}
}

func TestCPUChoicesFollowGreedyRules(t *testing.T) {
	attack := game.Card{Suit: game.Blade, Face: 4}
	pool := []game.Card{
		{Suit: game.Fang, Face: 8},
		{Suit: game.Shield, Face: 7},
		{Suit: game.Shield, Face: 1},
		{Suit: game.Blade, Face: 2},
	}
	if got := CPUDefenderPick(pool, attack); got != (game.Card{Suit: game.Shield, Face: 7}) {
		t.Fatalf("greedy winning response = %v, want Rock 7", got)
	}
	tiePool := []game.Card{
		{Suit: game.Fang, Face: 8},
		{Suit: game.Blade, Face: 2},
	}
	if got := CPUDefenderPick(tiePool, attack); got != (game.Card{Suit: game.Fang, Face: 8}) {
		t.Fatalf("lowest-face tie = %v", got)
	}
	lossPool := []game.Card{
		{Suit: game.Fang, Face: 2},
		{Suit: game.Fang, Face: 1},
	}
	if got := CPUDefenderPick(lossPool, attack); got != lossPool[1] {
		t.Fatalf("cheapest loss = %v, want %v", got, lossPool[1])
	}
	attackPool := []game.Card{
		{Suit: game.Shield, Face: 9},
		{Suit: game.Blade, Face: 9},
		{Suit: game.Fang, Face: 7},
	}
	if got := CPUAttackerPick(attackPool, rand.New(rand.NewSource(1))); got != attackPool[1] {
		t.Fatalf("highest-face tie-break = %v, want %v", got, attackPool[1])
	}
}

func TestRollBoxPrizeFrequenciesAndWeights(t *testing.T) {
	p := newTestProfile(t)
	for _, boxType := range []string{"wooden", "silver", "gold", "diamond"} {
		table, err := LootTable(boxType)
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for _, prize := range table {
			total += prize.Weight
		}
		if total != 100 {
			t.Errorf("%s table weights = %d, want 100", boxType, total)
		}

		rng := rand.New(rand.NewSource(42))
		counts := make(map[string]int)
		const trials = 10000
		for i := 0; i < trials; i++ {
			reward := RollBox(boxType, p, rng)
			key := rewardTypeKey(reward)
			counts[key]++
		}
		for _, prize := range table {
			key := prizeKey(prize)
			want := float64(prize.Weight) / float64(total)
			got := float64(counts[key]) / trials
			tolerance := 0.02
			if want < 0.03 {
				tolerance = 0.012
			}
			if diff := got - want; diff > tolerance || diff < -tolerance {
				t.Errorf("%s %s frequency %.3f, want %.3f ± %.3f", boxType, key, got, want, tolerance)
			}
		}
	}
}

func TestCampLoreReducesCampDrawThresholdWithoutChangingPrizeTables(t *testing.T) {
	p := newTestProfile(t)
	base, err := LootTable("gold")
	if err != nil {
		t.Fatal(err)
	}
	adjusted, err := LootTableForProfile("gold", p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(adjusted, base) {
		t.Fatalf("profile-specific Camp Draw table = %+v, want unchanged table %+v", adjusted, base)
	}
	if RequiredWins(p) != WinsPerCampDraw {
		t.Fatalf("threshold without Camp Lore = %d", RequiredWins(p))
	}
	p.SkillNodes["camp-lore"] = true
	if RequiredWins(p) != 8 {
		t.Fatalf("threshold with Camp Lore = %d, want 8", RequiredWins(p))
	}
	if got := skills.CampDrawThreshold(p, WinsPerCampDraw); got != 8 {
		t.Fatalf("skills threshold = %d", got)
	}
}

func TestOwnedSkinConvertsToMarksRegardlessOfCurator(t *testing.T) {
	p := newTestProfile(t)
	p.SkillNodes["chroma-curator"] = true
	for _, chroma := range chromasForRarity("common") {
		p.OwnedChromas[chroma.ID] = true
	}
	for seed := int64(0); seed < 1000; seed++ {
		reward := RollBox("wooden", p, rand.New(rand.NewSource(seed)))
		if reward.FallbackReason != "Owned skin converted to Marks" {
			continue
		}
		if reward.Marks <= 0 || reward.ChromaID != "" || reward.ChromaRarity != "" {
			t.Fatalf("duplicate skin reward = %+v", reward)
		}
		matchesCost := false
		for _, chroma := range chromasForRarity("common") {
			if p.OwnedChromas[chroma.ID] && reward.Marks == chroma.Cost {
				matchesCost = true
				break
			}
		}
		if !matchesCost {
			t.Fatalf("duplicate skin Marks %d do not match a skin's listed price", reward.Marks)
		}
		if err := applyBoxReward(p, reward); err != nil {
			t.Fatal(err)
		}
		if p.Marks != reward.Marks {
			t.Fatalf("duplicate skin added %d Marks, want %d", p.Marks, reward.Marks)
		}
		return
	}
	t.Fatal("no owned common skin was selected")
}

func TestEnergyBoxPrizeGrantsEnergyAndBonusCrowns(t *testing.T) {
	now := time.Now()
	p := newTestProfile(t)
	p.Energy = 0
	p.EnergyUpdatedAt = now
	if err := p.GrantBox("wooden", 1); err != nil {
		t.Fatal(err)
	}
	for seed := int64(0); seed < 1000; seed++ {
		rolled := RollBox("wooden", p, rand.New(rand.NewSource(seed)))
		if rolled.Energy == 0 {
			continue
		}
		reward, err := OpenPendingBox(p, "wooden", rand.New(rand.NewSource(seed)))
		if err != nil {
			t.Fatal(err)
		}
		if reward.Energy != 1 || reward.Crowns != 1 || p.Energy != 1 || p.Crowns != 1 {
			t.Fatalf("energy box reward = %+v; profile energy=%d crowns=%d",
				reward, p.Energy, p.Crowns)
		}
		return
	}
	t.Fatal("no energy prize found")
}

func TestRebalancedLootTables(t *testing.T) {
	tests := map[string][]LootPrize{
		"wooden": {
			{Weight: 38, Type: PrizeMarks, Amount: 55, BonusCrowns: 1},
			{Weight: 34, Type: PrizeMarks, Amount: 115, BonusCrowns: 1},
			{Weight: 17, Type: PrizeMarks, Amount: 240, BonusCrowns: 1},
			{Weight: 3, Type: PrizeMarks, Amount: 500, BonusCrowns: 1},
			{Weight: 1, Type: PrizeMarks, Amount: 1000, BonusCrowns: 1},
			{Weight: 5, Type: PrizeEnergy, Amount: 1, BonusCrowns: 1},
			{Weight: 2, Type: PrizeChroma, Rarity: "common"},
		},
		"silver": {
			{Weight: 32, Type: PrizeMarks, Amount: 105, BonusCrowns: 2},
			{Weight: 32, Type: PrizeMarks, Amount: 210, BonusCrowns: 2},
			{Weight: 18, Type: PrizeMarks, Amount: 420, BonusCrowns: 2},
			{Weight: 4, Type: PrizeMarks, Amount: 850, BonusCrowns: 2},
			{Weight: 2, Type: PrizeMarks, Amount: 1500, BonusCrowns: 2},
			{Weight: 8, Type: PrizeEnergy, Amount: 1, BonusCrowns: 2},
			{Weight: 3, Type: PrizeChroma, Rarity: "common"},
			{Weight: 1, Type: PrizeChroma, Rarity: "uncommon"},
		},
		"gold": {
			{Weight: 32, Type: PrizeMarks, Amount: 115, BonusCrowns: 4},
			{Weight: 27, Type: PrizeMarks, Amount: 240, BonusCrowns: 4},
			{Weight: 18, Type: PrizeMarks, Amount: 490, BonusCrowns: 4},
			{Weight: 4, Type: PrizeMarks, Amount: 1000, BonusCrowns: 4},
			{Weight: 1, Type: PrizeMarks, Amount: 2200, BonusCrowns: 4},
			{Weight: 8, Type: PrizeEnergy, Amount: 2, BonusCrowns: 4},
			{Weight: 5, Type: PrizeChroma, Rarity: "common"},
			{Weight: 3, Type: PrizeChroma, Rarity: "uncommon"},
			{Weight: 2, Type: PrizeChroma, Rarity: "rare"},
		},
		"diamond": {
			{Weight: 50, Type: PrizeMarks, Amount: 500, BonusCrowns: 8},
			{Weight: 25, Type: PrizeMarks, Amount: 1000, BonusCrowns: 8},
			{Weight: 9, Type: PrizeMarks, Amount: 2000, BonusCrowns: 8},
			{Weight: 5, Type: PrizeMarks, Amount: 4000, BonusCrowns: 8},
			{Weight: 2, Type: PrizeEnergy, Amount: 10, BonusCrowns: 8},
			{Weight: 5, Type: PrizeChroma, Rarity: "rare"},
			{Weight: 3, Type: PrizeChroma, Rarity: "epic"},
			{Weight: 1, Type: PrizeChroma, Rarity: "seasonal_or_premium"},
		},
	}
	for boxType, want := range tests {
		got, err := LootTable(boxType)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Errorf("%s has %d prizes, want %d", boxType, len(got), len(want))
			continue
		}
		for index := range want {
			if got[index] != want[index] {
				t.Errorf("%s prize %d = %+v, want %+v", boxType, index, got[index], want[index])
			}
		}
	}
}

func TestPendingBoxGrantOpenAndValidation(t *testing.T) {
	p := newTestProfile(t)
	if err := p.GrantBox("silver", 2); err != nil {
		t.Fatal(err)
	}
	if p.TotalPendingBoxes() != 2 {
		t.Fatalf("pending total = %d", p.TotalPendingBoxes())
	}
	reward, err := OpenPendingBox(p, "silver", rand.New(rand.NewSource(8)))
	if err != nil {
		t.Fatal(err)
	}
	if p.PendingBoxes["silver"] != 1 || p.Marks != reward.Marks || p.Crowns != reward.Crowns {
		t.Fatalf("pending/open reward state: pending=%d marks=%d crowns=%d reward=%+v",
			p.PendingBoxes["silver"], p.Marks, p.Crowns, reward)
	}
	if _, err := OpenPendingBox(p, "silver", rand.New(rand.NewSource(8))); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPendingBox(p, "silver", rand.New(rand.NewSource(8))); err == nil {
		t.Fatal("empty pending-box count should fail")
	}
	if _, err := OpenPendingBox(p, "unknown", rand.New(rand.NewSource(8))); err == nil {
		t.Fatal("invalid pending-box type should fail")
	}
}

func TestHeadlessSimulationBoxDistribution(t *testing.T) {
	p := newTestProfile(t)
	rng := rand.New(rand.NewSource(9001))
	counts := make(map[string]int)
	const trials = 5000
	for i := 0; i < trials; i++ {
		p.DrawsAvailable = 1
		result, err := OpenCampDraw(p, rng, HeadlessCampDrawUI{})
		if err != nil {
			t.Fatal(err)
		}
		counts[result.BoxType]++
	}
	woodenRate := float64(counts["wooden"]) / trials
	diamondRate := float64(counts["diamond"]) / trials
	if woodenRate < 0.14 || woodenRate > 0.23 {
		t.Errorf("wooden rate %.3f outside 0.14-0.23", woodenRate)
	}
	if diamondRate < 0.05 || diamondRate > 0.12 {
		t.Errorf("diamond rate %.3f outside 0.05-0.12", diamondRate)
	}
}

func rewardTypeKey(reward BoxOpenResult) string {
	switch {
	case reward.ChromaRarity != "":
		return PrizeChroma + ":" + reward.ChromaRarity
	case reward.Energy > 0:
		return fmt.Sprintf("%s:%d+crowns:%d", PrizeEnergy, reward.Energy, reward.Crowns)
	case reward.Marks > 0 && reward.Crowns > 0:
		return fmt.Sprintf("%s:%d+crowns:%d", PrizeMarks, reward.Marks, reward.Crowns)
	case reward.Crowns > 0:
		return fmt.Sprintf("%s:%d", PrizeCrowns, reward.Crowns)
	default:
		return fmt.Sprintf("%s:%d", PrizeMarks, reward.Marks)
	}
}

func prizeKey(prize LootPrize) string {
	if prize.Type == PrizeChroma {
		if len(chromasForRarity(prize.Rarity)) == 0 {
			return fmt.Sprintf("%s:%d", PrizeMarks, rarityFallbackMarks(prize.Rarity))
		}
		return PrizeChroma + ":" + prize.Rarity
	}
	if prize.Type == PrizeEnergy || prize.BonusCrowns > 0 {
		return fmt.Sprintf("%s:%d+crowns:%d", prize.Type, prize.Amount, prize.BonusCrowns)
	}
	return fmt.Sprintf("%s:%d", prize.Type, prize.Amount)
}
