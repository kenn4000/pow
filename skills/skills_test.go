package skills

import (
	"strings"
	"testing"
	"time"

	"prisonersofwar/profile"
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
