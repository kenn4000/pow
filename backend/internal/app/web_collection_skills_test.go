package app

import (
	"testing"
	"time"

	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func TestCollectionSkillsDiscountMarksPricesAndCharges(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.SkillNodes["thrifty-quartermaster"] = true
	p.SkillNodes["veterans-discount"] = true
	p.SkillNodes["hero-broker"] = true

	offers := shopConsumableOffers(p)
	var replaceCard webShopConsumableOffer
	for _, offer := range offers {
		if offer.ID == "replace-card-one" {
			replaceCard = offer
		}
	}
	if replaceCard.MarksCost != 162 {
		t.Fatalf("discounted consumable cost = %d, want 162", replaceCard.MarksCost)
	}
	p.Marks = replaceCard.MarksCost
	if _, err := buyShopConsumable(p, replaceCard.ID, "marks"); err != nil {
		t.Fatal(err)
	}
	if p.Marks != 0 || p.ReplaceCardTokens != 1 {
		t.Fatalf("discounted consumable purchase left %d Marks and %d tokens", p.Marks, p.ReplaceCardTokens)
	}

	p.Marks = shopChestMarksCost(p, shopWoodenBoxMarksCost)
	if p.Marks != 285 {
		t.Fatalf("discounted wooden chest price = %d, want 285", p.Marks)
	}
	if _, err := buyShopBox(p, "wooden", "marks"); err != nil {
		t.Fatal(err)
	}
	if p.Marks != 0 {
		t.Fatalf("discounted chest purchase left %d Marks", p.Marks)
	}

	if got := heroes.HeroBoxCost(p); got != 7200 {
		t.Fatalf("discounted Hero Box cost = %d, want 7200", got)
	}
}
