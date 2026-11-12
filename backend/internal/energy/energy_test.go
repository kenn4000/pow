package energy

import (
	"testing"
	"time"

	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

func TestMatchCosts(t *testing.T) {
	if got := MatchCost(false); got != 1 {
		t.Fatalf("single-game Energy cost = %d, want 1", got)
	}
	if got := MatchCost(true); got != 2 {
		t.Fatalf("best-of-three Energy cost = %d, want 2", got)
	}
}

func TestSpendAdsAndDailyReset(t *testing.T) {
	now := time.Date(2025, time.April, 1, 10, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := Spend(p, 1, now); err != nil {
		t.Fatal(err)
	}
	if p.Energy != profile.DefaultMaxEnergy-1 {
		t.Fatalf("energy after spend = %d", p.Energy)
	}
	for i := 0; i < DailyAdLimit; i++ {
		if err := WatchAd(p, now); err != nil {
			t.Fatal(err)
		}
	}
	if p.Energy != p.MaxEnergy || p.AdsWatchedToday != DailyAdLimit {
		t.Fatalf("ad state = energy %d, ads %d", p.Energy, p.AdsWatchedToday)
	}
	if err := WatchAd(p, now); err == nil {
		t.Fatal("expected the daily ad cap to be enforced")
	}
	nextDay := now.Add(24 * time.Hour)
	Current(p, nextDay)
	if p.AdsWatchedToday != 0 {
		t.Fatalf("ads after daily reset = %d", p.AdsWatchedToday)
	}
	if err := Spend(p, 1, nextDay); err != nil {
		t.Fatal(err)
	}
	if err := Spend(p, p.MaxEnergy, nextDay); err == nil {
		t.Fatal("expected an unaffordable energy spend to fail")
	}
}

func TestEnergySkillBonusesAndFreeRefill(t *testing.T) {
	now := time.Now()
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.SkillPoints = 6
	for _, id := range []string{"expanded-rations", "quick-recovery", "second-wind"} {
		if err := skills.UnlockNodeWithSkillPoint(p, id); err != nil {
			t.Fatal(err)
		}
	}
	p.Energy = 2
	if got := Current(p, now); got != 2 || p.MaxEnergy != 12 {
		t.Fatalf("energy/max = %d/%d, want 2/12", got, p.MaxEnergy)
	}
	if err := UseDailyFreeRefill(p, now); err != nil {
		t.Fatal(err)
	}
	if p.Energy != 12 {
		t.Fatalf("free refill energy = %d", p.Energy)
	}
	if err := UseDailyFreeRefill(p, now); err == nil {
		t.Fatal("expected daily refill to be single-use")
	}
}

func TestBuyEnergyRechargePacks(t *testing.T) {
	now := time.Date(2026, time.November, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		amount     int
		currency   string
		marks      int64
		crowns     int64
		wantMarks  int64
		wantCrowns int64
	}{
		{"one for Marks", OneEnergyRecharge, "marks", 500, 0, 0, 0},
		{"one for Crowns", OneEnergyRecharge, "crowns", 0, 25, 0, 0},
		{"ten for Marks", TenEnergyRecharge, "marks", 4500, 0, 0, 0},
		{"ten for Crowns", TenEnergyRecharge, "crowns", 0, 225, 0, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p, err := profile.New(now)
			if err != nil {
				t.Fatal(err)
			}
			p.Energy = 0
			p.EnergyUpdatedAt = now
			p.Marks, p.Crowns = test.marks, test.crowns
			if err := BuyEnergy(p, test.amount, test.currency, now); err != nil {
				t.Fatal(err)
			}
			if p.Energy != test.amount || p.Marks != test.wantMarks || p.Crowns != test.wantCrowns {
				t.Fatalf("purchase state: energy=%d marks=%d crowns=%d",
					p.Energy, p.Marks, p.Crowns)
			}
		})
	}
}

func TestBuyEnergyRejectsInvalidOrUnusablePurchasesWithoutCharging(t *testing.T) {
	now := time.Now()
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.Energy = 5
	p.Marks, p.Crowns = 5000, 10
	for _, test := range []struct {
		amount   int
		currency string
	}{
		{10, "marks"},
		{1, "gems"},
		{1, "crowns"},
		{7, "marks"},
	} {
		if err := BuyEnergy(p, test.amount, test.currency, now); err == nil {
			t.Errorf("BuyEnergy(%d, %q) succeeded unexpectedly", test.amount, test.currency)
		}
	}
	if p.Energy != 5 || p.Marks != 5000 || p.Crowns != 10 {
		t.Fatalf("rejected purchase changed profile: energy=%d marks=%d crowns=%d",
			p.Energy, p.Marks, p.Crowns)
	}
}

func TestGrantEnergyCapsAtMaximum(t *testing.T) {
	now := time.Now()
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.Energy = p.MaxEnergy - 1
	if err := Grant(p, 10, now); err != nil {
		t.Fatal(err)
	}
	if p.Energy != p.MaxEnergy || !p.EnergyUpdatedAt.Equal(now.UTC()) {
		t.Fatalf("granted energy state = %d/%d at %s",
			p.Energy, p.MaxEnergy, p.EnergyUpdatedAt)
	}
}
