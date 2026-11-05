package app

import (
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestCampaignWinSkillRewardsApplyDailyBonusesOnce(t *testing.T) {
	now := time.Date(2026, time.January, 8, 10, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.SkillNodes["first-blood"] = true
	p.SkillNodes["daily-double"] = true
	p.SkillNodes["rations-readiness"] = true
	p.SkillNodes["bounty-board"] = true
	p.Energy = p.MaxEnergy - 1

	firstBonus, readiness, err := applyCampaignWinSkillRewards(p, "basic", 100, now)
	if err != nil {
		t.Fatal(err)
	}
	if firstBonus != 100 || !readiness || p.Marks != 100 ||
		p.RestartMissionTokens != 1 || p.UndoMoveTokens != 1 || p.Energy != p.MaxEnergy {
		t.Fatalf("first campaign reward: extra=%d readiness=%t profile=%+v", firstBonus, readiness, p)
	}
	secondBonus, readiness, err := applyCampaignWinSkillRewards(p, "basic", 100, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if secondBonus != 50 || readiness || p.Marks != 150 ||
		p.RestartMissionTokens != 1 || p.UndoMoveTokens != 1 {
		t.Fatalf("second campaign reward: extra=%d readiness=%t profile=%+v", secondBonus, readiness, p)
	}
}
