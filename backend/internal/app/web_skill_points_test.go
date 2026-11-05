package app

import (
	"net/http"
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestSkillPointShopOffersAndPurchase(t *testing.T) {
	now := time.Now()
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.Crowns = 2400
	web := &webServer{playerProfile: p}
	page := web.shopPage(p)
	if len(page.SkillPointOffers) != 2 ||
		page.SkillPointOffers[0].Quantity != 1 || page.SkillPointOffers[0].CrownsCost != 300 ||
		page.SkillPointOffers[1].Quantity != 10 || page.SkillPointOffers[1].CrownsCost != 2400 {
		t.Fatalf("Skill Point offers = %+v", page.SkillPointOffers)
	}
	message, err := web.applyShopAction(p, actionRequest{Action: "buy-skill-points", ID: "skill-point-ten"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SkillPoints != 10 || p.Crowns != 0 || message == "" {
		t.Fatalf("Skill Point purchase = %q, points=%d crowns=%d", message, p.SkillPoints, p.Crowns)
	}
}

func TestSkillNodesCannotBeUnlockedWithCrowns(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Crowns = 1000
	web := &webServer{playerProfile: p}
	response := callDashboardHandler(t, web.handleSkills, http.MethodPost, "/api/skills?preview=dashboard",
		map[string]string{"action": "unlock", "id": "first-blood", "payment": "crowns"})
	if response.Code != http.StatusBadRequest || p.Crowns != 1000 || p.SkillNodes["first-blood"] {
		t.Fatalf("Crown unlock response=%d %s; profile=%+v", response.Code, response.Body.String(), p)
	}
}
