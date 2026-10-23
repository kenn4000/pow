package heroes

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/profile"
	"prisonersofwar/skills"
)

const (
	HeroBoxMarksCost      int64 = 3000
	HeroBoxDiscountedCost int64 = 2700
	DirectHeroCrownsCost        = profile.DirectHeroCrownsCost

	DuplicateEarlyCrowns int64 = 100
	DuplicateMidCrowns   int64 = 150
	DuplicateLateCrowns  int64 = 200

	DuplicateSkillBonus1 = 25
	DuplicateSkillBonus2 = 50

	TierEarly = 50
	TierMid   = 30
	TierLate  = 20
)

type HeroBoxResult struct {
	HeroID        string
	Crowns        int64
	Duplicate     bool
	DuplicateTier string
}

var heroBoxPool = map[string][]string{
	"early": {"treasurer", "ascendant"},
	"mid":   {"gambler", "diplomat", "ascetic"},
	"late":  {"herald", "hater"},
}

func HeroBoxCost(p *profile.Profile) int64 {
	if skills.AggregateBonuses(p).HeroBroker {
		return HeroBoxDiscountedCost
	}
	return HeroBoxMarksCost
}

func OpenHeroBox(p *profile.Profile, rng *rand.Rand) (HeroBoxResult, error) {
	return openHeroBox(p, rng, p != nil && p.HeroBoxesAvailable > 0)
}

func OpenHeroBoxWithMarks(p *profile.Profile, rng *rand.Rand) (HeroBoxResult, error) {
	return openHeroBox(p, rng, false)
}

func openHeroBox(p *profile.Profile, rng *rand.Rand, consumeFree bool) (HeroBoxResult, error) {
	if p == nil {
		return HeroBoxResult{}, errors.New("profile is required")
	}
	if rng == nil {
		return HeroBoxResult{}, errors.New("random source is required")
	}
	free := consumeFree
	if free && p.HeroBoxesAvailable < 1 {
		return HeroBoxResult{}, errors.New("no free Hero Boxes are available")
	}
	cost := HeroBoxCost(p)
	if !free && p.Marks < cost {
		return HeroBoxResult{}, fmt.Errorf("need %d Marks; you have %d", cost, p.Marks)
	}

	roll := rng.Intn(TierEarly + TierMid + TierLate)
	tier := "late"
	switch {
	case roll < TierEarly:
		tier = "early"
	case roll < TierEarly+TierMid:
		tier = "mid"
	}
	pool := heroBoxPool[tier]
	heroID := pool[rng.Intn(len(pool))]
	if free {
		p.HeroBoxesAvailable--
	} else {
		p.Marks -= cost
	}
	if !containsHero(p.UnlockedHeroes, heroID) {
		p.UnlockedHeroes = append(p.UnlockedHeroes, heroID)
		return HeroBoxResult{HeroID: heroID}, nil
	}

	duplicateCrowns := map[string]int64{
		"early": DuplicateEarlyCrowns,
		"mid":   DuplicateMidCrowns,
		"late":  DuplicateLateCrowns,
	}[tier]
	duplicateBonus := skills.AggregateBonuses(p).DuplicateBonusPercent
	if duplicateBonus > 0 {
		duplicateCrowns += duplicateCrowns * int64(duplicateBonus) / 100
	}
	p.Crowns += duplicateCrowns
	return HeroBoxResult{Crowns: duplicateCrowns, Duplicate: true, DuplicateTier: tier}, nil
}

func BuyHeroWithCrowns(p *profile.Profile, heroID string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	hero, ok := ByID(heroID)
	if !ok || hero.ID == "recruit" {
		return fmt.Errorf("hero %q cannot be purchased", heroID)
	}
	return p.BuyHeroWithCrowns(heroID)
}

func containsHero(heroIDs []string, heroID string) bool {
	for _, id := range heroIDs {
		if id == heroID {
			return true
		}
	}
	return false
}
