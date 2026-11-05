package skills

import (
	"embed"
	"errors"
	"fmt"
	"time"

	"prisonersofwar/internal/profile"
)

//go:embed icons/*.png
var IconFiles embed.FS

type Effect string

const (
	EffectMarksBasePercent        Effect = "marks_base_percent"
	EffectLossMarksPercent        Effect = "loss_marks_percent"
	EffectFirstWinBonus           Effect = "first_win_bonus"
	EffectSecondWinBonus          Effect = "second_win_bonus"
	EffectRationsReadiness        Effect = "rations_readiness"
	EffectDailyWinCrowns          Effect = "daily_win_crowns"
	EffectRankedStreak            Effect = "ranked_streak"
	EffectRankedPayoutPercent     Effect = "ranked_payout_percent"
	EffectMultiplayerMarksPercent Effect = "multiplayer_marks_percent"
	EffectRankedBestOfThreeMarks  Effect = "ranked_best_of_three_marks"
	EffectCampaignMarks           Effect = "campaign_marks_percent"
	EffectFirstModeWinMarks       Effect = "first_mode_win_marks"
	EffectCampDrawThreshold       Effect = "camp_draw_threshold"
	EffectCampDrawReroll          Effect = "camp_draw_reroll"
	EffectCampDrawExtraCard       Effect = "camp_draw_extra_card"
	EffectMaxEnergy               Effect = "max_energy"
	EffectEnergyRechargePct       Effect = "energy_recharge_percent"
	EffectDailyFreeRefill         Effect = "daily_free_refill"
	EffectEnergyOverflow          Effect = "energy_overflow"
	EffectConsumableDiscount      Effect = "consumable_discount_percent"
	EffectHeroBoxDiscount         Effect = "hero_box_discount_percent"
	EffectChestDiscount           Effect = "chest_discount_percent"
	EffectDuplicateBonus          Effect = "duplicate_bonus_percent"
	EffectConductor               Effect = "conductor"
)

type Node struct {
	ID              string `json:"id"`
	Branch          string `json:"branch"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Tier            int    `json:"tier"`
	CostSkillPoints int    `json:"cost_skill_points"`
	Prerequisite    string `json:"prerequisite,omitempty"`
	Effect          Effect `json:"effect"`
	Magnitude       int    `json:"magnitude"`
}

type Bonuses struct {
	MarksBasePercent          int
	LossMarksPercent          int
	FirstWinBonus             int
	SecondWinBonus            int
	RationsReadiness          bool
	DailyWinCrowns            int
	RankedStreakCrowns        int
	RankedPayoutPercent       int
	MultiplayerMarksPercent   int
	RankedBestOfThreeMarks    int
	CampaignMarksPercent      int
	FirstModeWinMarks         int
	CampDrawReduction         int
	CampDrawReroll            bool
	CampDrawExtraCards        int
	MaxEnergy                 int
	EnergyRechargePercent     int
	DailyFreeRefill           bool
	EnergyOverflow            bool
	ConsumableDiscountPercent int
	HeroBoxDiscountPercent    int
	ChestDiscountPercent      int
	DuplicateBonusPercent     int
	ConductorLevel            int
	HeroHistory               bool
	HeroPreview               bool
}

var tree = []Node{
	{ID: "first-blood", Branch: "daily-wins", Name: "First Blood", Description: "+50% Marks on your first win each day.", Tier: 1, CostSkillPoints: 1, Effect: EffectFirstWinBonus, Magnitude: 50},
	{ID: "daily-double", Branch: "daily-wins", Name: "Daily Double", Description: "+50% Marks on your second win each day.", Tier: 2, CostSkillPoints: 2, Prerequisite: "first-blood", Effect: EffectSecondWinBonus, Magnitude: 50},
	{ID: "rations-readiness", Branch: "daily-wins", Name: "Rations & Readiness", Description: "Your first campaign win each day grants +1 Restart token, +1 Undo token, and +1 Energy.", Tier: 2, CostSkillPoints: 2, Prerequisite: "first-blood", Effect: EffectRationsReadiness, Magnitude: 1},
	{ID: "daily-triumph", Branch: "daily-wins", Name: "Daily Triumph", Description: "+5 Crowns on your first multiplayer win each day.", Tier: 3, CostSkillPoints: 3, Prerequisite: "daily-double", Effect: EffectDailyWinCrowns, Magnitude: 5},

	{ID: "streak", Branch: "multiplayer", Name: "Streak", Description: "+3 Crowns for every 3 consecutive ranked match wins across modes; a loss resets the streak.", Tier: 1, CostSkillPoints: 1, Effect: EffectRankedStreak, Magnitude: 3},
	{ID: "field-warden", Branch: "multiplayer", Name: "Field Warden", Description: "+15% ranked mode payout multiplier.", Tier: 2, CostSkillPoints: 2, Prerequisite: "streak", Effect: EffectRankedPayoutPercent, Magnitude: 15},
	{ID: "mark-of-victory", Branch: "multiplayer", Name: "Mark of Victory", Description: "+20% Marks on any multiplayer win.", Tier: 2, CostSkillPoints: 2, Prerequisite: "streak", Effect: EffectMultiplayerMarksPercent, Magnitude: 20},
	{ID: "bounty-hunter", Branch: "multiplayer", Name: "Bounty Hunter", Description: "+250 flat Marks on a ranked best-of-3 match win.", Tier: 3, CostSkillPoints: 3, Prerequisite: "mark-of-victory", Effect: EffectRankedBestOfThreeMarks, Magnitude: 250},

	{ID: "quartermaster", Branch: "general-rewards", Name: "Quartermaster", Description: "+10% base Marks on match payouts.", Tier: 1, CostSkillPoints: 1, Effect: EffectMarksBasePercent, Magnitude: 10},
	{ID: "salvager", Branch: "general-rewards", Name: "Salvager", Description: "+15% Marks on losses.", Tier: 1, CostSkillPoints: 1, Effect: EffectLossMarksPercent, Magnitude: 15},
	{ID: "curator", Branch: "general-rewards", Name: "Curator", Description: "Campaign missions give +50% Marks.", Tier: 2, CostSkillPoints: 2, Prerequisite: "quartermaster", Effect: EffectCampaignMarks, Magnitude: 50},
	{ID: "bounty-board", Branch: "general-rewards", Name: "Bounty Board", Description: "Your first win in each mode each day grants +50 Marks.", Tier: 2, CostSkillPoints: 2, Prerequisite: "salvager", Effect: EffectFirstModeWinMarks, Magnitude: 50},

	{ID: "expanded-rations", Branch: "energy", Name: "Expanded Rations", Description: "+2 maximum Energy.", Tier: 1, CostSkillPoints: 1, Effect: EffectMaxEnergy, Magnitude: 2},
	{ID: "quick-recovery", Branch: "energy", Name: "Quick Recovery", Description: "Energy recharges 10% faster.", Tier: 2, CostSkillPoints: 2, Prerequisite: "expanded-rations", Effect: EffectEnergyRechargePct, Magnitude: 10},
	{ID: "stockpile", Branch: "energy", Name: "Stockpile", Description: "+4 maximum Energy.", Tier: 2, CostSkillPoints: 2, Prerequisite: "expanded-rations", Effect: EffectMaxEnergy, Magnitude: 4},
	{ID: "second-wind", Branch: "energy", Name: "Second Wind", Description: "One daily free Energy refill.", Tier: 3, CostSkillPoints: 3, Prerequisite: "quick-recovery", Effect: EffectDailyFreeRefill, Magnitude: 1},
	{ID: "overflow", Branch: "energy", Name: "Overflow", Description: "Energy regenerates while offline, up to 2x your maximum Energy.", Tier: 3, CostSkillPoints: 3, Prerequisite: "stockpile", Effect: EffectEnergyOverflow, Magnitude: 2},

	{ID: "thrifty-quartermaster", Branch: "collection", Name: "Thrifty Quartermaster", Description: "Consumables cost 10% fewer Marks.", Tier: 1, CostSkillPoints: 1, Effect: EffectConsumableDiscount, Magnitude: 10},
	{ID: "hero-broker", Branch: "collection", Name: "Hero Broker", Description: "Hero Boxes cost 10% fewer Marks.", Tier: 2, CostSkillPoints: 2, Prerequisite: "thrifty-quartermaster", Effect: EffectHeroBoxDiscount, Magnitude: 10},
	{ID: "veterans-discount", Branch: "collection", Name: "Veteran's Discount", Description: "Shop chests cost 5% fewer Marks.", Tier: 2, CostSkillPoints: 2, Prerequisite: "thrifty-quartermaster", Effect: EffectChestDiscount, Magnitude: 5},
	{ID: "lucky-duplicate", Branch: "collection", Name: "Lucky Duplicate", Description: "Duplicate hero Crown refunds increase by 20%.", Tier: 2, CostSkillPoints: 2, Prerequisite: "thrifty-quartermaster", Effect: EffectDuplicateBonus, Magnitude: 20},
	{ID: "premium-duplicate", Branch: "collection", Name: "Premium Duplicate", Description: "Duplicate hero Crown refunds increase by an additional 30% (stacks with Lucky Duplicate for +50% total).", Tier: 3, CostSkillPoints: 3, Prerequisite: "lucky-duplicate", Effect: EffectDuplicateBonus, Magnitude: 30},

	{ID: "camp-lore", Branch: "camp", Name: "Camp Lore", Description: "Camp Draws arrive after two fewer wins.", Tier: 1, CostSkillPoints: 1, Effect: EffectCampDrawThreshold, Magnitude: 2},
	{ID: "reroll", Branch: "camp", Name: "Reroll", Description: "Once per Camp Draw, reroll the prize and keep the higher-ranked reward.", Tier: 2, CostSkillPoints: 2, Prerequisite: "camp-lore", Effect: EffectCampDrawReroll, Magnitude: 1},
	{ID: "loaded-deck", Branch: "camp", Name: "Loaded Deck", Description: "Draw one extra card during the Camp Draw minigame.", Tier: 2, CostSkillPoints: 2, Prerequisite: "camp-lore", Effect: EffectCampDrawExtraCard, Magnitude: 1},
	{ID: "conductor-1", Branch: "camp", Name: "The Conductor", Description: "Tune one modifier by +1 or -1 at hero selection.", Tier: 2, CostSkillPoints: 2, Prerequisite: "camp-lore", Effect: EffectConductor, Magnitude: 1},
	{ID: "conductor-2", Branch: "camp", Name: "Master Conductor", Description: "Tune one modifier by up to +2 or -2 at hero selection.", Tier: 3, CostSkillPoints: 3, Prerequisite: "conductor-1", Effect: EffectConductor, Magnitude: 2},
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
		case EffectLossMarksPercent:
			bonuses.LossMarksPercent += node.Magnitude
		case EffectFirstWinBonus:
			bonuses.FirstWinBonus += node.Magnitude
		case EffectSecondWinBonus:
			bonuses.SecondWinBonus += node.Magnitude
		case EffectRationsReadiness:
			bonuses.RationsReadiness = true
		case EffectDailyWinCrowns:
			bonuses.DailyWinCrowns += node.Magnitude
		case EffectRankedStreak:
			bonuses.RankedStreakCrowns += node.Magnitude
		case EffectRankedPayoutPercent:
			bonuses.RankedPayoutPercent += node.Magnitude
		case EffectMultiplayerMarksPercent:
			bonuses.MultiplayerMarksPercent += node.Magnitude
		case EffectRankedBestOfThreeMarks:
			bonuses.RankedBestOfThreeMarks += node.Magnitude
		case EffectCampaignMarks:
			bonuses.CampaignMarksPercent += node.Magnitude
		case EffectFirstModeWinMarks:
			bonuses.FirstModeWinMarks += node.Magnitude
		case EffectCampDrawThreshold:
			bonuses.CampDrawReduction += node.Magnitude
		case EffectCampDrawReroll:
			bonuses.CampDrawReroll = true
		case EffectCampDrawExtraCard:
			bonuses.CampDrawExtraCards += node.Magnitude
		case EffectMaxEnergy:
			bonuses.MaxEnergy += node.Magnitude
		case EffectEnergyRechargePct:
			bonuses.EnergyRechargePercent += node.Magnitude
		case EffectDailyFreeRefill:
			bonuses.DailyFreeRefill = true
		case EffectEnergyOverflow:
			bonuses.EnergyOverflow = true
		case EffectConsumableDiscount:
			bonuses.ConsumableDiscountPercent += node.Magnitude
		case EffectHeroBoxDiscount:
			bonuses.HeroBoxDiscountPercent += node.Magnitude
		case EffectChestDiscount:
			bonuses.ChestDiscountPercent += node.Magnitude
		case EffectDuplicateBonus:
			bonuses.DuplicateBonusPercent += node.Magnitude
		case EffectConductor:
			if node.Magnitude > bonuses.ConductorLevel {
				bonuses.ConductorLevel = node.Magnitude
			}
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

func CanUnlockWithSkillPoint(p *profile.Profile, id string) (bool, string) {
	allowed, reason := canUnlockWithoutCurrency(p, id)
	if !allowed {
		return false, reason
	}
	node, _ := FindNode(id)
	if p.SkillPoints < node.CostSkillPoints {
		return false, fmt.Sprintf("need %d Skill Points", node.CostSkillPoints)
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

func UnlockNodeWithSkillPoint(p *profile.Profile, id string) error {
	allowed, reason := CanUnlockWithSkillPoint(p, id)
	if !allowed {
		return errors.New(reason)
	}
	node, _ := FindNode(id)
	p.ApplyEnergyTick(time.Now())
	p.SkillPoints -= node.CostSkillPoints
	if p.SkillNodes == nil {
		p.SkillNodes = make(map[string]bool)
	}
	p.SkillNodes[id] = true
	bonuses := AggregateBonuses(p)
	p.MaxEnergy = profile.DefaultMaxEnergy + bonuses.MaxEnergy
	if p.MaxEnergy > profile.MaxEnergyCap {
		p.MaxEnergy = profile.MaxEnergyCap
	}
	p.EnergyRechargeMs = profile.DefaultEnergyRecharge.Milliseconds() * 100 /
		int64(100+bonuses.EnergyRechargePercent)
	return nil
}
