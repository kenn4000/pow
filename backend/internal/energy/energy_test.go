package energy

import (
	"testing"
	"time"

	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

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
	p.Crowns = 100
	for _, id := range []string{"expanded-rations", "quick-recovery", "second-wind"} {
		if err := skills.UnlockNode(p, id); err != nil {
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
