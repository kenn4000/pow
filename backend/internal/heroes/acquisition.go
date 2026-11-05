package heroes

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

const (
	HeroBoxMarksCost      int64 = 8000
	HeroBoxDiscountedCost int64 = 7200
)

type HeroBoxResult struct {
	HeroID    string `json:"heroId,omitempty"`
	Crowns    int64  `json:"crowns,omitempty"`
	Duplicate bool   `json:"duplicate,omitempty"`
}

var heroBoxPool = []string{
	"warden", "hunter", "treasurer", "ascendant", "anchor",
	"gambler", "diplomat", "ascetic", "herald", "hater",
}

type BoxOdds struct {
	Label  string
	Chance float64
}

func HeroBoxOdds(p *profile.Profile) []BoxOdds {
	entries := make([]BoxOdds, 0, len(heroBoxPool))
	for _, id := range heroBoxPool {
		hero, _ := ByID(id)
		cost, _ := profile.HeroCrownsCost(id)
		duplicate := cost / 2
		if bonus := skills.AggregateBonuses(p).DuplicateBonusPercent; bonus > 0 {
			duplicate += duplicate * int64(bonus) / 100
		}
		entries = append(entries, BoxOdds{
			Label:  fmt.Sprintf("%s; duplicate: %d Crowns", hero.Name, duplicate),
			Chance: 100 / float64(len(heroBoxPool)),
		})
	}
	return entries
}

func HeroBoxCost(p *profile.Profile) int64 {
	discount := skills.AggregateBonuses(p).HeroBoxDiscountPercent
	return HeroBoxMarksCost * int64(100-discount) / 100
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

	heroID := heroBoxPool[rng.Intn(len(heroBoxPool))]
	if free {
		p.HeroBoxesAvailable--
	} else {
		p.Marks -= cost
	}
	if !containsHero(p.UnlockedHeroes, heroID) {
		p.UnlockedHeroes = append(p.UnlockedHeroes, heroID)
		return HeroBoxResult{HeroID: heroID}, nil
	}

	heroCost, ok := profile.HeroCrownsCost(heroID)
	if !ok {
		return HeroBoxResult{}, fmt.Errorf("hero %q has no Crown purchase price", heroID)
	}
	duplicateCrowns := heroCost / 2
	if bonus := skills.AggregateBonuses(p).DuplicateBonusPercent; bonus > 0 {
		duplicateCrowns += duplicateCrowns * int64(bonus) / 100
	}
	p.Crowns += duplicateCrowns
	return HeroBoxResult{HeroID: heroID, Crowns: duplicateCrowns, Duplicate: true}, nil
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
