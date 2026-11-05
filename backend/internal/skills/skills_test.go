package skills

import (
	"strings"
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestSkillTreeHasSixBranchesAndTierPointCosts(t *testing.T) {
	nodes := AllNodes()
	if len(nodes) != 27 {
		t.Fatalf("skill tree has %d nodes, want 27", len(nodes))
	}
	for _, test := range []struct {
		id           string
		branch       string
		tier         int
		prerequisite string
	}{
		{"first-blood", "daily-wins", 1, ""},
		{"daily-double", "daily-wins", 2, "first-blood"},
		{"rations-readiness", "daily-wins", 2, "first-blood"},
		{"daily-triumph", "daily-wins", 3, "daily-double"},
		{"streak", "multiplayer", 1, ""},
		{"field-warden", "multiplayer", 2, "streak"},
		{"mark-of-victory", "multiplayer", 2, "streak"},
		{"bounty-hunter", "multiplayer", 3, "mark-of-victory"},
		{"quartermaster", "general-rewards", 1, ""},
		{"salvager", "general-rewards", 1, ""},
		{"curator", "general-rewards", 2, "quartermaster"},
		{"bounty-board", "general-rewards", 2, "salvager"},
		{"expanded-rations", "energy", 1, ""},
		{"quick-recovery", "energy", 2, "expanded-rations"},
		{"stockpile", "energy", 2, "expanded-rations"},
		{"second-wind", "energy", 3, "quick-recovery"},
		{"overflow", "energy", 3, "stockpile"},
		{"thrifty-quartermaster", "collection", 1, ""},
		{"hero-broker", "collection", 2, "thrifty-quartermaster"},
		{"veterans-discount", "collection", 2, "thrifty-quartermaster"},
		{"lucky-duplicate", "collection", 2, "thrifty-quartermaster"},
		{"premium-duplicate", "collection", 3, "lucky-duplicate"},
		{"camp-lore", "camp", 1, ""},
		{"reroll", "camp", 2, "camp-lore"},
		{"loaded-deck", "camp", 2, "camp-lore"},
		{"conductor-1", "camp", 2, "camp-lore"},
		{"conductor-2", "camp", 3, "conductor-1"},
	} {
		node, ok := FindNode(test.id)
		if !ok {
			t.Errorf("missing skill node %q", test.id)
			continue
		}
		wantCost := test.tier
		if node.Branch != test.branch || node.Tier != test.tier ||
			node.CostSkillPoints != wantCost || node.Prerequisite != test.prerequisite {
			t.Errorf("%s = %+v, want branch %s, tier/cost %d, prerequisite %q",
				test.id, node, test.branch, test.tier, test.prerequisite)
		}
	}
}

func TestUnlockNodeUsesOnlyTierSkillPointCost(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := UnlockNodeWithSkillPoint(p, "quartermaster"); err == nil {
		t.Fatal("unlock with no Skill Points should fail")
	}
	p.SkillPoints = 2
	if err := UnlockNodeWithSkillPoint(p, "field-warden"); err == nil || !strings.Contains(err.Error(), "requires") {
		t.Fatalf("missing prerequisite error = %v", err)
	}
	if p.SkillPoints != 2 || p.Crowns != 0 {
		t.Fatalf("failed unlock changed resources: points=%d crowns=%d", p.SkillPoints, p.Crowns)
	}
	if err := UnlockNodeWithSkillPoint(p, "streak"); err != nil {
		t.Fatal(err)
	}
	if err := UnlockNodeWithSkillPoint(p, "field-warden"); err == nil || !strings.Contains(err.Error(), "need 2") {
		t.Fatalf("insufficient T2 points error = %v", err)
	}
	p.SkillPoints = 2
	if err := UnlockNodeWithSkillPoint(p, "field-warden"); err != nil {
		t.Fatal(err)
	}
	if p.SkillPoints != 0 || p.Crowns != 0 || !p.SkillNodes["field-warden"] {
		t.Fatalf("T2 purchase state: points=%d crowns=%d nodes=%v", p.SkillPoints, p.Crowns, p.SkillNodes)
	}
	if err := UnlockNodeWithSkillPoint(p, "field-warden"); err == nil {
		t.Fatal("duplicate skill unlock should fail")
	}
}

func TestAggregateBonusesForOverhauledBranches(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"first-blood", "daily-double", "rations-readiness", "daily-triumph",
		"streak", "field-warden", "mark-of-victory", "bounty-hunter",
		"quartermaster", "salvager", "curator", "bounty-board",
		"expanded-rations", "quick-recovery", "stockpile", "second-wind", "overflow",
		"thrifty-quartermaster", "hero-broker", "veterans-discount", "lucky-duplicate", "premium-duplicate",
		"camp-lore", "reroll", "loaded-deck", "conductor-1", "conductor-2",
	} {
		p.SkillNodes[id] = true
	}
	bonuses := AggregateBonuses(p)
	if bonuses.MarksBasePercent != 10 || bonuses.LossMarksPercent != 15 ||
		bonuses.FirstWinBonus != 50 || bonuses.SecondWinBonus != 50 ||
		!bonuses.RationsReadiness || bonuses.DailyWinCrowns != 5 ||
		bonuses.RankedStreakCrowns != 3 || bonuses.RankedPayoutPercent != 15 ||
		bonuses.MultiplayerMarksPercent != 20 || bonuses.RankedBestOfThreeMarks != 250 ||
		bonuses.CampaignMarksPercent != 50 || bonuses.FirstModeWinMarks != 50 ||
		bonuses.CampDrawReduction != 2 || !bonuses.CampDrawReroll ||
		bonuses.CampDrawExtraCards != 1 || bonuses.MaxEnergy != 6 ||
		bonuses.EnergyRechargePercent != 10 || !bonuses.DailyFreeRefill ||
		!bonuses.EnergyOverflow || bonuses.ConsumableDiscountPercent != 10 ||
		bonuses.HeroBoxDiscountPercent != 10 || bonuses.ChestDiscountPercent != 5 ||
		bonuses.DuplicateBonusPercent != 50 || bonuses.ConductorLevel != 2 {
		t.Fatalf("skill bonuses = %+v", bonuses)
	}
	if got := CampDrawThreshold(p, 10); got != 8 {
		t.Fatalf("Camp Draw threshold = %d, want 8", got)
	}
}
