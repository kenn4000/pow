package app

import (
	"fmt"

	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

func webConductorTuning(p *profile.Profile, modifier string, amount int) (heroes.Tuning, error) {
	if modifier == "" && amount == 0 {
		return nil, nil
	}
	if p == nil {
		return nil, fmt.Errorf("Conductor tuning requires a player profile")
	}
	level := skills.AggregateBonuses(p).ConductorLevel
	if level == 0 {
		return nil, fmt.Errorf("unlock The Conductor before tuning a hero")
	}
	if modifier == "" || amount == 0 {
		return nil, fmt.Errorf("choose one modifier and a nonzero tuning amount")
	}
	tuning := heroes.Tuning{modifier: amount}
	if err := validateWebConductorTuning(p, tuning); err != nil {
		return nil, err
	}
	return tuning, nil
}

func validateWebConductorTuning(p *profile.Profile, tuning heroes.Tuning) error {
	level := skills.AggregateBonuses(p).ConductorLevel
	if err := heroes.ValidateTuning(tuning, level); err != nil {
		return fmt.Errorf("invalid Conductor tuning: %w", err)
	}
	return nil
}

func applyWebHeroTuning(hero heroes.Hero, tuning heroes.Tuning) heroes.Hero {
	for modifier, amount := range tuning {
		switch modifier {
		case "bounty":
			hero.Bounty += amount
		case "shift":
			hero.Shift += amount
		case "favored":
			hero.Favored += amount
		}
	}
	return hero
}
