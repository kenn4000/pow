package app

import (
	"errors"
	"fmt"

	"prisonersofwar/internal/profile"
)

func shopSkillPointOffers() []webShopSkillPointOffer {
	return []webShopSkillPointOffer{
		{ID: "skill-point-one", Name: "Skill Point", Quantity: 1, CrownsCost: 300},
		{ID: "skill-point-ten", Name: "Skill Points", Quantity: 10, CrownsCost: 2400},
	}
}

func buyShopSkillPoints(p *profile.Profile, id string) (string, error) {
	if p == nil {
		return "", errors.New("profile is required")
	}
	for _, offer := range shopSkillPointOffers() {
		if offer.ID != id {
			continue
		}
		if p.Crowns < offer.CrownsCost {
			return "", fmt.Errorf("need %d Crowns; you have %d", offer.CrownsCost, p.Crowns)
		}
		p.Crowns -= offer.CrownsCost
		p.SkillPoints += offer.Quantity
		return fmt.Sprintf("Purchased %d %s for %d Crowns.", offer.Quantity, offer.Name, offer.CrownsCost), nil
	}
	return "", fmt.Errorf("unknown Skill Point offer %q", id)
}
