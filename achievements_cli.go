package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"prisonersofwar/achievements"
	"prisonersofwar/catalog"
	"prisonersofwar/economy"
	"prisonersofwar/game"
	"prisonersofwar/profile"
)

func updateAchievementStats(p *profile.Profile, result game.MatchResult, mode economy.Mode) {
	p.TotalGames++
	modeID := economy.ModeID(mode)
	if result.Won {
		p.WinStreakByMode[modeID]++
		if p.WinStreakByMode[modeID] > p.BestWinStreakByMode[modeID] {
			p.BestWinStreakByMode[modeID] = p.WinStreakByMode[modeID]
		}
	} else {
		p.WinStreakByMode[modeID] = 0
	}
	if result.Won && (mode == economy.ModeCPUHard || mode == economy.ModeRanked) {
		p.HardCPUWins++
	}
	comeback := false
	for _, matchGame := range result.Games {
		if !matchGame.Won {
			continue
		}
		switch matchGame.PlayerSeat {
		case game.SeatX:
			p.WinsAsSeatX++
		case game.SeatY:
			p.WinsAsSeatY++
		}
		if matchGame.ScoreAfterR2 < matchGame.OpponentAfterR2 {
			comeback = true
		}
	}
	if comeback {
		p.ComebackWins++
	}
	if result.Tied {
		p.TotalMatchTies++
	}
	for _, matchGame := range result.Games {
		p.TotalClashes += matchGame.Clashes
		if matchGame.Clashes > p.MaxClashesInGame {
			p.MaxClashesInGame = matchGame.Clashes
		}
	}
}

func buildMatchRecord(ctx achievements.Context, mode economy.Mode, marks int64, now time.Time) profile.MatchRecord {
	bountySuit := ""
	if ctx.Result != nil && len(ctx.Result.Games) > 0 {
		bountySuit = achievements.SuitRoleID(ctx.Result.Games[len(ctx.Result.Games)-1].BountySuit)
	}
	return profile.MatchRecord{
		At:                  now,
		Mode:                economy.ModeID(mode),
		PlayerSeat:          ctx.PlayerSeat,
		PlayerScore:         ctx.Result.PlayerScore,
		OppScore:            ctx.Result.OpponentScore,
		Won:                 ctx.MatchWon,
		BountySuit:          bountySuit,
		PlayerPrisoners:     ctx.PlayerPrisoners,
		OppPrisoners:        ctx.OppPrisoners,
		PlayerPrisonerSuits: ctx.PlayerPrisonerSuits,
		Clashes:             ctx.Clashes,
		MarksEarned:         marks,
	}
}

func applyUnlockEvent(p *profile.Profile, event achievements.UnlockEvent) {
	p.Marks += event.Marks
	p.Crowns += event.Crowns
	p.SkillPoints += event.SkillPoints
	p.ChromaPickTokens += event.ChromaPick
	p.HeroBoxesAvailable += event.HeroBoxes
	if event.Badge != "" {
		alreadyOwned := false
		for _, badge := range p.Badges {
			if badge == event.Badge {
				alreadyOwned = true
				break
			}
		}
		if !alreadyOwned {
			p.Badges = append(p.Badges, event.Badge)
		}
	}
}

func evaluateAchievementEvent(p *profile.Profile, event achievements.EventKind, source string) []achievements.UnlockEvent {
	if p == nil {
		return nil
	}
	unlockEvents := achievements.EvaluateAll(achievements.Context{
		Profile: p, Event: event, Now: time.Now(),
		UnlockedHeroCount: len(p.UnlockedHeroes), AcquisitionSource: source,
	})
	for _, unlockEvent := range unlockEvents {
		applyUnlockEvent(p, unlockEvent)
	}
	return unlockEvents
}

func showUnlockToast(event achievements.UnlockEvent) {
	if event.SetID != "" && event.SkillPoints > 0 {
		setName := event.SetID
		for _, set := range achievements.AllSets() {
			if set.ID == event.SetID {
				setName = set.Name
				break
			}
		}
		fmt.Printf("\n*** Set Complete: %s ***\n+%d Skill Tree Point(s)\n", setName, event.SkillPoints)
		return
	}
	definition, ok := achievements.Find(event.AchievementID)
	if !ok {
		fmt.Printf("\n*** Achievement Unlocked: %s ***\n", event.AchievementID)
		return
	}
	name := definition.Name
	if event.Tier > 0 && event.Tier <= len(definition.Tiers) {
		name += " " + definition.Tiers[event.Tier-1].Name
	}
	fmt.Printf("\n*** Achievement Unlocked: %s ***\n", name)
	if event.Marks > 0 {
		fmt.Printf("+%d Marks\n", event.Marks)
	}
	if event.Crowns > 0 {
		fmt.Printf("+%d Crowns\n", event.Crowns)
	}
	if event.ChromaPick > 0 {
		fmt.Printf("+%d Pick-Any-Chroma token(s)\n", event.ChromaPick)
	}
	if event.HeroBoxes > 0 {
		fmt.Printf("+%d Hero Box(es)\n", event.HeroBoxes)
	}
	if event.Badge != "" {
		fmt.Printf("Badge earned: %s\n", event.Badge)
	}
}

func (a *app) achievementsMenu(p *profile.Profile) error {
	for {
		fmt.Println("\n=== ACHIEVEMENTS ===")
		a.showRoleSwatches(p)
		for category := achievements.CatOnboarding; category <= achievements.CatMisc; category++ {
			fmt.Printf("%d) %s\n", int(category)+1, categoryName(category))
		}
		fmt.Println("13) Sets")
		fmt.Println("0) Return")
		answer, err := a.ask("Choose a category or Sets: ")
		if err != nil {
			return err
		}
		answer = strings.TrimSpace(answer)
		if answer == "0" {
			return nil
		}
		if answer == "13" || strings.EqualFold(answer, "sets") {
			a.showAchievementSets(p)
			continue
		}
		var selection int
		if _, err := fmt.Sscanf(answer, "%d", &selection); err != nil || selection < 1 || selection > int(achievements.CatMisc)+1 {
			fmt.Println("Enter a listed category number, 13 for Sets, or 0 to return.")
			continue
		}
		a.showAchievementCategory(p, achievements.Category(selection-1))
	}
}

func (a *app) showAchievementCategory(p *profile.Profile, category achievements.Category) {
	fmt.Printf("\n--- %s ---\n", categoryName(category))
	a.showRoleSwatches(p)
	for _, achievement := range achievements.All() {
		if achievement.Category != category {
			continue
		}
		if achievement.Hidden && !achievements.IsUnlocked(p, achievement) {
			fmt.Println("??? | ??? | ???")
			continue
		}
		fmt.Printf("%s | %s | %s\n", achievement.Name, achievementProgress(p, achievement), achievementStatus(p, achievement))
		fmt.Printf("  %s\n", achievement.Description)
		if len(achievement.Tiers) > 0 {
			for _, tier := range achievement.Tiers {
				fmt.Printf("  Tier %s", tier.Name)
				if condition := achievementTierCondition(achievement, tier); condition != "" {
					fmt.Printf(" — %s", condition)
				}
				fmt.Printf(" — %d Marks, %d Crowns", tier.RewardMarks, tier.RewardCrowns)
				if tier.RewardChromaPick > 0 {
					fmt.Printf(", %d Pick-Any-Chroma token", tier.RewardChromaPick)
				}
				if tier.RewardHeroBox > 0 {
					fmt.Printf(", %d Hero Box", tier.RewardHeroBox)
				}
				fmt.Println()
			}
		} else {
			fmt.Printf("  Reward: %d Marks, %d Crowns", achievement.RewardMarks, achievement.RewardCrowns)
			if achievement.RewardHeroBox > 0 {
				fmt.Printf(", %d Hero Box", achievement.RewardHeroBox)
			}
			fmt.Println()
		}
	}
}

func achievementTierCondition(achievement achievements.Achievement, tier achievements.Tier) string {
	switch achievement.ID {
	case "bounty_connoisseur":
		return fmt.Sprintf("win with active Bounty = %+d", tier.Goal)
	case "favored_aficionado":
		return fmt.Sprintf("win with active Favored = %+d", tier.Goal)
	case "shift_whisperer":
		return fmt.Sprintf("win with active Shift = %+d", tier.Goal)
	case "hero_variety":
		return fmt.Sprintf("own at least %d non-Recruit heroes", tier.Goal)
	default:
		return ""
	}
}

func (a *app) showAchievementSets(p *profile.Profile) {
	fmt.Println("\n--- SETS ---")
	a.showRoleSwatches(p)
	for _, set := range achievements.AllSets() {
		completed := 0
		for _, id := range set.Members {
			definition, ok := achievements.Find(id)
			if ok && achievements.IsUnlocked(p, definition) {
				completed++
			}
		}
		state := "1 Skill Tree Point available on completion"
		if p.CompletedSets[set.ID] {
			state = "complete"
		}
		fmt.Printf("%s: %d/%d achievements — %s\n", set.Name, completed, len(set.Members), state)
	}
	fmt.Printf("Skill Tree Points: %d\n", p.SkillPoints)
}

func achievementProgress(p *profile.Profile, achievement achievements.Achievement) string {
	if len(achievement.Tiers) == 0 {
		if achievement.CounterID == "" {
			return "one-time"
		}
		goal := singleAchievementGoal(achievement.ID)
		progress := p.AchievementCounters[achievement.CounterID]
		return fmt.Sprintf("%d/%d", progress, goal)
	}
	if achievement.IndependentTiers {
		mask := p.AchievementTiers[achievement.ID]
		count := 0
		for tier := range achievement.Tiers {
			if mask&(1<<tier) != 0 {
				count++
			}
		}
		return fmt.Sprintf("%d/%d tiers", count, len(achievement.Tiers))
	}
	currentTier := p.AchievementTiers[achievement.ID]
	if currentTier >= len(achievement.Tiers) {
		return "all tiers complete"
	}
	next := achievement.Tiers[currentTier]
	if achievement.ID == "hero_variety" {
		unique := make(map[string]bool)
		for _, heroID := range p.UnlockedHeroes {
			if heroID != "" && heroID != "recruit" {
				unique[heroID] = true
			}
		}
		return fmt.Sprintf("%s: %d/%d", next.Name, len(unique), next.Goal)
	}
	if achievement.ID == "collector" {
		switch currentTier {
		case 0:
			return fmt.Sprintf("%s: %d/%d", next.Name, p.TotalChromasPurchased, next.Goal)
		case 1:
			return fmt.Sprintf("%s: %d/%d", next.Name, p.RareChromasPurchased, next.Goal)
		default:
			return fmt.Sprintf("%s: %d/%d", next.Name, p.SuperRareChromasPurchased, next.Goal)
		}
	}
	if achievement.ID == "sparse" {
		if currentTier == 0 {
			return "Match-based; achieve a sparse win"
		}
		return fmt.Sprintf("Tier %s complete; next: %s", achievement.Tiers[currentTier-1].Name, next.Name)
	}
	progress := p.AchievementCounters[achievement.CounterID]
	return fmt.Sprintf("%s: %d/%d", next.Name, progress, next.Goal)
}

func achievementStatus(p *profile.Profile, achievement achievements.Achievement) string {
	if achievements.IsUnlocked(p, achievement) {
		return "✓"
	}
	if achievement.Hidden {
		return "???"
	}
	return "—"
}

func singleAchievementGoal(id string) int {
	switch id {
	case "bounty_hunter":
		return 3
	case "clash_master":
		return 3
	case "long_march":
		return 100
	case "hard_way":
		return 10
	case "first_clash", "dead_heat", "streak_easy", "streak_medium", "streak_hard", "streak_ranked":
		return 1
	case "collector":
		return 3
	default:
		return 1
	}
}

func categoryName(category achievements.Category) string {
	names := []string{
		"Onboarding", "Strategy", "Skill", "Comeback", "Dedication", "Collection",
		"Retention", "Seat Mastery", "Streak", "Bounty", "Ties", "Miscellaneous",
	}
	if int(category) < 0 || int(category) >= len(names) {
		return "Unknown"
	}
	return names[category]
}

func (a *app) showRoleSwatches(p *profile.Profile) {
	fmt.Printf("Roles: %s Scissors  %s Paper  %s Rock\n",
		a.roleSwatch(game.Blade, p), a.roleSwatch(game.Fang, p), a.roleSwatch(game.Shield, p))
}

func (a *app) roleSwatch(suit game.Suit, p *profile.Profile) string {
	old := a.playerProfile
	a.playerProfile = p
	color := a.ansiSuitColor(suit)
	a.playerProfile = old
	if !colorsEnabled {
		return "[" + colorAssignment(p, achievements.SuitRoleID(suit)) + "]"
	}
	return color + "●\033[0m"
}

func updatePurchaseAchievements(p *profile.Profile, now time.Time) ([]achievements.UnlockEvent, error) {
	events := achievements.EvaluateAll(achievements.Context{
		Profile: p,
		Event:   achievements.EventPurchase,
		Now:     now,
	})
	for _, event := range events {
		applyUnlockEvent(p, event)
	}
	if err := p.Save(); err != nil {
		return nil, err
	}
	return events, nil
}

func processCampDrawAchievements(p *profile.Profile, now time.Time) ([]achievements.UnlockEvent, error) {
	events := achievements.EvaluateAll(achievements.Context{
		Profile: p,
		Event:   achievements.EventCampDrawOpened,
		Now:     now,
	})
	for _, event := range events {
		applyUnlockEvent(p, event)
	}
	if err := p.Save(); err != nil {
		return nil, err
	}
	return events, nil
}

func validateAchievementContext(ctx achievements.Context) error {
	if ctx.Profile == nil {
		return errors.New("profile is required")
	}
	return nil
}

func achievementCategoryProgress(p *profile.Profile, category achievements.Category) float64 {
	total, unlocked := 0, 0
	for _, achievement := range achievements.All() {
		if achievement.Category != category {
			continue
		}
		total++
		if achievements.IsUnlocked(p, achievement) {
			unlocked++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(unlocked) / float64(total)
}

func safeAchievementProgress(progress, goal int) int {
	if goal <= 0 {
		return 0
	}
	return int(math.Min(float64(progress), float64(goal)))
}

func redeemPickAny(p *profile.Profile, chromaID string) error {
	return catalog.RedeemChromaPick(p, chromaID)
}
