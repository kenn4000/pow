package app

import (
	"fmt"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
)

type webBattleRecord struct {
	Round             int              `json:"round"`
	Exchange          int              `json:"exchange"`
	Attacker          string           `json:"attacker"`
	AttackerIndex     int              `json:"-"`
	Attack            webFaceCard      `json:"attack"`
	Defense           webFaceCard      `json:"defense"`
	AttackFaceHidden  bool             `json:"attackFaceHidden,omitempty"`
	AttackFaceRange   string           `json:"attackFaceRange,omitempty"`
	DefenseFaceHidden bool             `json:"defenseFaceHidden,omitempty"`
	DefenseFaceRange  string           `json:"defenseFaceRange,omitempty"`
	Outcome           webBattleOutcome `json:"outcome"`
	Scores            [2]int           `json:"scores"`
}

type webBoxReward struct {
	Label  string  `json:"label"`
	Chance float64 `json:"chance"`
}

func boxRewardTables(p *profile.Profile) map[string][]webBoxReward {
	tables := map[string][]webBoxReward{}
	for _, box := range []string{"wooden", "silver", "gold", "diamond"} {
		prizes, _ := lootbox.LootTableForProfile(box, p)
		total := 0
		for _, prize := range prizes {
			total += prize.Weight
		}
		for _, prize := range prizes {
			if prize.Weight <= 0 {
				continue
			}
			var label string
			switch prize.Type {
			case lootbox.PrizeMarks:
				label = fmt.Sprintf("%d Marks", prize.Amount)
			case lootbox.PrizeCrowns:
				label = fmt.Sprintf("%d Crowns", prize.Amount)
			case lootbox.PrizeEnergy:
				label = fmt.Sprintf("%d Energy", prize.Amount)
			case lootbox.PrizeChroma:
				if prize.Rarity == "seasonal_or_premium" {
					label = "random seasonal or premium skin"
				} else {
					label = "random " + prize.Rarity + " skin"
				}
			}
			if prize.BonusCrowns > 0 {
				label += fmt.Sprintf(" + %d Crown", prize.BonusCrowns)
				if prize.BonusCrowns != 1 {
					label += "s"
				}
			}
			tables[box] = append(tables[box], webBoxReward{Label: label, Chance: float64(prize.Weight) * 100 / float64(total)})
		}
	}
	for _, entry := range heroes.HeroBoxOdds(p) {
		tables["hero"] = append(tables["hero"], webBoxReward{Label: entry.Label, Chance: entry.Chance})
	}
	return tables
}

func achievementLevels(p *profile.Profile) map[string]int {
	levels := map[string]int{}
	if p == nil {
		return levels
	}
	for _, definition := range achievements.All() {
		levels[definition.ID] = p.AchievementTiers[definition.ID]
		if p.Achievements[definition.ID] && levels[definition.ID] == 0 {
			levels[definition.ID] = 1
		}
	}
	return levels
}

func (s *webServer) matchAchievementUpdates() []string {
	updates := []string{}
	if s.session.initialAchievements == nil {
		return updates
	}
	after := achievementLevels(s.playerProfile)
	for _, definition := range achievements.All() {
		before, level := s.session.initialAchievements[definition.ID], after[definition.ID]
		if level <= before {
			continue
		}
		if len(definition.Tiers) == 0 {
			updates = append(updates, definition.Name+" — "+achievementRewardDescription(
				definition.RewardMarks, definition.RewardCrowns, definition.RewardBadge,
				definition.RewardChromaPick, definition.RewardHeroBox, definition.RewardWoodenBoxes,
				definition.RewardSilverBoxes, definition.RewardGoldBoxes, definition.RewardSkillPoints,
				definition.RewardUndoTokens,
				definition.RewardRestartTokens, definition.RewardSkipTokens, definition.RewardSwapTokens))
			continue
		}
		for tier := before; tier < level && tier < len(definition.Tiers); tier++ {
			reward := definition.Tiers[tier]
			updates = append(updates, definition.Name+" · "+reward.Name+" — "+achievementRewardDescription(
				reward.RewardMarks, reward.RewardCrowns, reward.RewardBadge, reward.RewardChromaPick,
				reward.RewardHeroBox, reward.RewardWoodenBoxes, reward.RewardSilverBoxes,
				reward.RewardGoldBoxes, reward.RewardSkillPoints, reward.RewardUndoTokens, reward.RewardRestartTokens,
				reward.RewardSkipTokens, reward.RewardSwapTokens))
		}
	}
	return updates
}

func achievementCounter(p *profile.Profile, definition achievements.Achievement) (int, int, string) {
	if p == nil || definition.CounterID == "" {
		return 0, 0, ""
	}
	return p.AchievementCounters[definition.CounterID], definition.Goal, definition.ProgressUnit
}
