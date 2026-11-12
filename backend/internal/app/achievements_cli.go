package app

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

func updateAchievementStats(p *profile.Profile, result game.MatchResult, mode economy.Mode) {
	updateAchievementStatsForGameMode(p, result, mode, "main", time.Now())
}

func updateAchievementStatsForGameMode(p *profile.Profile, result game.MatchResult, mode economy.Mode, gameMode string, now time.Time) {
	if p == nil {
		return
	}
	if p.AchievementCounters == nil {
		p.AchievementCounters = make(map[string]int)
	}
	if p.WinStreakByMode == nil {
		p.WinStreakByMode = make(map[int]int)
	}
	if now.IsZero() {
		now = time.Now()
	}
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
	if canonicalMode := achievements.NormalizeGameMode(gameMode); canonicalMode != "" && result.Won {
		p.AchievementCounters["mode_wins_"+canonicalMode]++
	}
	if result.Won {
		heroID := ""
		for _, matchGame := range result.Games {
			if matchGame.PlayerHeroID != "" {
				heroID = matchGame.PlayerHeroID
				break
			}
		}
		if heroID == "" {
			heroID = p.LastHeroPicked
		}
		if heroID != "" {
			p.AchievementCounters["hero_match_wins_total"]++
			p.AchievementCounters["hero_match_wins_"+heroID]++
		}
	}
	if mode == economy.ModeRanked || mode == economy.ModeFriendRanked {
		p.AchievementCounters["ranked_matches_played"]++
	}
	if mode == economy.ModeRanked || mode == economy.ModeFriendRanked || mode == economy.ModeFriendCasual {
		current := p.AchievementCounters["human_win_streak_current"]
		if result.Won {
			current++
		} else {
			current = 0
		}
		p.AchievementCounters["human_win_streak_current"] = current
		if current > p.AchievementCounters["human_win_streak_best"] {
			p.AchievementCounters["human_win_streak_best"] = current
		}
	}
	year, week := now.ISOWeek()
	weekKey := year*100 + week
	if p.AchievementCounters["weekly_match_week"] != weekKey {
		p.AchievementCounters["weekly_match_week"] = weekKey
		p.AchievementCounters["weekly_match_current"] = 0
	}
	p.AchievementCounters["weekly_match_current"]++
	if p.AchievementCounters["weekly_match_current"] > p.AchievementCounters["weekly_matches_best"] {
		p.AchievementCounters["weekly_matches_best"] = p.AchievementCounters["weekly_match_current"]
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
	if p.PendingBoxes == nil {
		p.PendingBoxes = make(map[string]int)
	}
	p.PendingBoxes["wooden"] += event.WoodenBoxes
	p.PendingBoxes["silver"] += event.SilverBoxes
	p.PendingBoxes["gold"] += event.GoldBoxes
	p.UndoMoveTokens += event.UndoMoveTokens
	p.RestartMissionTokens += event.RestartMissionTokens
	p.CampaignSkipTokens += event.CampaignSkipTokens
	p.ReplaceCardTokens += event.ReplaceCardTokens
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
	return evaluateAchievementContext(p, achievements.Context{
		Profile: p, Event: event, Now: time.Now(),
		UnlockedHeroCount: len(p.UnlockedHeroes), AcquisitionSource: source,
	})
}

func recordBoxOpened(p *profile.Profile) []achievements.UnlockEvent {
	if p == nil {
		return nil
	}
	if p.AchievementCounters == nil {
		p.AchievementCounters = make(map[string]int)
	}
	p.AchievementCounters["boxes_opened"]++
	return evaluateAchievementEvent(p, achievements.EventBoxOpened, "box")
}

func recordConsumableUsed(p *profile.Profile) []achievements.UnlockEvent {
	if p == nil {
		return nil
	}
	if p.AchievementCounters == nil {
		p.AchievementCounters = make(map[string]int)
	}
	p.AchievementCounters["consumables_used"]++
	return evaluateAchievementEvent(p, achievements.EventConsumableUsed, "consumable")
}

func achievementUnlockSummary(events []achievements.UnlockEvent) string {
	var message strings.Builder
	for _, event := range events {
		definition, ok := achievements.Find(event.AchievementID)
		if !ok {
			continue
		}
		fmt.Fprintf(&message, " Achievement unlocked: %s (%s).", definition.Name,
			achievementRewardDescription(
				event.Marks, event.Crowns, event.Badge, event.ChromaPick, event.HeroBoxes,
				event.WoodenBoxes, event.SilverBoxes, event.GoldBoxes, event.SkillPoints,
				event.UndoMoveTokens, event.RestartMissionTokens, event.CampaignSkipTokens,
				event.ReplaceCardTokens))
	}
	return message.String()
}

func evaluateCampaignCompletionAchievement(p *profile.Profile, campaignID string) []achievements.UnlockEvent {
	if p == nil {
		return nil
	}
	return evaluateAchievementContext(p, achievements.Context{
		Profile: p, Event: achievements.EventCampaignCompleted, Now: time.Now(),
		UnlockedHeroCount: len(p.UnlockedHeroes), CampaignID: campaignID,
	})
}

func evaluateAchievementContext(p *profile.Profile, ctx achievements.Context) []achievements.UnlockEvent {
	unlockEvents := achievements.EvaluateAll(ctx)
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
	if event.SkillPoints > 0 {
		fmt.Printf("+%d Skill Tree Point(s)\n", event.SkillPoints)
	}
	if event.WoodenBoxes > 0 {
		fmt.Printf("+%d Wooden Box(es)\n", event.WoodenBoxes)
	}
	if event.SilverBoxes > 0 {
		fmt.Printf("+%d Silver Box(es)\n", event.SilverBoxes)
	}
	if event.GoldBoxes > 0 {
		fmt.Printf("+%d Gold Box(es)\n", event.GoldBoxes)
	}
	if event.ChromaPick > 0 {
		fmt.Printf("+%d Pick-Any-Chroma token(s)\n", event.ChromaPick)
	}
	if event.HeroBoxes > 0 {
		fmt.Printf("+%d Hero Box(es)\n", event.HeroBoxes)
	}
	if event.UndoMoveTokens > 0 {
		fmt.Printf("+%d Undo token(s)\n", event.UndoMoveTokens)
	}
	if event.RestartMissionTokens > 0 {
		fmt.Printf("+%d Restart token(s)\n", event.RestartMissionTokens)
	}
	if event.CampaignSkipTokens > 0 {
		fmt.Printf("+%d Skip Mission token(s)\n", event.CampaignSkipTokens)
	}
	if event.ReplaceCardTokens > 0 {
		fmt.Printf("+%d Swap Card token(s)\n", event.ReplaceCardTokens)
	}
	if event.Badge != "" {
		fmt.Printf("Badge earned: %s\n", event.Badge)
	}
}

func (a *app) achievementsMenu(p *profile.Profile) error {
	for {
		fmt.Println("\n=== ACHIEVEMENTS ===")
		a.showRoleSwatches(p)
		for category := achievements.CatCampaign; category <= achievements.CatMeta; category++ {
			fmt.Printf("%d) %s\n", int(category)+1, categoryName(category))
		}
		fmt.Println("5) Sets")
		fmt.Println("0) Return")
		answer, err := a.ask("Choose a category or Sets: ")
		if err != nil {
			return err
		}
		answer = strings.TrimSpace(answer)
		if answer == "0" {
			return nil
		}
		if answer == "5" || strings.EqualFold(answer, "sets") {
			a.showAchievementSets(p)
			continue
		}
		var selection int
		if _, err := fmt.Sscanf(answer, "%d", &selection); err != nil || selection < 1 || selection > int(achievements.CatMeta)+1 {
			fmt.Println("Enter a listed category number, 5 for Sets, or 0 to return.")
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
				fmt.Printf(" — %s", achievementRewardDescription(
					tier.RewardMarks, tier.RewardCrowns, tier.RewardBadge, tier.RewardChromaPick,
					tier.RewardHeroBox, tier.RewardWoodenBoxes, tier.RewardSilverBoxes,
					tier.RewardGoldBoxes, tier.RewardSkillPoints, tier.RewardUndoTokens, tier.RewardRestartTokens,
					tier.RewardSkipTokens, tier.RewardSwapTokens))
				fmt.Println()
			}
		} else {
			fmt.Printf("  Reward: %s", achievementRewardDescription(
				achievement.RewardMarks, achievement.RewardCrowns, achievement.RewardBadge,
				achievement.RewardChromaPick, achievement.RewardHeroBox, achievement.RewardWoodenBoxes,
				achievement.RewardSilverBoxes, achievement.RewardGoldBoxes, achievement.RewardSkillPoints,
				achievement.RewardUndoTokens,
				achievement.RewardRestartTokens, achievement.RewardSkipTokens, achievement.RewardSwapTokens))
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
		return fmt.Sprintf("win at least 10 matches using %d different heroes", tier.Goal)
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
		state := "in progress"
		if completed == len(set.Members) {
			state = "complete"
		}
		fmt.Printf("%s: %d/%d achievements — %s (display grouping only)\n", set.Name, completed, len(set.Members), state)
	}
	fmt.Printf("Skill Tree Points: %d\n", p.SkillPoints)
}

func achievementProgress(p *profile.Profile, achievement achievements.Achievement) string {
	if len(achievement.Tiers) == 0 {
		if achievement.CounterID == "" {
			return "one-time"
		}
		progress := p.AchievementCounters[achievement.CounterID]
		return fmt.Sprintf("%d/%d", progress, achievement.Goal)
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

func categoryName(category achievements.Category) string {
	names := []string{"Campaigns", "Feats", "Per-Mode", "Meta"}
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
