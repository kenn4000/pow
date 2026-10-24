package skills

import (
	"strings"
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestCanUnlockPrerequisiteCurrencyAndBonuses(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if allowed, reason := CanUnlock(p, "field-warden"); allowed || !strings.Contains(reason, "requires") {
		t.Fatalf("locked prerequisite result = %t, %q", allowed, reason)
	}
	p.Crowns = 100
	if err := UnlockNode(p, "quartermaster"); err != nil {
		t.Fatal(err)
	}
	if err := UnlockNode(p, "field-warden"); err != nil {
		t.Fatal(err)
	}
	if err := UnlockNode(p, "quartermaster"); err == nil {
		t.Fatal("duplicate skill unlock should fail")
	}
	bonuses := AggregateBonuses(p)
	if bonuses.MarksBasePercent != 10 || bonuses.ModeMultiplier != 5 || p.Crowns != 55 {
		t.Fatalf("bonuses/currency = %+v, %d", bonuses, p.Crowns)
	}
	p.Crowns = 0
	if err := UnlockNode(p, "first-blood"); err == nil {
		t.Fatal("unlock without Crowns should fail")
	}
}

func TestUnlockNodeWithSkillPointValidatesAndConsumesExactlyOne(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := UnlockNodeWithSkillPoint(p, "quartermaster"); err == nil {
		t.Fatal("unlock with no Skill Tree Points should fail")
	}
	p.SkillPoints = 1
	if err := UnlockNodeWithSkillPoint(p, "field-warden"); err == nil {
		t.Fatal("missing prerequisite should fail")
	}
	if p.SkillPoints != 1 {
		t.Fatal("failed prerequisite consumed a point")
	}
	if err := UnlockNodeWithSkillPoint(p, "quartermaster"); err != nil {
		t.Fatal(err)
	}
	if p.SkillPoints != 0 || !p.SkillNodes["quartermaster"] {
		t.Fatalf("skill-point purchase state = %d, %+v", p.SkillPoints, p.SkillNodes)
	}
	if err := UnlockNodeWithSkillPoint(p, "quartermaster"); err == nil {
		t.Fatal("double unlock should fail")
	}
	if p.SkillPoints != 0 {
		t.Fatal("failed duplicate unlock consumed a point")
	}
}

func TestSkillTreeExpansionContainsTenSpecifiedNodesAndEffects(t *testing.T) {
	nodes := AllNodes()
	if len(nodes) != 19 {
		t.Fatalf("skill tree has %d nodes, want 19 (9 existing + 10 new)", len(nodes))
	}
	for _, test := range []struct {
		id   string
		cost int64
	}{
		{"conductor_1", Conductor1Cost},
		{"conductor_2", Conductor2Cost},
		{"hero_broker", HeroBrokerCost},
		{"curator", CuratorCost},
		{"lucky_dupe", LuckyDupeCost},
		{"premium_dupe", PremiumDupeCost},
		{"campaign_free", CampaignFreeCost},
		{"daily_energy", DailyEnergyCost},
		{"hero_history", HeroHistoryCost},
		{"hero_preview", HeroPreviewCost},
	} {
		node, ok := FindNode(test.id)
		if !ok || node.CostCrowns != test.cost {
			t.Errorf("%s node = %+v, found %t; want cost %d", test.id, node, ok, test.cost)
		}
	}
	if node, _ := FindNode("conductor_2"); node.Prerequisite != "conductor_1" {
		t.Errorf("conductor_2 prerequisite = %q", node.Prerequisite)
	}
	if node, _ := FindNode("premium_dupe"); node.Prerequisite != "lucky_dupe" {
		t.Errorf("premium_dupe prerequisite = %q", node.Prerequisite)
	}
	if node, _ := FindNode("hero_preview"); node.Prerequisite != "hero_history" {
		t.Errorf("hero_preview prerequisite = %q", node.Prerequisite)
	}
}

func TestAggregateHeroSkillsAndDailyEnergyLogin(t *testing.T) {
	now := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"conductor_1", "conductor_2", "hero_broker", "curator", "lucky_dupe",
		"premium_dupe", "campaign_free", "daily_energy", "hero_history", "hero_preview",
	} {
		p.SkillNodes[id] = true
	}
	bonuses := AggregateBonuses(p)
	if bonuses.ConductorLevel != 2 || !bonuses.HeroBroker || bonuses.CampaignMarksPercent != 50 ||
		bonuses.DuplicateBonusPercent != 50 || !bonuses.CampaignFree || !bonuses.DailyEnergy ||
		!bonuses.HeroHistory || !bonuses.HeroPreview {
		t.Fatalf("hero skill bonuses = %+v", bonuses)
	}
	p.Energy = 4
	p.LastLoginDate = now.AddDate(0, 0, -1)
	p.EnergyUpdatedAt = now
	p.ApplyLogin(now)
	if p.Energy != 6 {
		t.Fatalf("daily login energy = %d, want 6", p.Energy)
	}
	sameDay := now.Add(time.Hour)
	p.EnergyUpdatedAt = sameDay
	p.ApplyLogin(sameDay)
	if p.Energy != 6 {
		t.Fatalf("same-day login granted energy twice: %d", p.Energy)
	}
	nextDay := now.AddDate(0, 0, 1)
	p.EnergyUpdatedAt = nextDay
	p.ApplyLogin(nextDay)
	if p.Energy != 8 {
		t.Fatalf("next-day energy = %d, want 8", p.Energy)
	}
}
