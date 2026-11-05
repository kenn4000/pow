package profile

import (
	"testing"
	"time"
)

func TestResetDailyProgressResetsSkillCountersAndModeWins(t *testing.T) {
	now := time.Date(2026, time.January, 8, 10, 0, 0, 0, time.Local)
	p, err := New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.DailyWins = 3
	p.DailyMultiplayerWins = 2
	p.DailyCampaignWins = 1
	p.DailyModeWins["main"] = true
	p.ResetDailyProgress(now.Add(24 * time.Hour))
	if p.DailyWins != 0 || p.DailyMultiplayerWins != 0 || p.DailyCampaignWins != 0 ||
		len(p.DailyModeWins) != 0 {
		t.Fatalf("daily progression was not reset: %+v", p)
	}
}

func TestEnergyOverflowAllowsRegenerationToTwiceMaximum(t *testing.T) {
	now := time.Date(2026, time.January, 8, 10, 0, 0, 0, time.UTC)
	p, err := New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.SkillNodes["overflow"] = true
	p.Energy = p.MaxEnergy
	p.EnergyUpdatedAt = now
	p.ApplyEnergyTick(now.Add(15 * DefaultEnergyRecharge))
	if cap := EnergyStorageCap(p); cap != 2*p.MaxEnergy {
		t.Fatalf("overflow cap = %d, want %d", cap, 2*p.MaxEnergy)
	}
	if p.Energy != EnergyStorageCap(p) {
		t.Fatalf("overflow energy = %d, want cap %d", p.Energy, EnergyStorageCap(p))
	}
}
