package heroes

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestRecruitIsStarterAndHeroBoxContainsAllNonRecruitHeroes(t *testing.T) {
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
	if len(heroBoxPool) != 10 {
		t.Fatalf("Hero Box pool has %d heroes, want 10", len(heroBoxPool))
	}
	seen := make(map[string]bool)
	for _, heroID := range heroBoxPool {
		if _, ok := ByID(heroID); !ok || heroID == "recruit" || seen[heroID] {
			t.Errorf("Hero Box pool contains invalid or duplicate hero %q", heroID)
		}
		seen[heroID] = true
	}
}

func TestOpenHeroBoxSelectsAllHeroesUniformlyAndPaysHalfPriceForDuplicates(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = HeroBoxMarksCost * 10000
	p.UnlockedHeroes = append(p.UnlockedHeroes, heroBoxPool...)
	var counts = make(map[string]int)
	rng := rand.New(rand.NewSource(42))
	const samples = 10000
	for index := 0; index < samples; index++ {
		result, err := OpenHeroBox(p, rng)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Duplicate || !containsHero(p.UnlockedHeroes, result.HeroID) {
			t.Fatalf("owned hero result = %+v", result)
		}
		cost, ok := profile.HeroCrownsCost(result.HeroID)
		if !ok || result.Crowns != cost/2 {
			t.Fatalf("duplicate reward for %s = %d, want half price %d",
				result.HeroID, result.Crowns, cost/2)
		}
		counts[result.HeroID]++
	}
	for _, heroID := range heroBoxPool {
		rate := float64(counts[heroID]) / samples
		if rate < .07 || rate > .13 {
			t.Errorf("%s frequency %.3f outside target 0.10±0.03", heroID, rate)
		}
	}
}

func TestOpenHeroBoxFreeDiscountAndInsufficientMarks(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = HeroBoxDiscountedCost
	p.SkillNodes["hero-broker"] = true
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
	p.Crowns = 300
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
