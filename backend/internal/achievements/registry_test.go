package achievements

import (
	"fmt"
	"testing"
	"time"

	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

func TestEveryAchievementHasEvaluatorAndHandlesEmptyContext(t *testing.T) {
	evaluators := EvalRegistry()
	definitions := All()
	if len(evaluators) != len(definitions) {
		t.Fatalf("registered %d evaluators for %d achievements", len(evaluators), len(definitions))
	}
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ctx := Context{Profile: p, Event: EventProfileLoad}
	for _, definition := range definitions {
		evaluator, ok := evaluators[definition.ID]
		if !ok {
			t.Errorf("achievement %q has no evaluator", definition.ID)
			continue
		}
		evaluator(ctx)
	}
	EvaluateAll(ctx)
}

func TestEveryAchievementEvaluatorHasPassingAndFailingContexts(t *testing.T) {
	failureProfile := newAchievementProfile(t)
	failureContext := Context{Profile: failureProfile, Event: EventProfileLoad}
	for _, definition := range All() {
		_, unlocked, tier := registry[definition.ID](failureContext)
		if unlocked || tier != 0 {
			t.Errorf("%s unexpectedly passed the empty profile context: unlocked=%t tier=%d",
				definition.ID, unlocked, tier)
		}
	}

	p := newAchievementProfile(t)
	wonGame := func(cards []game.Card) game.MatchGame {
		return game.MatchGame{Won: true, PlayerJail: cards, BountySuit: game.Blade}
	}
	oneSuit := []game.Card{{Suit: game.Blade, Face: 1}, {Suit: game.Blade, Face: 2}, {Suit: game.Blade, Face: 3}}
	pass := map[string]Context{
		"sparse": {
			Profile: p, Event: EventMatchEnd, MatchWon: true,
			Result: &game.MatchResult{Games: []game.MatchGame{wonGame([]game.Card{
				{Suit: game.Blade, Face: 2}, {Suit: game.Fang, Face: 4},
			})}},
		},
		"efficient": {
			Profile: p, Event: EventMatchEnd,
			Result: &game.MatchResult{Games: []game.MatchGame{wonGame([]game.Card{
				{Suit: game.Blade, Face: 1}, {Suit: game.Blade, Face: 2}, {Suit: game.Blade, Face: 3},
				{Suit: game.Fang, Face: 1}, {Suit: game.Fang, Face: 2}, {Suit: game.Fang, Face: 3},
				{Suit: game.Shield, Face: 1},
			})}},
		},
		"bounty_hunter": {
			Profile: p, Event: EventMatchEnd,
			Result: &game.MatchResult{Games: []game.MatchGame{wonGame(oneSuit)}},
		},
		"bounty_denied": {
			Profile: p, Event: EventMatchEnd,
			Result: &game.MatchResult{Games: []game.MatchGame{{Won: true, BountySuit: game.Blade}}},
		},
		"bounty_decided": {
			Profile: p, Event: EventMatchEnd,
			Result: &game.MatchResult{Games: []game.MatchGame{{
				Won: true, PlayerJail: []game.Card{{Suit: game.Blade, Face: 1}},
				OpponentJail: []game.Card{{Suit: game.Shield, Face: 1}}, BountySuit: game.Blade,
				Modifiers: game.GameModifiers{Bounty: 1, Reckoning: [3]int{1, 1, 1}},
			}}},
		},
		"aggressor": {Profile: &profile.Profile{WinsAsSeatX: 1}},
		"defender":  {Profile: &profile.Profile{WinsAsSeatY: 1}},
		"streak_easy": {Profile: &profile.Profile{
			WinStreakByMode: map[int]int{economy.ModeIDCPUEasy: 3},
		}},
		"streak_medium": {Profile: &profile.Profile{
			WinStreakByMode: map[int]int{economy.ModeIDCPUNormal: 3},
		}},
		"streak_hard": {Profile: &profile.Profile{
			WinStreakByMode: map[int]int{economy.ModeIDCPUHard: 3},
		}},
		"streak_ranked": {Profile: &profile.Profile{
			WinStreakByMode: map[int]int{economy.ModeIDRanked: 3},
		}},
		"comeback":    {Profile: &profile.Profile{ComebackWins: 1}},
		"first_clash": {Profile: &profile.Profile{TotalClashes: 1}},
		"clash_master": {Profile: &profile.Profile{
			MaxClashesInGame: 3,
		}},
		"dead_heat":  {Profile: &profile.Profile{TotalMatchTies: 1}},
		"collector":  {Profile: &profile.Profile{TotalChromasPurchased: 3}},
		"long_march": {Profile: &profile.Profile{TotalGames: 100}},
		"hard_way":   {Profile: &profile.Profile{HardCPUWins: 10}},
		"login_streak": {Profile: &profile.Profile{
			LoginStreakBest: 1,
		}},
		"first_blood": {
			Profile: &profile.Profile{TotalWins: 1}, Event: EventMatchEnd, MatchWon: true,
		},
		"both_sides": {
			Profile: &profile.Profile{
				MatchHistory: []profile.MatchRecord{{Won: true, PlayerSeat: "X"}},
			},
			Event: EventMatchEnd, MatchWon: true, PlayerSeat: "Y",
		},
		"the_cycle": {
			Profile: p, Event: EventMatchEnd,
			Result: &game.MatchResult{Games: []game.MatchGame{{Won: true, SuitDoublingWin: true}}},
		},
		"master_both": {
			Profile: &profile.Profile{AchievementCounters: map[string]int{
				"master_both_X_streak": 3, "master_both_Y_streak": 3,
			}},
			Event: EventMatchEnd,
		},
		"photo_finish": {
			Profile: p, Event: EventMatchEnd, MatchWon: true,
			Result: &game.MatchResult{PlayerScore: 2, OpponentScore: 1},
		},
		"no_not_like_that": {
			Profile: p, Event: EventMatchEnd,
			Result: &game.MatchResult{Games: []game.MatchGame{{
				Won: false, PlayerJail: []game.Card{
					{Suit: game.Blade, Face: 3}, {Suit: game.Blade, Face: 3}, {Suit: game.Blade, Face: 3},
				}, BountySuit: game.Fang,
			}}},
		},
		"bounty_connoisseur": {
			Profile: p, Event: EventMatchEnd, MatchWon: true,
			Result: &game.MatchResult{Won: true, Games: []game.MatchGame{{
				Won: true, Modifiers: game.GameModifiers{Bounty: -3, Reckoning: [3]int{1, 1, 1}},
			}}},
		},
		"favored_aficionado": {
			Profile: p, Event: EventMatchEnd, MatchWon: true,
			Result: &game.MatchResult{Won: true, Games: []game.MatchGame{{
				Won: true, Modifiers: game.GameModifiers{Favored: 2, Reckoning: [3]int{1, 1, 1}},
			}}},
		},
		"shift_whisperer": {
			Profile: p, Event: EventMatchEnd, MatchWon: true,
			Result: &game.MatchResult{Won: true, Games: []game.MatchGame{{
				Won: true, Modifiers: game.GameModifiers{Shift: 3, Reckoning: [3]int{1, 1, 1}},
			}}},
		},
		"hero_variety": {
			Profile: &profile.Profile{UnlockedHeroes: []string{"recruit", "warden", "hunter", "anchor"}},
			Event:   EventHeroAcquired, AcquisitionSource: "campaign",
		},
	}
	for _, definition := range All() {
		testContext, ok := pass[definition.ID]
		if !ok {
			t.Errorf("missing passing context for %s", definition.ID)
			continue
		}
		_, unlocked, tier := registry[definition.ID](testContext)
		if !unlocked && tier == 0 {
			t.Errorf("%s failed its passing context", definition.ID)
		}
	}
}

func TestModifierAchievementsUnlockOnlyTheMatchingIndependentTier(t *testing.T) {
	tests := []struct {
		id        string
		modifiers game.GameModifiers
		tier      int
	}{
		{"bounty_connoisseur", game.GameModifiers{Bounty: -3, Reckoning: [3]int{1, 1, 1}}, 1},
		{"bounty_connoisseur", game.GameModifiers{Bounty: -2, Reckoning: [3]int{1, 1, 1}}, 2},
		{"bounty_connoisseur", game.GameModifiers{Bounty: -1, Reckoning: [3]int{1, 1, 1}}, 3},
		{"bounty_connoisseur", game.GameModifiers{Bounty: 0, Reckoning: [3]int{1, 1, 1}}, 4},
		{"bounty_connoisseur", game.GameModifiers{Bounty: 1, Reckoning: [3]int{1, 1, 1}}, 5},
		{"bounty_connoisseur", game.GameModifiers{Bounty: 2, Reckoning: [3]int{1, 1, 1}}, 6},
		{"bounty_connoisseur", game.GameModifiers{Bounty: 3, Reckoning: [3]int{1, 1, 1}}, 7},
		{"favored_aficionado", game.GameModifiers{Favored: -1, Reckoning: [3]int{1, 1, 1}}, 1},
		{"favored_aficionado", game.GameModifiers{Favored: 0, Reckoning: [3]int{1, 1, 1}}, 2},
		{"favored_aficionado", game.GameModifiers{Favored: 1, Reckoning: [3]int{1, 1, 1}}, 3},
		{"favored_aficionado", game.GameModifiers{Favored: 2, Reckoning: [3]int{1, 1, 1}}, 4},
		{"shift_whisperer", game.GameModifiers{Shift: 0, Reckoning: [3]int{1, 1, 1}}, 1},
		{"shift_whisperer", game.GameModifiers{Shift: 1, Reckoning: [3]int{1, 1, 1}}, 2},
		{"shift_whisperer", game.GameModifiers{Shift: 2, Reckoning: [3]int{1, 1, 1}}, 3},
		{"shift_whisperer", game.GameModifiers{Shift: 3, Reckoning: [3]int{1, 1, 1}}, 4},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s/tier-%d", test.id, test.tier), func(t *testing.T) {
			p := newAchievementProfile(t)
			result := game.MatchResult{Won: true, Games: []game.MatchGame{{
				Won: true, Modifiers: test.modifiers,
			}}}
			ctx := NewMatchContext(p, result, economy.ModeIDRanked, "X")
			events := EvaluateAll(ctx)
			var match *UnlockEvent
			for index := range events {
				if events[index].AchievementID == test.id {
					match = &events[index]
					break
				}
			}
			if match == nil || match.Tier != test.tier {
				t.Fatalf("matching tier event = %+v; events %v", match, events)
			}
			if got := p.AchievementTiers[test.id]; got != 1<<(test.tier-1) {
				t.Fatalf("tier mask = %b, want only tier %d", got, test.tier)
			}
			if repeated := EvaluateAll(ctx); hasUnlockFor(repeated, test.id) {
				t.Fatalf("repeated match awarded tier %d again: %+v", test.tier, repeated)
			}
		})
	}
}

func TestHeroVarietyExcludesRecruitAndRewardsAtThreeSixAndTen(t *testing.T) {
	p := newAchievementProfile(t)
	p.UnlockedHeroes = []string{"recruit", "warden", "hunter", "anchor"}
	events := EvaluateAll(Context{Profile: p, Event: EventHeroAcquired, AcquisitionSource: "campaign"})
	if !hasTierUnlock(events, "hero_variety", 1) || p.AchievementTiers["hero_variety"] != 1 {
		t.Fatalf("three non-Recruit heroes did not unlock Novice: %+v", events)
	}
	for _, heroID := range []string{"treasurer", "ascendant", "gambler"} {
		p.UnlockedHeroes = append(p.UnlockedHeroes, heroID)
	}
	events = EvaluateAll(Context{Profile: p, Event: EventHeroAcquired, AcquisitionSource: "box"})
	if !hasTierUnlock(events, "hero_variety", 2) || p.AchievementTiers["hero_variety"] != 2 {
		t.Fatalf("six non-Recruit heroes did not unlock Collector: %+v", events)
	}
	for _, heroID := range []string{"diplomat", "ascetic", "herald", "hater"} {
		p.UnlockedHeroes = append(p.UnlockedHeroes, heroID)
	}
	events = EvaluateAll(Context{Profile: p, Event: EventHeroAcquired, AcquisitionSource: "purchase"})
	var completion *UnlockEvent
	for index := range events {
		if events[index].AchievementID == "hero_variety" && events[index].Tier == 3 {
			completion = &events[index]
		}
	}
	if completion == nil || completion.Crowns != 500 || completion.Badge != "badge_completionist" ||
		completion.HeroBoxes != 1 || p.AchievementTiers["hero_variety"] != 3 {
		t.Fatalf("ten non-Recruit heroes rewards = %+v; events %v", completion, events)
	}
	if !IsUnlocked(p, Achievement{ID: "hero_variety", Tiers: []Tier{{Level: 1}, {Level: 2}, {Level: 3}}}) {
		t.Fatal("Hero Variety should be complete after all three tiers")
	}
}

func TestSparseTierMatchingAndMatchHistoryTierIV(t *testing.T) {
	p := newAchievementProfile(t)
	games := []game.MatchGame{
		{Won: true, PlayerJail: []game.Card{{Suit: game.Blade, Face: 2}, {Suit: game.Fang, Face: 4}}},
		{Won: true, PlayerJail: []game.Card{{Suit: game.Shield, Face: 3}}, OpponentJail: []game.Card{{Suit: game.Blade, Face: 5}}},
	}
	result := game.MatchResult{Won: true, PlayerScore: 8, OpponentScore: 4, Games: games}
	ctx := NewMatchContext(p, result, economy.ModeIDRanked, "X")
	_, unlocked, tier := registry["sparse"](ctx)
	if !unlocked || tier != 4 {
		t.Fatalf("best-of-three sparse result = unlocked %t, tier %d", unlocked, tier)
	}
	events := EvaluateAll(ctx)
	found := false
	for _, event := range events {
		if event.AchievementID == "sparse" && event.Tier == 4 {
			found = event.Marks == 5500 && event.Crowns == 50
		}
	}
	if !found {
		t.Fatalf("tier IV event with crossed rewards not found: %+v", events)
	}
	if p.AchievementTiers["sparse"] != 4 {
		t.Fatalf("stored sparse tier = %d", p.AchievementTiers["sparse"])
	}
}

func TestEfficientBountyComebackAndMiscEvaluators(t *testing.T) {
	p := newAchievementProfile(t)
	playerJail := []game.Card{
		{Suit: game.Blade, Face: 1}, {Suit: game.Blade, Face: 2}, {Suit: game.Blade, Face: 3},
		{Suit: game.Fang, Face: 1}, {Suit: game.Fang, Face: 2}, {Suit: game.Fang, Face: 3},
		{Suit: game.Shield, Face: 1}, {Suit: game.Shield, Face: 2}, {Suit: game.Shield, Face: 3},
	}
	winningGame := game.MatchGame{
		Won: true, PlayerSeat: game.SeatX, PlayerJail: playerJail,
		OpponentJail: []game.Card{{Suit: game.Blade, Face: 1}},
		BountySuit:   game.Blade, Clashes: 0, ScoreAfterR2: 3, OpponentAfterR2: 8,
		SuitDoublingWin: true,
	}
	result := game.MatchResult{Won: true, PlayerScore: 20, OpponentScore: 8, Games: []game.MatchGame{winningGame}}
	ctx := NewMatchContext(p, result, economy.ModeID(economy.ModeCPUHard), "X")
	for id, wantTier := range map[string]int{"efficient": 3} {
		_, unlocked, tier := registry[id](ctx)
		if !unlocked || tier != wantTier {
			t.Errorf("%s result = unlocked %t tier %d, want %d", id, unlocked, tier, wantTier)
		}
	}
	p.ComebackWins = 1
	_, unlocked, tier := registry["comeback"](ctx)
	if !unlocked || tier != 1 {
		t.Fatalf("comeback result = %t, tier %d", unlocked, tier)
	}
	_, unlocked, _ = registry["the_cycle"](ctx)
	if !unlocked {
		t.Fatal("doubling win did not unlock The Cycle")
	}

	bountyWon := game.MatchGame{
		Won: true, PlayerJail: []game.Card{
			{Suit: game.Blade, Face: 1}, {Suit: game.Blade, Face: 2}, {Suit: game.Blade, Face: 3},
		}, BountySuit: game.Blade,
	}
	ctx.Result.Games = []game.MatchGame{bountyWon}
	_, unlocked, _ = registry["bounty_hunter"](ctx)
	if !unlocked {
		t.Fatal("three bounty-suit prisoners did not unlock Bounty Hunter")
	}
	_, unlocked, _ = registry["bounty_denied"](ctx)
	if unlocked {
		t.Fatal("Bounty Denied unlocked despite bounty-suit prisoners")
	}
	ctx.Result.Games = []game.MatchGame{{
		Won: true, PlayerJail: []game.Card{{Suit: game.Blade, Face: 1}},
		OpponentJail: []game.Card{{Suit: game.Shield, Face: 1}},
		BountySuit:   game.Blade,
		Modifiers:    game.GameModifiers{Bounty: 1, Reckoning: [3]int{1, 1, 1}},
	}}
	ctx.Result.PlayerScore = 1
	ctx.Result.OpponentScore = 0
	_, unlocked, _ = registry["bounty_decided"](ctx)
	if !unlocked {
		t.Fatal("bounty bonus alone should decide this game's score")
	}

	losingGame := game.MatchGame{
		Won: false, PlayerJail: []game.Card{{Suit: game.Shield, Face: 9}},
		BountySuit: game.Blade,
	}
	ctx.Result.Games = []game.MatchGame{losingGame}
	ctx.MatchWon = false
	_, unlocked, _ = registry["no_not_like_that"](ctx)
	if !unlocked {
		t.Fatal("lost game with nine points in one suit did not qualify")
	}
}

func TestCountersTierCrossingAndSetCompletionAreIdempotent(t *testing.T) {
	p := newAchievementProfile(t)
	p.WinsAsSeatX = 100
	p.TotalClashes = 5
	p.MaxClashesInGame = 3
	p.WinStreakByMode[economy.ModeIDCPUEasy] = 2
	ctx := Context{
		Profile: p, Event: EventMatchEnd, MatchWon: true, PlayerSeat: "X",
		Result: &game.MatchResult{Won: true, Games: []game.MatchGame{{Won: true, PlayerSeat: game.SeatX}}},
	}
	events := EvaluateAll(ctx)
	var aggressor *UnlockEvent
	var setCompleted bool
	for i := range events {
		if events[i].AchievementID == "aggressor" {
			aggressor = &events[i]
		}
		if events[i].SetID == "aggressor" {
			setCompleted = events[i].SkillPoints == 1
		}
	}
	if aggressor == nil || aggressor.Tier != 3 || aggressor.Marks != 6200 || aggressor.Crowns != 100 ||
		aggressor.Badge != "badge_aggressor" || aggressor.HeroBoxes != 1 || !setCompleted {
		t.Fatalf("full tier crossing/set reward missing: %+v", events)
	}
	if p.AchievementCounters["streak_easy"] != 2 || p.AchievementCounters["total_clashes"] != 5 ||
		p.AchievementCounters["max_clashes_in_game"] != 3 {
		t.Fatalf("counter values not recorded: %+v", p.AchievementCounters)
	}
	events = EvaluateAll(ctx)
	for _, event := range events {
		if event.AchievementID == "aggressor" || event.SetID == "aggressor" {
			t.Fatalf("achievement/set reward repeated: %+v", event)
		}
	}
	p.WinStreakByMode[economy.ModeIDCPUEasy] = 0
	EvaluateAll(Context{Profile: p, Event: EventMatchEnd, Result: ctx.Result})
	if p.AchievementCounters["streak_easy"] != 0 {
		t.Fatalf("streak counter did not reset: %d", p.AchievementCounters["streak_easy"])
	}
	p.TotalClashes = 1
	p.MaxClashesInGame = 1
	EvaluateAll(Context{Profile: p, Event: EventProfileLoad})
	if p.AchievementCounters["total_clashes"] != 5 || p.AchievementCounters["max_clashes_in_game"] != 3 {
		t.Fatal("cumulative/best-record counters regressed")
	}
}

func TestMatchHistoryBackedAchievementsAndContextDerivation(t *testing.T) {
	p := newAchievementProfile(t)
	p.MatchHistory = []profile.MatchRecord{{Won: true, PlayerSeat: "X"}}
	result := game.MatchResult{
		Won: true, PlayerScore: 20, OpponentScore: 12,
		Games: []game.MatchGame{{
			Won: true, PlayerSeat: game.SeatX,
			PlayerJail:   []game.Card{{Suit: game.Fang, Face: 7}, {Suit: game.Blade, Face: 2}},
			OpponentJail: []game.Card{{Suit: game.Shield, Face: 3}},
			BountySuit:   game.Fang, Clashes: 1, ScoreAfterR2: 4, OpponentAfterR2: 8,
		}},
	}
	ctx := NewMatchContext(p, result, economy.ModeID(economy.ModeCPUEasy), "Y")
	if ctx.PlayerPrisoners != 2 || ctx.OppPrisoners != 1 || ctx.BountyCount != 1 ||
		ctx.MaxFace != 7 || ctx.Clashes != 1 || ctx.ScoreAfterR2 != 4 || ctx.OppScoreAfterR2 != 8 {
		t.Fatalf("derived context = %+v", ctx)
	}
	_, unlocked, _ := registry["both_sides"](ctx)
	if !unlocked {
		t.Fatal("current Seat Y win plus historical Seat X win should unlock Both Sides")
	}
}

func TestLoginRewardAndSetCompletesOnlyWhenAllMembersAreUnlocked(t *testing.T) {
	p := newAchievementProfile(t)
	p.LoginStreakBest = 30
	loginEvents := EvaluateAll(Context{Profile: p, Event: EventLogin})
	var loginTier *UnlockEvent
	var loginSet bool
	for i := range loginEvents {
		if loginEvents[i].AchievementID == "login_streak" {
			loginTier = &loginEvents[i]
		}
		if loginEvents[i].SetID == "login_streak" {
			loginSet = loginEvents[i].SkillPoints == 1
		}
	}
	if loginTier == nil || loginTier.Tier != 3 || loginTier.Marks != 3850 ||
		loginTier.Crowns != 50 || loginTier.Badge != "badge_devoted" || loginTier.HeroBoxes != 1 || !loginSet {
		t.Fatalf("login tier/set events = %+v", loginEvents)
	}

	p.Achievements["bounty_hunter"] = true
	p.Achievements["bounty_denied"] = true
	events := EvaluateAll(Context{Profile: p, Event: EventProfileLoad})
	for _, event := range events {
		if event.SetID == "bounty" {
			t.Fatal("incomplete Bounty set was marked complete")
		}
	}
	p.Achievements["bounty_decided"] = true
	events = EvaluateAll(Context{Profile: p, Event: EventProfileLoad})
	found := false
	for _, event := range events {
		if event.SetID == "bounty" && event.SkillPoints == 1 {
			found = true
		}
	}
	if !found || !p.CompletedSets["bounty"] {
		t.Fatal("complete Bounty set did not award one Skill Tree Point")
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

func hasTierUnlock(events []UnlockEvent, id string, tier int) bool {
	for _, event := range events {
		if event.AchievementID == id && event.Tier == tier {
			return true
		}
	}
	return false
}

func hasUnlockFor(events []UnlockEvent, id string) bool {
	for _, event := range events {
		if event.AchievementID == id {
			return true
		}
	}
	return false
}
