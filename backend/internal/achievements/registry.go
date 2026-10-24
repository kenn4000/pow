package achievements

import (
	"math"

	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

type EvalFunc func(ctx Context) (progress int, unlocked bool, tierAchieved int)

type UnlockEvent struct {
	AchievementID string
	Tier          int
	Marks         int64
	Crowns        int64
	SkillPoints   int
	ChromaPick    int
	HeroBoxes     int
	Badge         string
	SetID         string
}

var registry map[string]EvalFunc

func init() {
	registry = make(map[string]EvalFunc, len(achievements))
	register("sparse", evalSparse)
	register("efficient", evalEfficient)
	register("bounty_hunter", evalBountyHunter)
	register("bounty_denied", evalBountyDenied)
	register("bounty_decided", evalBountyDecided)
	register("aggressor", evalCounterTiersFrom(func(p *profile.Profile) int { return p.WinsAsSeatX }, []int{1, 10, 100}))
	register("defender", evalCounterTiersFrom(func(p *profile.Profile) int { return p.WinsAsSeatY }, []int{1, 10, 100}))
	register("streak_easy", evalStreak(economy.ModeIDCPUEasy))
	register("streak_medium", evalStreak(economy.ModeIDCPUNormal))
	register("streak_hard", evalStreak(economy.ModeIDCPUHard))
	register("streak_ranked", evalStreak(economy.ModeIDRanked))
	register("comeback", evalCounterTiersFrom(func(p *profile.Profile) int { return p.ComebackWins }, []int{1, 10, 50}))
	register("first_clash", evalCounter(func(p *profile.Profile) int { return p.TotalClashes }, 1))
	register("clash_master", evalCounter(func(p *profile.Profile) int { return p.MaxClashesInGame }, 3))
	register("dead_heat", evalCounter(func(p *profile.Profile) int { return p.TotalMatchTies }, 1))
	register("collector", evalCollector)
	register("long_march", evalCounter(func(p *profile.Profile) int { return p.TotalGames }, 100))
	register("hard_way", evalCounter(func(p *profile.Profile) int { return p.HardCPUWins }, 10))
	register("login_streak", evalCounterTiersFrom(func(p *profile.Profile) int { return p.LoginStreakBest }, []int{1, 7, 30}))
	register("first_blood", evalFirstBlood)
	register("both_sides", evalBothSides)
	register("the_cycle", evalTheCycle)
	register("master_both", evalMasterBoth)
	register("photo_finish", evalPhotoFinish)
	register("no_not_like_that", evalNoNotLikeThat)
	register("bounty_connoisseur", evalModifierTiers("bounty", map[int]int{-3: 1, -2: 2, -1: 3, 0: 4, 1: 5, 2: 6, 3: 7}))
	register("favored_aficionado", evalModifierTiers("favored", map[int]int{-1: 1, 0: 2, 1: 3, 2: 4}))
	register("shift_whisperer", evalModifierTiers("shift", map[int]int{0: 1, 1: 2, 2: 3, 3: 4}))
	register("hero_variety", evalHeroVariety)
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
	if ctx.Event == EventMatchEnd {
		updateSeatMasterStreaks(ctx)
	}
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
				AchievementID: achievement.ID,
				Marks:         achievement.RewardMarks,
				Crowns:        achievement.RewardCrowns,
				ChromaPick:    achievement.RewardChromaPick,
				HeroBoxes:     achievement.RewardHeroBox,
				Badge:         achievement.RewardBadge,
				SetID:         achievement.SetID,
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
				AchievementID: achievement.ID,
				Tier:          tierAchieved,
				Marks:         tier.RewardMarks,
				Crowns:        tier.RewardCrowns,
				ChromaPick:    tier.RewardChromaPick,
				HeroBoxes:     tier.RewardHeroBox,
				Badge:         tier.RewardBadge,
				SetID:         achievement.SetID,
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
			event.ChromaPick += tier.RewardChromaPick
			event.HeroBoxes += tier.RewardHeroBox
			if tier.RewardBadge != "" {
				event.Badge = tier.RewardBadge
			}
		}
		ctx.Profile.AchievementTiers[achievement.ID] = tierAchieved
		events = append(events, event)
	}
	for _, set := range sets {
		if ctx.Profile.CompletedSets[set.ID] || !setComplete(ctx.Profile, set) {
			continue
		}
		ctx.Profile.CompletedSets[set.ID] = true
		events = append(events, UnlockEvent{SetID: set.ID, SkillPoints: 1})
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

func evalSparse(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil {
		return 0, false, 0
	}
	progress, tier := 0, 0
	for _, matchGame := range ctx.Result.Games {
		if !matchGame.Won {
			continue
		}
		playerSuits := suitCount(matchGame.PlayerJail)
		combined := suitCount(append(append([]game.Card(nil), matchGame.PlayerJail...), matchGame.OpponentJail...))
		switch {
		case playerSuits == 2 && tier < 1:
			tier = 1
		case playerSuits == 1 && tier < 3:
			tier = 3
		}
		if combined == 2 && tier < 2 {
			tier = 2
		}
		if playerSuits > progress {
			progress = playerSuits
		}
	}
	if ctx.IsBestOfThree && ctx.MatchWon {
		wonGames := 0
		allWonGamesSparse := true
		for _, matchGame := range ctx.Result.Games {
			if !matchGame.Won {
				continue
			}
			wonGames++
			if suitCount(matchGame.PlayerJail) > 2 {
				allWonGamesSparse = false
			}
		}
		if wonGames > 0 && allWonGamesSparse {
			tier = 4
			progress = 4
		}
	}
	return progress, tier > 0, tier
}

func evalEfficient(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil {
		return 0, false, 0
	}
	maxPrisoners, tier := 0, 0
	for _, matchGame := range ctx.Result.Games {
		if !matchGame.Won {
			continue
		}
		count := len(matchGame.PlayerJail)
		if count > maxPrisoners {
			maxPrisoners = count
		}
		if count >= 7 && tier < 1 {
			tier = 1
		}
		if count >= 8 && tier < 2 {
			tier = 2
		}
		if count == 9 && matchGame.Clashes == 0 {
			tier = 3
		}
	}
	return maxPrisoners, tier > 0, tier
}

func evalBountyHunter(ctx Context) (int, bool, int) {
	return matchSingle(ctx, func(matchGame game.MatchGame) bool {
		return matchGame.Won && countBounty(matchGame) >= 3
	})
}

func evalBountyDenied(ctx Context) (int, bool, int) {
	return matchSingle(ctx, func(matchGame game.MatchGame) bool {
		return matchGame.Won && countBounty(matchGame) == 0
	})
}

func evalBountyDecided(ctx Context) (int, bool, int) {
	if ctx.Event != EventMatchEnd || ctx.Result == nil {
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
	if ctx.Event != EventHeroAcquired || ctx.Profile == nil {
		return 0, false, 0
	}
	unique := make(map[string]struct{}, len(ctx.Profile.UnlockedHeroes))
	for _, heroID := range ctx.Profile.UnlockedHeroes {
		if heroID != "" && heroID != "recruit" {
			unique[heroID] = struct{}{}
		}
	}
	count := len(unique)
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
