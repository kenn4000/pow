package energy

import (
	"errors"
	"fmt"
	"time"

	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

const (
	DailyAdLimit        = 5
	EnergyPerAd         = 3
	CrownsPerFullRefill = 100
)

func Current(p *profile.Profile, now time.Time) int {
	if p == nil {
		return 0
	}
	bonuses := skills.AggregateBonuses(p)
	p.MaxEnergy = profile.DefaultMaxEnergy + bonuses.MaxEnergy
	if p.MaxEnergy > profile.MaxEnergyCap {
		p.MaxEnergy = profile.MaxEnergyCap
	}
	p.EnergyRechargeMs = profile.DefaultEnergyRecharge.Milliseconds() * 100 /
		int64(100+bonuses.EnergyRechargePercent)
	resetDailyLimits(p, now)
	p.ApplyEnergyTick(now)
	return p.Energy
}

func Spend(p *profile.Profile, amount int, now time.Time) error {
	if p == nil {
		return errors.New("profile is required")
	}
	if amount <= 0 {
		return errors.New("energy cost must be positive")
	}
	Current(p, now)
	if p.Energy < amount {
		return fmt.Errorf("not enough energy: have %d, need %d", p.Energy, amount)
	}
	p.Energy -= amount
	if p.Energy < p.MaxEnergy && p.EnergyUpdatedAt.After(now) {
		p.EnergyUpdatedAt = now.UTC()
	}
	return nil
}

func WatchAd(p *profile.Profile, now time.Time) error {
	if p == nil {
		return errors.New("profile is required")
	}
	Current(p, now)
	if p.AdsWatchedToday >= DailyAdLimit {
		return fmt.Errorf("daily ad limit reached (%d/%d)", p.AdsWatchedToday, DailyAdLimit)
	}
	p.AdsWatchedToday++
	p.Energy += EnergyPerAd
	if p.Energy > p.MaxEnergy {
		p.Energy = p.MaxEnergy
	}
	p.EnergyUpdatedAt = now.UTC()
	return nil
}

func BuyFullRefill(p *profile.Profile, now time.Time) error {
	if p == nil {
		return errors.New("profile is required")
	}
	Current(p, now)
	if p.Energy >= p.MaxEnergy {
		return errors.New("energy is already full")
	}
	if p.Crowns < CrownsPerFullRefill {
		return fmt.Errorf("not enough Crowns: need %d", CrownsPerFullRefill)
	}
	p.Crowns -= CrownsPerFullRefill
	p.Energy = p.MaxEnergy
	p.EnergyUpdatedAt = now.UTC()
	return nil
}

func UseDailyFreeRefill(p *profile.Profile, now time.Time) error {
	if p == nil {
		return errors.New("profile is required")
	}
	Current(p, now)
	if !skills.AggregateBonuses(p).DailyFreeRefill {
		return errors.New("daily free refill skill is not unlocked")
	}
	if p.DailyFreeRefillUsed {
		return errors.New("daily free refill already used")
	}
	p.DailyFreeRefillUsed = true
	p.Energy = p.MaxEnergy
	p.EnergyUpdatedAt = now.UTC()
	return nil
}

func TimeUntilNext(p *profile.Profile, now time.Time) time.Duration {
	if p == nil || p.Energy >= p.MaxEnergy {
		return 0
	}
	if p.EnergyUpdatedAt.IsZero() {
		return 0
	}
	recharge := time.Duration(p.EnergyRechargeMs) * time.Millisecond
	if recharge <= 0 {
		return 0
	}
	next := p.EnergyUpdatedAt.Add(recharge)
	if !next.After(now) {
		return 0
	}
	return next.Sub(now)
}

func resetDailyLimits(p *profile.Profile, now time.Time) {
	localNow := now.In(time.Local)
	if p.AdsResetAt.IsZero() || !localNow.Before(p.AdsResetAt.In(time.Local)) {
		p.AdsWatchedToday = 0
		p.AdsResetAt = midnightTomorrow(localNow)
	}
	if p.DailyWinResetAt.IsZero() || !localNow.Before(p.DailyWinResetAt.In(time.Local)) {
		p.DailyWins = 0
		p.DailyWinResetAt = midnightTomorrow(localNow)
	}
	if p.FreeRefillResetAt.IsZero() || !localNow.Before(p.FreeRefillResetAt.In(time.Local)) {
		p.DailyFreeRefillUsed = false
		p.FreeRefillResetAt = midnightTomorrow(localNow)
	}
}

func midnightTomorrow(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}
