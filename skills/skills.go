package skills

import (
	"errors"
	"fmt"

	"prisonersofwar/profile"
)

type Effect string

const (
	EffectMarksBasePercent  Effect = "marks_base_percent"
	EffectModeMultiplier    Effect = "mode_multiplier"
	EffectFirstWinBonus     Effect = "first_win_bonus"
	EffectCampDrawThreshold Effect = "camp_draw_threshold"
	EffectRareRewardPercent Effect = "rare_reward_percent"
	EffectCurator           Effect = "curator"
	EffectMaxEnergy         Effect = "max_energy"
	EffectEnergyRechargePct Effect = "energy_recharge_percent"
	EffectDailyFreeRefill   Effect = "daily_free_refill"
)

type Node struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	CostCrowns   int64  `json:"cost_crowns"`
	Prerequisite string `json:"prerequisite,omitempty"`
	Effect       Effect `json:"effect"`
	Magnitude    int    `json:"magnitude"`
}

type Bonuses struct {
	MarksBasePercent      int
	ModeMultiplier        int
	FirstWinBonus         int
	CampDrawReduction     int
	RareRewardPercent     int
	MaxEnergy             int
	EnergyRechargePercent int
	DailyFreeRefill       bool
	Curator               bool
}

var tree = []Node{
	{ID: "quartermaster", Name: "Quartermaster", Description: "+10% base Marks on match payouts.", CostCrowns: 20, Effect: EffectMarksBasePercent, Magnitude: 10},
	{ID: "field-warden", Name: "Field Warden", Description: "+5% ranked mode payout multiplier.", CostCrowns: 25, Effect: EffectModeMultiplier, Magnitude: 5, Prerequisite: "quartermaster"},
	{ID: "first-blood", Name: "First Blood", Description: "+50% Marks for the first win each day.", CostCrowns: 20, Effect: EffectFirstWinBonus, Magnitude: 50},
	{ID: "camp-lore", Name: "Fortunes Favor", Description: "Camp Draws arrive after two fewer wins.", CostCrowns: 25, Effect: EffectCampDrawThreshold, Magnitude: 2},
	{ID: "fortune", Name: "Lucky Find", Description: "Shift 5 percentage points from the highest currency roll to rare chromas.", CostCrowns: 30, Effect: EffectRareRewardPercent, Magnitude: 5, Prerequisite: "camp-lore"},
	{ID: "curator", Name: "Curator", Description: "Reroll owned chromas once; a second duplicate becomes Marks.", CostCrowns: 40, Effect: EffectCurator, Magnitude: 1, Prerequisite: "fortune"},
	{ID: "expanded-rations", Name: "Expanded Rations", Description: "+2 maximum energy.", CostCrowns: 20, Effect: EffectMaxEnergy, Magnitude: 2},
	{ID: "quick-recovery", Name: "Quick Recovery", Description: "Energy recharges 10% faster.", CostCrowns: 25, Effect: EffectEnergyRechargePct, Magnitude: 10, Prerequisite: "expanded-rations"},
	{ID: "second-wind", Name: "Second Wind", Description: "One daily free energy refill.", CostCrowns: 30, Effect: EffectDailyFreeRefill, Magnitude: 1, Prerequisite: "quick-recovery"},
}

func AllNodes() []Node {
	return append([]Node(nil), tree...)
}

func FindNode(id string) (Node, bool) {
	for _, node := range tree {
		if node.ID == id {
			return node, true
		}
	}
	return Node{}, false
}

func AggregateBonuses(p *profile.Profile) Bonuses {
	var bonuses Bonuses
	if p == nil {
		return bonuses
	}
	for _, node := range tree {
		if !p.SkillNodes[node.ID] {
			continue
		}
		switch node.Effect {
		case EffectMarksBasePercent:
			bonuses.MarksBasePercent += node.Magnitude
		case EffectModeMultiplier:
			bonuses.ModeMultiplier += node.Magnitude
		case EffectFirstWinBonus:
			bonuses.FirstWinBonus += node.Magnitude
		case EffectCampDrawThreshold:
			bonuses.CampDrawReduction += node.Magnitude
		case EffectRareRewardPercent:
			bonuses.RareRewardPercent += node.Magnitude
		case EffectMaxEnergy:
			bonuses.MaxEnergy += node.Magnitude
		case EffectEnergyRechargePct:
			bonuses.EnergyRechargePercent += node.Magnitude
		case EffectDailyFreeRefill:
			bonuses.DailyFreeRefill = true
		case EffectCurator:
			bonuses.Curator = true
		}
	}
	return bonuses
}

func CampDrawThreshold(p *profile.Profile, baseThreshold int) int {
	threshold := baseThreshold - AggregateBonuses(p).CampDrawReduction
	if threshold < 1 {
		return 1
	}
	return threshold
}

func CanUnlock(p *profile.Profile, id string) (bool, string) {
	allowed, reason := canUnlockWithoutCurrency(p, id)
	if !allowed {
		return false, reason
	}
	node, _ := FindNode(id)
	if p.Crowns < node.CostCrowns {
		return false, "not enough Crowns"
	}
	return true, ""
}

func CanUnlockWithSkillPoint(p *profile.Profile, id string) (bool, string) {
	allowed, reason := canUnlockWithoutCurrency(p, id)
	if !allowed {
		return false, reason
	}
	if p.SkillPoints < 1 {
		return false, "not enough Skill Tree Points"
	}
	return true, ""
}

func canUnlockWithoutCurrency(p *profile.Profile, id string) (bool, string) {
	if p == nil {
		return false, "profile is required"
	}
	node, ok := FindNode(id)
	if !ok {
		return false, fmt.Sprintf("unknown skill node %q", id)
	}
	if p.SkillNodes[id] {
		return false, "skill is already unlocked"
	}
	if node.Prerequisite != "" && !p.SkillNodes[node.Prerequisite] {
		return false, fmt.Sprintf("requires %s", node.Prerequisite)
	}
	return true, ""
}

func UnlockNode(p *profile.Profile, id string) error {
	allowed, reason := CanUnlock(p, id)
	if !allowed {
		return errors.New(reason)
	}
	node, _ := FindNode(id)
	p.Crowns -= node.CostCrowns
	unlockNode(p, id)
	return nil
}

func UnlockNodeWithSkillPoint(p *profile.Profile, id string) error {
	allowed, reason := CanUnlockWithSkillPoint(p, id)
	if !allowed {
		return errors.New(reason)
	}
	p.SkillPoints--
	unlockNode(p, id)
	return nil
}

func unlockNode(p *profile.Profile, id string) {
	p.SkillNodes[id] = true
	bonuses := AggregateBonuses(p)
	p.MaxEnergy = profile.DefaultMaxEnergy + bonuses.MaxEnergy
	if p.MaxEnergy > profile.MaxEnergyCap {
		p.MaxEnergy = profile.MaxEnergyCap
	}
	recharge := profile.DefaultEnergyRecharge.Milliseconds()
	p.EnergyRechargeMs = recharge * 100 / int64(100+bonuses.EnergyRechargePercent)
	if p.Energy > p.MaxEnergy {
		p.Energy = p.MaxEnergy
	}
}
