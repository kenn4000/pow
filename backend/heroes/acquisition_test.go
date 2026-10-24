package heroes

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/profile"
)

func TestRecruitIsStarterAndBoxPoolContainsTieredUnlocks(t *testing.T) {
	recruit, ok := ByID("recruit")
	if !ok || recruit.Name != "The Recruit" || recruit.Unlock != "default" {
		t.Fatalf("Recruit definition = %+v, found %t", recruit, ok)
	}
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultHeroID != "recruit" || len(p.UnlockedHeroes) != 1 || p.UnlockedHeroes[0] != "recruit" {
		t.Fatalf("new profile starter heroes = %q, %v", p.DefaultHeroID, p.UnlockedHeroes)
	}
	for tier, heroIDs := range heroBoxPool {
		if len(heroIDs) == 0 {
			t.Errorf("%s tier has no heroes", tier)
		}
		for _, heroID := range heroIDs {
			if _, ok := ByID(heroID); !ok || heroID == "recruit" {
				t.Errorf("%s tier contains invalid hero %q", tier, heroID)
			}
		}
	}
}

func TestOpenHeroBoxUsesTierWeightsAndDuplicateBonuses(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = HeroBoxMarksCost * 10000
	for _, heroIDs := range heroBoxPool {
		p.UnlockedHeroes = append(p.UnlockedHeroes, heroIDs...)
	}
	var counts = map[string]int{}
	rng := rand.New(rand.NewSource(42))
	const samples = 10000
	for index := 0; index < samples; index++ {
		result, err := OpenHeroBox(p, rng)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Duplicate {
			t.Fatalf("owned pool hero was awarded again: %+v", result)
		}
		counts[result.DuplicateTier]++
	}
	for tier, target := range map[string]float64{"early": .50, "mid": .30, "late": .20} {
		rate := float64(counts[tier]) / samples
		if rate < target-.025 || rate > target+.025 {
			t.Errorf("%s tier frequency %.3f outside target %.2f±.025", tier, rate, target)
		}
	}

	for _, test := range []struct {
		tier  string
		skill string
		want  int64
	}{
		{"early", "", 100},
		{"mid", "", 150},
		{"late", "", 200},
		{"early", "lucky_dupe", 125},
		{"mid", "lucky_dupe", 187},
		{"late", "lucky_dupe", 250},
		{"early", "premium_dupe", 150},
		{"mid", "premium_dupe", 225},
		{"late", "premium_dupe", 300},
	} {
		t.Run(test.tier+"/"+test.skill, func(t *testing.T) {
			result := openDuplicateTier(t, test.tier, test.skill)
			if result.Crowns != test.want {
				t.Fatalf("duplicate payout = %d, want %d", result.Crowns, test.want)
			}
		})
	}
}

func TestOpenHeroBoxFreeDiscountAndInsufficientMarks(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = HeroBoxDiscountedCost
	p.SkillNodes["hero_broker"] = true
	if _, err := OpenHeroBox(p, rand.New(rand.NewSource(1))); err != nil {
		t.Fatalf("discounted box: %v", err)
	}
	if p.Marks != 0 {
		t.Fatalf("discounted box left %d Marks", p.Marks)
	}
	p.Marks = 0
	p.HeroBoxesAvailable = 1
	if _, err := OpenHeroBox(p, rand.New(rand.NewSource(2))); err != nil {
		t.Fatalf("free box: %v", err)
	}
	if p.HeroBoxesAvailable != 0 || p.Marks != 0 {
		t.Fatalf("free box resources = %d Marks, %d boxes", p.Marks, p.HeroBoxesAvailable)
	}
	if _, err := OpenHeroBox(p, rand.New(rand.NewSource(3))); err == nil {
		t.Fatal("opening a paid box without enough Marks should fail")
	}
	p.Marks = HeroBoxMarksCost
	p.HeroBoxesAvailable = 2
	if _, err := OpenHeroBoxWithMarks(p, rand.New(rand.NewSource(4))); err != nil {
		t.Fatalf("paid box with free boxes waiting: %v", err)
	}
	if p.Marks != HeroBoxMarksCost-HeroBoxDiscountedCost || p.HeroBoxesAvailable != 2 {
		t.Fatalf("paid box resources = %d Marks, %d free boxes", p.Marks, p.HeroBoxesAvailable)
	}
}

func TestOpenHeroBoxAwardsAnUnownedHero(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		p, err := profile.New(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		p.Marks = HeroBoxMarksCost
		result, err := OpenHeroBoxWithMarks(p, rand.New(rand.NewSource(seed)))
		if err != nil {
			t.Fatal(err)
		}
		if result.HeroID != "" {
			if result.Duplicate || result.Crowns != 0 || !containsHero(p.UnlockedHeroes, result.HeroID) {
				t.Fatalf("new hero result/profile = %+v, %v", result, p.UnlockedHeroes)
			}
			if p.Marks != 0 {
				t.Fatalf("hero box left %d Marks", p.Marks)
			}
			return
		}
	}
	t.Fatal("no new hero was awarded in deterministic sample")
}

func TestBuyHeroWithCrownsValidatesOwnershipAndBalance(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := BuyHeroWithCrowns(p, "recruit"); err == nil {
		t.Fatal("Recruit should not be purchasable")
	}
	if err := BuyHeroWithCrowns(p, "warden"); err == nil {
		t.Fatal("purchase without Crowns should fail")
	}
	p.Crowns = DirectHeroCrownsCost
	if err := BuyHeroWithCrowns(p, "warden"); err != nil {
		t.Fatal(err)
	}
	if p.Crowns != 0 || !containsHero(p.UnlockedHeroes, "warden") {
		t.Fatalf("purchase state = %d Crowns, heroes %v", p.Crowns, p.UnlockedHeroes)
	}
	if err := BuyHeroWithCrowns(p, "warden"); err == nil {
		t.Fatal("duplicate purchase should fail")
	}
}

func openDuplicateTier(t *testing.T, tier, skill string) HeroBoxResult {
	t.Helper()
	for seed := int64(0); seed < 1000; seed++ {
		p, err := profile.New(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		p.HeroBoxesAvailable = 1
		for _, heroIDs := range heroBoxPool {
			p.UnlockedHeroes = append(p.UnlockedHeroes, heroIDs...)
		}
		if skill != "" {
			p.SkillNodes[skill] = true
		}
		result, err := OpenHeroBox(p, rand.New(rand.NewSource(seed)))
		if err != nil {
			t.Fatal(err)
		}
		if result.DuplicateTier == tier {
			return result
		}
	}
	t.Fatalf("could not generate %s tier", tier)
	return HeroBoxResult{}
}
