package achievements

import (
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

func TestCatalogMatchesUpdatedAchievementGroups(t *testing.T) {
	counts := map[Category]int{}
	for _, achievement := range All() {
		counts[achievement.Category]++
	}
	if counts[CatCampaign] != 7 || counts[CatFeat] != 9 || counts[CatPerMode] != 8 || counts[CatMeta] != 11 {
		t.Fatalf("catalog counts = %v", counts)
	}
	for _, id := range []string{
		"camp_pow", "camp_abandon", "camp_ladder", "camp_poker", "camp_rummy", "camp_puzzler", "camp_tug_war",
		"sparse", "efficient", "bounty_hunter", "bounty_denied", "bounty_decided", "comeback",
		"clash_master", "dead_heat", "photo_finish", "mode_main", "mode_fog", "mode_abandon",
		"mode_poker", "mode_rummy", "mode_puzzler", "mode_ladder", "mode_tug-war",
		"chroma_collector", "lucky_draw", "win_streak_humans", "login_streak", "long_march",
		"hero_variety", "branch_master", "tinkerer", "marathon", "mode_tourist", "ranked_regular",
	} {
		if _, ok := Find(id); !ok {
			t.Errorf("missing achievement %q", id)
		}
	}

	mode, _ := Find("mode_main")
	if got := []int{mode.Tiers[0].Goal, mode.Tiers[1].Goal, mode.Tiers[2].Goal}; got[0] != 1 || got[1] != 10 || got[2] != 30 {
		t.Fatalf("per-mode goals = %v", got)
	}
	capstone := mode.Tiers[2]
	if capstone.RewardMarks != 1200 || capstone.RewardCrowns != 2 ||
		capstone.RewardSilverBoxes != 1 || capstone.RewardUndoTokens != 1 {
		t.Fatalf("Prisoners of War capstone = %+v", capstone)
	}
	for _, tier := range mode.Tiers {
		if tier.RewardMarks != int64(tier.Level*400) {
			t.Errorf("level %d marks = %d", tier.Level, tier.RewardMarks)
		}
	}
	for _, set := range AllSets() {
		if len(set.Members) == 0 {
			t.Errorf("empty display set %q", set.ID)
		}
	}
}

func TestEveryAchievementHasAnEvaluator(t *testing.T) {
	p := newAchievementProfile(t)
	ctx := Context{Profile: p, Event: EventProfileLoad}
	evaluators := EvalRegistry()
	for _, definition := range All() {
		evaluator, ok := evaluators[definition.ID]
		if !ok {
			t.Errorf("achievement %q has no evaluator", definition.ID)
			continue
		}
		_, unlocked, tier := evaluator(ctx)
		if unlocked || tier != 0 {
			t.Errorf("%s unexpectedly passed an empty profile: unlocked=%t tier=%d", definition.ID, unlocked, tier)
		}
	}
}

func TestFeatUnlocksGrantOneSkillPoint(t *testing.T) {
	nineCards := []game.Card{
		{Suit: game.Blade, Face: 1}, {Suit: game.Blade, Face: 2}, {Suit: game.Blade, Face: 3},
		{Suit: game.Fang, Face: 1}, {Suit: game.Fang, Face: 2}, {Suit: game.Fang, Face: 3},
		{Suit: game.Shield, Face: 1}, {Suit: game.Shield, Face: 2}, {Suit: game.Shield, Face: 3},
	}
	tests := []struct {
		id     string
		result game.MatchResult
	}{
		{"sparse", game.MatchResult{Won: true, Games: []game.MatchGame{{Won: true, PlayerJail: []game.Card{{Suit: game.Blade}, {Suit: game.Fang}}}}}},
		{"efficient", game.MatchResult{Won: true, Games: []game.MatchGame{{Won: true, PlayerJail: nineCards}}}},
		{"bounty_hunter", game.MatchResult{Won: true, Games: []game.MatchGame{{Won: true, BountySuit: game.Blade, PlayerJail: []game.Card{{Suit: game.Blade}, {Suit: game.Blade}, {Suit: game.Blade}}}}}},
		{"bounty_denied", game.MatchResult{Won: true, Games: []game.MatchGame{{Won: true, BountySuit: game.Blade, PlayerJail: []game.Card{{Suit: game.Fang}}}}}},
		{"bounty_decided", game.MatchResult{Won: true, Games: []game.MatchGame{{Won: true,
			PlayerJail: []game.Card{{Suit: game.Blade, Face: 1}}, OpponentJail: []game.Card{{Suit: game.Shield, Face: 1}},
			BountySuit: game.Blade, Modifiers: game.GameModifiers{Bounty: 1, Reckoning: [3]int{1, 1, 1}},
		}}}},
		{"photo_finish", game.MatchResult{Won: true, PlayerScore: 4, OpponentScore: 3}},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			p := newAchievementProfile(t)
			ctx := NewMatchContext(p, test.result, 0, "X")
			ctx.Event = EventMatchEnd
			events := EvaluateAll(ctx)
			event, unlocked := unlockFor(events, test.id)
			if !unlocked || !p.Achievements[test.id] || event.SkillPoints != 1 {
				t.Fatalf("achievement=%t reward=%+v events=%+v", p.Achievements[test.id], event, events)
			}
			if repeated := EvaluateAll(ctx); hasUnlock(repeated, test.id) {
				t.Fatalf("repeat evaluation awarded again: %+v", repeated)
			}
		})
	}

	p := newAchievementProfile(t)
	p.ComebackWins = 1
	p.MaxClashesInGame = 3
	p.TotalMatchTies = 1
	events := EvaluateAll(Context{Profile: p, Event: EventProfileLoad})
	for _, id := range []string{"comeback", "clash_master", "dead_heat"} {
		if !hasUnlock(events, id) {
			t.Errorf("%s did not unlock from lifetime stats: %+v", id, events)
		}
	}
	skillPointRewards := 0
	for _, event := range events {
		skillPointRewards += event.SkillPoints
	}
	if skillPointRewards != 3 {
		t.Fatalf("feat Skill Point rewards = %d, want 3", skillPointRewards)
	}
}

func TestCampaignCompletionAndTierRewardsAreIdempotent(t *testing.T) {
	p := newAchievementProfile(t)
	p.Campaigns["basic"] = &profile.CampaignProgress{Completed: true}
	events := EvaluateAll(Context{Profile: p, Event: EventCampaignCompleted, CampaignID: "basic"})
	campaign, ok := unlockFor(events, "camp_pow")
	if !ok || campaign.Marks != 3000 || campaign.Crowns != 10 {
		t.Fatalf("campaign reward = %+v; events %+v", campaign, events)
	}
	if repeated := EvaluateAll(Context{Profile: p, Event: EventCampaignCompleted, CampaignID: "basic"}); hasUnlock(repeated, "camp_pow") {
		t.Fatalf("campaign reward repeated: %+v", repeated)
	}

	p.AchievementCounters["mode_wins_main"] = 30
	events = EvaluateAll(Context{Profile: p, Event: EventProfileLoad})
	mode, ok := unlockFor(events, "mode_main")
	if !ok || mode.Tier != 3 || mode.Marks != 2400 || mode.Crowns != 3 ||
		mode.SilverBoxes != 1 || mode.UndoMoveTokens != 1 {
		t.Fatalf("mode rewards = %+v; events %+v", mode, events)
	}
	if repeated := EvaluateAll(Context{Profile: p, Event: EventProfileLoad}); hasUnlock(repeated, "mode_main") {
		t.Fatalf("mode rewards repeated: %+v", repeated)
	}
	if p.SkillPoints != 0 {
		t.Fatalf("display-set completion unexpectedly awarded Skill Points: %d", p.SkillPoints)
	}
}

func TestMetaProgressUsesOwnedItemsAndTrackedActions(t *testing.T) {
	p := newAchievementProfile(t)
	for id := range p.OwnedChromas {
		p.OwnedChromas[id] = true
	}
	for i := 0; i < 47; i++ {
		p.OwnedChromas[string(rune('a'+i))] = true
	}
	p.AchievementCounters["boxes_opened"] = 50
	p.AchievementCounters["consumables_used"] = 200
	p.AchievementCounters["weekly_matches_best"] = 60
	p.AchievementCounters["ranked_matches_played"] = 200
	p.AchievementCounters["human_win_streak_best"] = 12
	events := EvaluateAll(Context{Profile: p, Event: EventProfileLoad})
	var tokenRewards UnlockEvent
	for _, id := range []string{"chroma_collector", "lucky_draw", "tinkerer", "marathon", "ranked_regular", "win_streak_humans"} {
		event, ok := unlockFor(events, id)
		if !ok || event.Tier != 3 || event.Marks != 2400 || event.Crowns != 3 || event.GoldBoxes != 1 {
			t.Errorf("%s capstone = %+v, exists=%t", id, event, ok)
		}
		tokenRewards.UndoMoveTokens += event.UndoMoveTokens
		tokenRewards.RestartMissionTokens += event.RestartMissionTokens
		tokenRewards.CampaignSkipTokens += event.CampaignSkipTokens
		tokenRewards.ReplaceCardTokens += event.ReplaceCardTokens
	}
	if tokenRewards.CampaignSkipTokens < 2 || tokenRewards.ReplaceCardTokens < 1 ||
		tokenRewards.UndoMoveTokens < 2 || tokenRewards.RestartMissionTokens < 1 {
		t.Fatalf("rotated token rewards missing: undo=%d restart=%d skip=%d swap=%d",
			tokenRewards.UndoMoveTokens, tokenRewards.RestartMissionTokens,
			tokenRewards.CampaignSkipTokens, tokenRewards.ReplaceCardTokens)
	}
}

func TestHeroVarietyAndBranchMasterTrackActualProgress(t *testing.T) {
	p := newAchievementProfile(t)
	p.TotalWins = 10
	p.HeroWinCounts = map[string]int{"recruit": 4, "warden": 1, "hunter": 1, "anchor": 1}
	events := EvaluateAll(Context{Profile: p, Event: EventProfileLoad})
	hero, ok := unlockFor(events, "hero_variety")
	if !ok || hero.Tier != 1 || hero.Marks != 400 {
		t.Fatalf("Hero Variety first tier = %+v; events %+v", hero, events)
	}

	p = newAchievementProfile(t)
	for _, node := range skills.AllNodes() {
		if node.Branch == "daily-wins" {
			p.SkillNodes[node.ID] = true
		}
	}
	events = EvaluateAll(Context{Profile: p, Event: EventSkillUnlocked})
	branch, ok := unlockFor(events, "branch_master")
	if !ok || branch.Tier != 1 || branch.Marks != 400 {
		t.Fatalf("Branch Master first tier = %+v; events %+v", branch, events)
	}
}

func TestGameModeNormalization(t *testing.T) {
	tests := map[string]string{
		"": "main", "POW": "main", "fog-of-war": "fog",
		"puzzle": "puzzler", "tugwar": "tug-war", "poker": "poker",
		"poker-duel": "poker", "abandon-duel": "abandon",
		"unknown": "",
	}
	for input, want := range tests {
		if got := NormalizeGameMode(input); got != want {
			t.Errorf("NormalizeGameMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func newAchievementProfile(t *testing.T) *profile.Profile {
	t.Helper()
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func hasUnlock(events []UnlockEvent, id string) bool {
	_, ok := unlockFor(events, id)
	return ok
}

func unlockFor(events []UnlockEvent, id string) (UnlockEvent, bool) {
	for _, event := range events {
		if event.AchievementID == id {
			return event, true
		}
	}
	return UnlockEvent{}, false
}
