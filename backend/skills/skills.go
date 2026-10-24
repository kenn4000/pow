package skills

import (
	"errors"
	"fmt"

	"prisonersofwar/profile"
)

type Effect string

const (
	Conductor1Cost   int64 = 500
	Conductor2Cost   int64 = 1200
	HeroBrokerCost   int64 = 300
	CuratorCost      int64 = 400
	LuckyDupeCost    int64 = 400
	PremiumDupeCost  int64 = 800
	CampaignFreeCost int64 = 300
	DailyEnergyCost  int64 = 500
	HeroHistoryCost  int64 = 400
	HeroPreviewCost  int64 = 600
)

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
	EffectConductor         Effect = "conductor"
	EffectHeroBroker        Effect = "hero_broker"
	EffectCampaignMarks     Effect = "campaign_marks_percent"
	EffectLuckyDuplicate    Effect = "lucky_duplicate"
	EffectPremiumDuplicate  Effect = "premium_duplicate"
	EffectCampaignFree      Effect = "campaign_free"
	EffectDailyEnergy       Effect = "daily_energy"
	EffectHeroHistory       Effect = "hero_history"
	EffectHeroPreview       Effect = "hero_preview"
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
	ConductorLevel        int
	HeroBroker            bool
	CampaignMarksPercent  int
	DuplicateBonusPercent int
	CampaignFree          bool
	DailyEnergy           bool
	HeroHistory           bool
	HeroPreview           bool
}

var tree = []Node{
	{ID: "quartermaster", Name: "Quartermaster", Description: "+10% base Marks on match payouts.", CostCrowns: 20, Effect: EffectMarksBasePercent, Magnitude: 10},
	{ID: "field-warden", Name: "Field Warden", Description: "+5% ranked mode payout multiplier.", CostCrowns: 25, Effect: EffectModeMultiplier, Magnitude: 5, Prerequisite: "quartermaster"},
	{ID: "first-blood", Name: "First Blood", Description: "+50% Marks for the first win each day.", CostCrowns: 20, Effect: EffectFirstWinBonus, Magnitude: 50},
	{ID: "camp-lore", Name: "Fortunes Favor", Description: "Camp Draws arrive after two fewer wins.", CostCrowns: 25, Effect: EffectCampDrawThreshold, Magnitude: 2},
	{ID: "fortune", Name: "Lucky Find", Description: "Shift 5 percentage points from the highest currency roll to rare chromas.", CostCrowns: 30, Effect: EffectRareRewardPercent, Magnitude: 5, Prerequisite: "camp-lore"},
	{ID: "chroma-curator", Name: "Chroma Curator", Description: "Reroll owned chromas once; a second duplicate becomes Marks.", CostCrowns: 40, Effect: EffectCurator, Magnitude: 1, Prerequisite: "fortune"},
	{ID: "expanded-rations", Name: "Expanded Rations", Description: "+2 maximum energy.", CostCrowns: 20, Effect: EffectMaxEnergy, Magnitude: 2},
	{ID: "quick-recovery", Name: "Quick Recovery", Description: "Energy recharges 10% faster.", CostCrowns: 25, Effect: EffectEnergyRechargePct, Magnitude: 10, Prerequisite: "expanded-rations"},
	{ID: "second-wind", Name: "Second Wind", Description: "One daily free energy refill.", CostCrowns: 30, Effect: EffectDailyFreeRefill, Magnitude: 1, Prerequisite: "quick-recovery"},
	{ID: "conductor_1", Name: "The Conductor", Description: "Tune one modifier by +1 or -1 at hero selection.", CostCrowns: Conductor1Cost, Effect: EffectConductor, Magnitude: 1},
	{ID: "conductor_2", Name: "Master Conductor", Description: "Tune two different modifiers by +1 or -1 at hero selection.", CostCrowns: Conductor2Cost, Prerequisite: "conductor_1", Effect: EffectConductor, Magnitude: 2},
	{ID: "hero_broker", Name: "Hero Broker", Description: "Hero Boxes cost 10% fewer Marks.", CostCrowns: HeroBrokerCost, Effect: EffectHeroBroker, Magnitude: 1},
	{ID: "curator", Name: "Curator", Description: "Campaign missions give 50% more Marks.", CostCrowns: CuratorCost, Effect: EffectCampaignMarks, Magnitude: 50},
	{ID: "lucky_dupe", Name: "Lucky Duplicate", Description: "Duplicate hero Crowns increase by 25%.", CostCrowns: LuckyDupeCost, Effect: EffectLuckyDuplicate, Magnitude: 25},
	{ID: "premium_dupe", Name: "Premium Duplicate", Description: "Duplicate hero Crowns increase by 50%.", CostCrowns: PremiumDupeCost, Prerequisite: "lucky_dupe", Effect: EffectPremiumDuplicate, Magnitude: 50},
	{ID: "campaign_free", Name: "Campaign Freebie", Description: "The first mission in each campaign costs no energy.", CostCrowns: CampaignFreeCost, Effect: EffectCampaignFree, Magnitude: 1},
	{ID: "daily_energy", Name: "Daily Rations", Description: "Gain 2 energy on your first login each day.", CostCrowns: DailyEnergyCost, Effect: EffectDailyEnergy, Magnitude: 2},
	{ID: "hero_history", Name: "Hero History", Description: "See your opponent's last two hero picks.", CostCrowns: HeroHistoryCost, Effect: EffectHeroHistory, Magnitude: 2},
	{ID: "hero_preview", Name: "Hero Preview", Description: "Preview combined modifiers before locking in your hero.", CostCrowns: HeroPreviewCost, Prerequisite: "hero_history", Effect: EffectHeroPreview, Magnitude: 1},
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
		case EffectConductor:
			if node.Magnitude > bonuses.ConductorLevel {
				bonuses.ConductorLevel = node.Magnitude
			}
		case EffectHeroBroker:
			bonuses.HeroBroker = true
		case EffectCampaignMarks:
			bonuses.CampaignMarksPercent += node.Magnitude
		case EffectLuckyDuplicate:
			if node.Magnitude > bonuses.DuplicateBonusPercent {
				bonuses.DuplicateBonusPercent = node.Magnitude
			}
		case EffectPremiumDuplicate:
			if node.Magnitude > bonuses.DuplicateBonusPercent {
				bonuses.DuplicateBonusPercent = node.Magnitude
			}
		case EffectCampaignFree:
			bonuses.CampaignFree = true
		case EffectDailyEnergy:
			bonuses.DailyEnergy = true
		case EffectHeroHistory:
			bonuses.HeroHistory = true
		case EffectHeroPreview:
			bonuses.HeroPreview = true
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
