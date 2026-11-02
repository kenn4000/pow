package app

import (
	"fmt"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/economy"
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
	for _, box := range []string{"wooden", "silver", "gold"} {
		prizes, _ := lootbox.LootTableForProfile(box, p)
		total := 0
		for _, prize := range prizes {
			total += prize.Weight
		}
		for _, prize := range prizes {
			label := fmt.Sprintf("%d %s", prize.Amount, prize.Type)
			if prize.Type == lootbox.PrizeChroma {
				label = prize.Rarity + " chroma"
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
			updates = append(updates, definition.Name+" — "+achievementRewardDescription(definition.RewardMarks, definition.RewardCrowns, definition.RewardBadge, definition.RewardChromaPick, definition.RewardHeroBox))
			continue
		}
		for tier := before; tier < level && tier < len(definition.Tiers); tier++ {
			reward := definition.Tiers[tier]
			updates = append(updates, definition.Name+" · "+reward.Name+" — "+achievementRewardDescription(reward.RewardMarks, reward.RewardCrowns, reward.RewardBadge, reward.RewardChromaPick, reward.RewardHeroBox))
		}
	}
	return updates
}

func achievementCounter(p *profile.Profile, id string) (int, int, string) {
	switch id {
	case "long_march":
		return p.TotalGames, 100, "matches played"
	case "hard_way":
		return p.HardCPUWins, 10, "Hard CPU wins"
	case "first_clash":
		return p.TotalClashes, 1, "tied exchanges"
	case "clash_master":
		return p.MaxClashesInGame, 3, "clashes in one match (best)"
	case "dead_heat":
		return p.TotalMatchTies, 1, "equal-score matches"
	case "first_blood":
		return p.TotalWins, 1, "matches won"
	case "both_sides":
		count := 0
		if p.WinsAsSeatX > 0 {
			count++
		}
		if p.WinsAsSeatY > 0 {
			count++
		}
		return count, 2, "distinct seats won from"
	default:
		modes := map[string]int{"streak_easy": economy.ModeIDCPUEasy, "streak_medium": economy.ModeIDCPUNormal, "streak_hard": economy.ModeIDCPUHard, "streak_ranked": economy.ModeIDRanked}
		if mode, ok := modes[id]; ok {
			return p.WinStreakByMode[mode], 3, "consecutive wins in this mode"
		}
	}
	return 0, 0, ""
}
