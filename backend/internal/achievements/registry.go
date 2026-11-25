package achievements

import (
	"math"
	"strings"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

type EvalFunc func(ctx Context) (progress int, unlocked bool, tierAchieved int)

type UnlockEvent struct {
	AchievementID        string
	Tier                 int
	Marks                int64
	Crowns               int64
	SkillPoints          int
	ChromaPick           int
	HeroBoxes            int
	WoodenBoxes          int
	SilverBoxes          int
	GoldBoxes            int
	UndoMoveTokens       int
	RestartMissionTokens int
	CampaignSkipTokens   int
	ReplaceCardTokens    int
	Badge                string
	SetID                string
}

var registry map[string]EvalFunc

func init() {
	registry = make(map[string]EvalFunc, len(achievements))
	register("sparse", evalSparse)
	register("efficient", evalEfficient)
	register("bounty_hunter", evalBountyHunter)
	register("bounty_denied", evalBountyDenied)
	register("bounty_decided", evalBountyDecided)
	register("comeback", evalComebackFeat)
	register("clash_master", evalCounter(func(p *profile.Profile) int { return p.MaxClashesInGame }, 3))
	register("dead_heat", evalCounter(func(p *profile.Profile) int { return p.TotalMatchTies }, 1))
	register("photo_finish", evalPhotoFinish)
	register("chroma_collector", evalChromaCollector)
	register("hero_variety", evalHeroVariety)
	register("branch_master", evalBranchMaster)
	register("mode_tourist", evalModeTourist)
	for _, achievement := range achievements {
		switch {
		case len(achievement.CampaignIDs) > 0:
			register(achievement.ID, evalCampaignCompletion(achievement.CampaignIDs...))
		case achievement.ModeID != "":
			goals := make([]int, len(achievement.Tiers))
			for i, tier := range achievement.Tiers {
				goals[i] = tier.Goal
			}
			register(achievement.ID, evalStoredCounterTiers(achievement.CounterID, goals))
		case len(achievement.Tiers) > 0 && registry[achievement.ID] == nil:
			goals := make([]int, len(achievement.Tiers))
			for i, tier := range achievement.Tiers {
				goals[i] = tier.Goal
			}
			switch achievement.ID {
			case "login_streak":
				register(achievement.ID, evalCounterTiersFrom(func(p *profile.Profile) int { return p.LoginStreakBest }, goals))
			case "long_march":
				register(achievement.ID, evalCounterTiersFrom(func(p *profile.Profile) int { return p.TotalWins }, goals))
			default:
				register(achievement.ID, evalStoredCounterTiers(achievement.CounterID, goals))
			}
		}
	}
}

func evalCampaignCompletion(campaignIDs ...string) EvalFunc {
	return func(ctx Context) (int, bool, int) {
		if ctx.Profile == nil || (ctx.Event != EventCampaignCompleted && ctx.Event != EventProfileLoad) {
			return 0, false, 0
		}
		triggered := ctx.Event == EventProfileLoad
		for _, campaignID := range campaignIDs {
			progress := ctx.Profile.Campaigns[campaignID]
			if progress == nil || !progress.Completed {
				return 0, false, 0
			}
			if ctx.CampaignID == campaignID {
				triggered = true
			}
		}
		if triggered {
			return 1, true, 1
		}
		return 0, false, 0
	}
}

func register(id string, evaluator EvalFunc) {
	registry[id] = evaluator
}

func EvalRegistry() map[string]EvalFunc {
	result := make(map[string]EvalFunc, len(registry))
	for id, evaluator := range registry {
		result[id] = evaluator
	}
	return result
}

func EvaluateAll(ctx Context) []UnlockEvent {
	if ctx.Profile == nil {
		return nil
	}
	ensureAchievementMaps(ctx.Profile)
	events := make([]UnlockEvent, 0)
	for _, achievement := range achievements {
		evaluator := registry[achievement.ID]
		progress, unlocked, tierAchieved := evaluator(ctx)
		updateCounter(ctx.Profile, achievement, progress)
		if len(achievement.Tiers) == 0 {
			if ctx.Profile.Achievements[achievement.ID] || !unlocked {
				continue
			}
			ctx.Profile.Achievements[achievement.ID] = true
			events = append(events, UnlockEvent{
				AchievementID:        achievement.ID,
				Marks:                achievement.RewardMarks,
				Crowns:               achievement.RewardCrowns,
				SkillPoints:          achievement.RewardSkillPoints,
				ChromaPick:           achievement.RewardChromaPick,
				HeroBoxes:            achievement.RewardHeroBox,
				WoodenBoxes:          achievement.RewardWoodenBoxes,
				SilverBoxes:          achievement.RewardSilverBoxes,
				GoldBoxes:            achievement.RewardGoldBoxes,
				UndoMoveTokens:       achievement.RewardUndoTokens,
				RestartMissionTokens: achievement.RewardRestartTokens,
				CampaignSkipTokens:   achievement.RewardSkipTokens,
				ReplaceCardTokens:    achievement.RewardSwapTokens,
				Badge:                achievement.RewardBadge,
				SetID:                achievement.SetID,
			})
			continue
		}
		if achievement.IndependentTiers {
			mask := ctx.Profile.AchievementTiers[achievement.ID]
			if tierAchieved <= 0 || tierAchieved > len(achievement.Tiers) {
				continue
			}
			bit := 1 << (tierAchieved - 1)
			if mask&bit != 0 {
				continue
			}
			tier := achievement.Tiers[tierAchieved-1]
			ctx.Profile.AchievementTiers[achievement.ID] = mask | bit
			events = append(events, UnlockEvent{
				AchievementID:        achievement.ID,
				Tier:                 tierAchieved,
				Marks:                tier.RewardMarks,
				Crowns:               tier.RewardCrowns,
				SkillPoints:          tier.RewardSkillPoints,
				ChromaPick:           tier.RewardChromaPick,
				HeroBoxes:            tier.RewardHeroBox,
				WoodenBoxes:          tier.RewardWoodenBoxes,
				SilverBoxes:          tier.RewardSilverBoxes,
				GoldBoxes:            tier.RewardGoldBoxes,
				UndoMoveTokens:       tier.RewardUndoTokens,
				RestartMissionTokens: tier.RewardRestartTokens,
				CampaignSkipTokens:   tier.RewardSkipTokens,
				ReplaceCardTokens:    tier.RewardSwapTokens,
				Badge:                tier.RewardBadge,
				SetID:                achievement.SetID,
			})
			continue
		}
		currentTier := ctx.Profile.AchievementTiers[achievement.ID]
		if currentTier >= len(achievement.Tiers) || tierAchieved <= currentTier {
			continue
		}
		event := UnlockEvent{AchievementID: achievement.ID, Tier: tierAchieved, SetID: achievement.SetID}
		for _, tier := range achievement.Tiers {
			if tier.Level <= currentTier || tier.Level > tierAchieved {
				continue
			}
			event.Marks += tier.RewardMarks
			event.Crowns += tier.RewardCrowns
			event.SkillPoints += tier.RewardSkillPoints
			event.ChromaPick += tier.RewardChromaPick
			event.HeroBoxes += tier.RewardHeroBox
			event.WoodenBoxes += tier.RewardWoodenBoxes
			event.SilverBoxes += tier.RewardSilverBoxes
			event.GoldBoxes += tier.RewardGoldBoxes
			event.UndoMoveTokens += tier.RewardUndoTokens
			event.RestartMissionTokens += tier.RewardRestartTokens
			event.CampaignSkipTokens += tier.RewardSkipTokens
			event.ReplaceCardTokens += tier.RewardSwapTokens
			if tier.RewardBadge != "" {
				event.Badge = tier.RewardBadge
			}
		}
		ctx.Profile.AchievementTiers[achievement.ID] = tierAchieved
		events = append(events, event)
	}
	return events
}

func ProgressFor(ctx Context, id string) (int, bool, int, bool) {
	_, ok := Find(id)
	if !ok {
		return 0, false, 0, false
	}
	evaluator := registry[id]
	if evaluator == nil {
		return 0, false, 0, false
	}
	progress, unlocked, tier := evaluator(ctx)
	return progress, unlocked, tier, true
}

func IsUnlocked(p *profile.Profile, achievement Achievement) bool {
	if p == nil {
		return false
	}
	if len(achievement.Tiers) == 0 {
		return p.Achievements[achievement.ID]
	}
	if achievement.IndependentTiers {
		completeMask := (1 << len(achievement.Tiers)) - 1
		return p.AchievementTiers[achievement.ID]&completeMask == completeMask
	}
	return p.AchievementTiers[achievement.ID] >= len(achievement.Tiers)
}

func updateCounter(p *profile.Profile, achievement Achievement, progress int) {
	if achievement.CounterID == "" {
		return
	}
	switch achievement.CounterType {
	case CounterStreak:
		p.AchievementCounters[achievement.CounterID] = progress
	case CounterCumulative, CounterBestRecord:
		if progress > p.AchievementCounters[achievement.CounterID] {
			p.AchievementCounters[achievement.CounterID] = progress
		}
	case CounterSingleMatch:
		p.AchievementCounters[achievement.CounterID] = progress
	}
}

func setComplete(p *profile.Profile, set Set) bool {
	for _, id := range set.Members {
		achievement, ok := Find(id)
		if !ok || !IsUnlocked(p, achievement) {
			return false
		}
	}
	return len(set.Members) > 0
}

func ensureAchievementMaps(p *profile.Profile) {
	if p.Achievements == nil {
		p.Achievements = make(map[string]bool)
	}
	if p.AchievementTiers == nil {
		p.AchievementTiers = make(map[string]int)
	}
	if p.AchievementCounters == nil {
		p.AchievementCounters = make(map[string]int)
	}
	if p.CompletedSets == nil {
		p.CompletedSets = make(map[string]bool)
	}
	if p.WinStreakByMode == nil {
		p.WinStreakByMode = make(map[int]int)
	}
	if p.MatchHistory == nil {
		p.MatchHistory = make([]profile.MatchRecord, 0)
	}
}

func updateSeatMasterStreaks(ctx Context) {
	p := ctx.Profile
	for _, seat := range []string{"X", "Y"} {
		key := "master_both_" + seat + "_streak"
		streak := 0
		if ctx.MatchWon && ctx.PlayerSeat == seat {
			streak = 1
			for _, record := range p.MatchHistory {
				if !record.Won || record.PlayerSeat != seat {
					break
				}
				streak++
			}
		}
		if streak > p.AchievementCounters[key] {
			p.AchievementCounters[key] = streak
		}
	}
}

func evalStoredCounterTiers(counterID string, goals []int) EvalFunc {
	return evalCounterTiersFrom(func(p *profile.Profile) int {
		return p.AchievementCounters[counterID]
	}, goals)
}

func evalChromaCollector(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	count := 0
	for id, owned := range ctx.Profile.OwnedChromas {
		if owned && id != "tribal-red" && id != "tribal-blue" && id != "tribal-yellow" {
			count++
		}
	}
	tier := tierFor(count, []int{5, 20, 50})
	return count, tier > 0, tier
}

func evalComebackFeat(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	progress := ctx.Profile.ComebackWins
	return progress, progress > 0, 0
}

func evalBranchMaster(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	branches := make(map[string]bool)
	for _, node := range skills.AllNodes() {
		if _, exists := branches[node.Branch]; !exists {
			branches[node.Branch] = true
		}
		if !ctx.Profile.SkillNodes[node.ID] {
			branches[node.Branch] = false
		}
	}
	count := 0
	for _, complete := range branches {
		if complete {
			count++
		}
	}
	tier := tierFor(count, []int{1, 3, 6})
	return count, tier > 0, tier
}

func evalModeTourist(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	count := 0
	for _, mode := range []string{"main", "fog", "abandon", "poker", "rummy", "puzzler", "ladder", "tug-war"} {
		if ctx.Profile.AchievementCounters["mode_wins_"+mode] > 0 {
			count++
		}
	}
	tier := tierFor(count, []int{2, 4, 7})
	return count, tier > 0, tier
}

func evalSparse(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil || !ctx.MatchWon {
		return 0, false, 0
	}
	progress := 0
	for _, matchGame := range ctx.Result.Games {
		if !matchGame.Won {
			continue
		}
		playerSuits := suitCount(matchGame.PlayerJail)
		if playerSuits == 2 {
			return 1, true, 0
		}
		if playerSuits > progress {
			progress = playerSuits
		}
	}
	return 0, false, 0
}

func evalEfficient(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil || !ctx.MatchWon {
		return 0, false, 0
	}
	maxPrisoners := 0
	for _, matchGame := range ctx.Result.Games {
		if !matchGame.Won {
			continue
		}
		count := len(matchGame.PlayerJail)
		if count > maxPrisoners {
			maxPrisoners = count
		}
		if count == 9 && matchGame.Clashes == 0 {
			return 1, true, 0
		}
	}
	return 0, false, 0
}

func evalBountyHunter(ctx Context) (int, bool, int) {
	if !ctx.MatchWon {
		return 0, false, 0
	}
	return matchSingle(ctx, func(matchGame game.MatchGame) bool {
		return matchGame.Won && countBounty(matchGame) >= 3
	})
}

func evalBountyDenied(ctx Context) (int, bool, int) {
	if !ctx.MatchWon {
		return 0, false, 0
	}
	return matchSingle(ctx, func(matchGame game.MatchGame) bool {
		return matchGame.Won && countBounty(matchGame) == 0
	})
}

func evalBountyDecided(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil || !ctx.MatchWon {
		return 0, false, 0
	}
	for _, matchGame := range ctx.Result.Games {
		if !matchGame.Won {
			continue
		}
		playerBreakdown := game.ScoreWithModifiers(matchGame.PlayerJail, matchGame.PlayerCapturedByRound, matchGame.BountySuit, matchGame.Modifiers)
		opponentScore := game.ScoreWithModifiers(matchGame.OpponentJail, matchGame.OpponentCapturedByRound, matchGame.BountySuit, matchGame.Modifiers).Total
		nonBountyScore := playerBreakdown.Total - playerBreakdown.BountyBonus
		if nonBountyScore <= opponentScore && playerBreakdown.Total > opponentScore {
			return 1, true, 0
		}
	}
	return 0, false, 0
}

func evalCounterTiersFrom(current func(*profile.Profile) int, goals []int) EvalFunc {
	return func(ctx Context) (int, bool, int) {
		if ctx.Profile == nil {
			return 0, false, 0
		}
		value := current(ctx.Profile)
		tier := tierFor(value, goals)
		return value, tier > 0, tier
	}
}

func evalStreak(mode int) EvalFunc {
	return func(ctx Context) (int, bool, int) {
		if ctx.Profile == nil {
			return 0, false, 0
		}
		progress := ctx.Profile.WinStreakByMode[mode]
		return progress, progress >= 3, 0
	}
}

func evalCounter(current func(*profile.Profile) int, goal int) EvalFunc {
	return func(ctx Context) (int, bool, int) {
		if ctx.Profile == nil {
			return 0, false, 0
		}
		progress := current(ctx.Profile)
		return progress, progress >= goal, 0
	}
}

func evalCollector(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	p := ctx.Profile
	tier := 0
	switch {
	case p.SuperRareChromasPurchased >= 3:
		tier = 3
	case p.RareChromasPurchased >= 3:
		tier = 2
	case p.TotalChromasPurchased >= 3:
		tier = 1
	}
	progress := p.TotalChromasPurchased
	if tier == 1 {
		progress = p.RareChromasPurchased
	} else if tier >= 2 {
		progress = p.SuperRareChromasPurchased
	}
	return progress, tier > 0, tier
}

func evalFirstBlood(ctx Context) (int, bool, int) {
	won := ctx.Event == EventMatchEnd && ctx.MatchWon && ctx.Profile != nil && ctx.Profile.TotalWins == 1
	if won {
		return 1, true, 0
	}
	return 0, false, 0
}

func evalBothSides(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	wonAsX, wonAsY := false, false
	for _, match := range ctx.Profile.MatchHistory {
		if !match.Won {
			continue
		}
		wonAsX = wonAsX || match.PlayerSeat == "X"
		wonAsY = wonAsY || match.PlayerSeat == "Y"
	}
	if ctx.Event == EventMatchEnd && ctx.MatchWon {
		wonAsX = wonAsX || ctx.PlayerSeat == "X"
		wonAsY = wonAsY || ctx.PlayerSeat == "Y"
	}
	progress := 0
	if wonAsX {
		progress++
	}
	if wonAsY {
		progress++
	}
	return progress, ctx.Event == EventMatchEnd && ctx.MatchWon && progress == 2, 0
}

func evalTheCycle(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil {
		return 0, false, 0
	}
	for _, matchGame := range ctx.Result.Games {
		if matchGame.Won && matchGame.SuitDoublingWin {
			return 1, true, 0
		}
	}
	return 0, false, 0
}

func evalMasterBoth(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	x := ctx.Profile.AchievementCounters["master_both_X_streak"]
	y := ctx.Profile.AchievementCounters["master_both_Y_streak"]
	progress := int(math.Min(float64(x), float64(y)))
	return progress, ctx.Event == EventMatchEnd && x >= 3 && y >= 3, 0
}

func evalPhotoFinish(ctx Context) (int, bool, int) {
	if ctx.Result == nil {
		return 0, false, 0
	}
	difference := ctx.Result.PlayerScore - ctx.Result.OpponentScore
	won := ctx.Event == EventMatchEnd && ctx.MatchWon && difference == 1
	if won {
		return 1, true, 0
	}
	return 0, false, 0
}

func evalNoNotLikeThat(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil {
		return 0, false, 0
	}
	for _, matchGame := range ctx.Result.Games {
		if matchGame.Won {
			continue
		}
		breakdown := game.ScoreWithModifiers(matchGame.PlayerJail, matchGame.PlayerCapturedByRound, matchGame.BountySuit, matchGame.Modifiers)
		for _, suitScore := range breakdown.BySuit {
			if suitScore >= 9 {
				return 1, true, 0
			}
		}
	}
	return 0, false, 0
}

func evalModifierTiers(modifier string, tierByValue map[int]int) EvalFunc {
	return func(ctx Context) (int, bool, int) {
		if ctx.Event != EventMatchEnd || !ctx.MatchWon || ctx.Result == nil {
			return 0, false, 0
		}
		for _, matchGame := range ctx.Result.Games {
			if !matchGame.Won {
				continue
			}
			value := 0
			switch modifier {
			case "bounty":
				value = matchGame.Modifiers.Bounty
			case "shift":
				value = matchGame.Modifiers.Shift
			case "favored":
				value = matchGame.Modifiers.Favored
			default:
				return 0, false, 0
			}
			if tier, ok := tierByValue[value]; ok {
				return value, true, tier
			}
		}
		return 0, false, 0
	}
}

func evalHeroVariety(ctx Context) (int, bool, int) {
	if ctx.Profile == nil {
		return 0, false, 0
	}
	unique := make(map[string]struct{}, len(ctx.Profile.UnlockedHeroes))
	for heroID, wins := range ctx.Profile.HeroWinCounts {
		if wins > 0 && heroID != "" {
			unique[heroID] = struct{}{}
		}
	}
	for counterID, wins := range ctx.Profile.AchievementCounters {
		if wins > 0 && strings.HasPrefix(counterID, "hero_match_wins_") && counterID != "hero_match_wins_total" {
			id := strings.TrimPrefix(counterID, "hero_match_wins_")
			if id != "" { unique[id] = struct{}{} }
		}
	}
	count := len(unique)
	totalWins := ctx.Profile.AchievementCounters["hero_match_wins_total"]
	if ctx.Profile.TotalWins > totalWins {
		totalWins = ctx.Profile.TotalWins
	}
	if totalWins < 10 {
		return count, false, 0
	}
	tier := 0
	switch {
	case count >= 10:
		tier = 3
	case count >= 6:
		tier = 2
	case count >= 3:
		tier = 1
	}
	return count, tier > 0, tier
}

func matchSingle(ctx Context, condition func(game.MatchGame) bool) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil {
		return 0, false, 0
	}
	for _, matchGame := range ctx.Result.Games {
		if condition(matchGame) {
			return 1, true, 0
		}
	}
	return 0, false, 0
}

func suitCount(cards []game.Card) int {
	suits := make(map[game.Suit]bool)
	for _, card := range cards {
		suits[card.Suit] = true
	}
	return len(suits)
}

func countBounty(matchGame game.MatchGame) int {
	count := 0
	for _, card := range matchGame.PlayerJail {
		if card.Suit == matchGame.BountySuit {
			count++
		}
	}
	return count
}

func tierFor(value int, goals []int) int {
	tier := 0
	for index, goal := range goals {
		if value < goal {
			break
		}
		tier = index + 1
	}
	return tier
}

func roleSuitTotal(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
