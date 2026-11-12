package app

import (
	"testing"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

func TestApplyUnlockEventAppliesRewardsAndDeduplicatesBadges(t *testing.T) {
	p := &profile.Profile{}
	event := achievements.UnlockEvent{
		Marks: 100, Crowns: 5, SkillPoints: 1, ChromaPick: 1, HeroBoxes: 2, Badge: "badge_test",
		WoodenBoxes: 1, SilverBoxes: 2, GoldBoxes: 3,
		UndoMoveTokens: 1, RestartMissionTokens: 1, CampaignSkipTokens: 1, ReplaceCardTokens: 1,
	}
	applyUnlockEvent(p, event)
	applyUnlockEvent(p, event)
	if p.Marks != 200 || p.Crowns != 10 || p.SkillPoints != 2 ||
		p.ChromaPickTokens != 2 || p.HeroBoxesAvailable != 4 ||
		p.PendingBoxes["wooden"] != 2 || p.PendingBoxes["silver"] != 4 || p.PendingBoxes["gold"] != 6 ||
		p.UndoMoveTokens != 2 || p.RestartMissionTokens != 2 ||
		p.CampaignSkipTokens != 2 || p.ReplaceCardTokens != 2 {
		t.Fatalf("rewards were not applied per event: %+v", p)
	}
	if len(p.Badges) != 1 || p.Badges[0] != "badge_test" {
		t.Fatalf("badge duplication: %v", p.Badges)
	}
}

func TestCampaignCompletionAchievementGrantsRewardsOnce(t *testing.T) {
	p := &profile.Profile{
		Marks: 20, Crowns: 4, UndoMoveTokens: 2, RestartMissionTokens: 3,
		CampaignSkipTokens: 4, ReplaceCardTokens: 5,
		Campaigns: map[string]*profile.CampaignProgress{
			"poker": {CampaignID: "poker", Completed: true},
		},
	}
	events := evaluateCampaignCompletionAchievement(p, "poker")
	if len(events) != 1 || events[0].AchievementID != "camp_poker" ||
		events[0].Marks != 3000 || events[0].Crowns != 10 {
		t.Fatalf("Poker completion achievement = %+v", events)
	}
	if p.Marks != 3020 || p.Crowns != 14 || p.UndoMoveTokens != 2 ||
		p.RestartMissionTokens != 3 || p.CampaignSkipTokens != 4 || p.ReplaceCardTokens != 5 {
		t.Fatalf("completion rewards were not applied: %+v", p)
	}
	if events = evaluateCampaignCompletionAchievement(p, "poker"); len(events) != 0 {
		t.Fatalf("replayed completion re-awarded achievement: %+v", events)
	}
	p.Campaigns["rummy"] = &profile.CampaignProgress{CampaignID: "rummy", Completed: true}
	if len(evaluateCampaignCompletionAchievement(p, "rummy")) != 1 {
		t.Fatal("completion for one variant incorrectly blocked another variant achievement")
	}
	legacyProfile := &profile.Profile{
		Campaigns: map[string]*profile.CampaignProgress{
			"abandon": {CampaignID: "abandon", Completed: true},
		},
	}
	legacyEvents := evaluateAchievementEvent(legacyProfile, achievements.EventProfileLoad, "load")
	if !legacyProfile.Achievements["camp_abandon"] ||
		len(legacyEvents) != 1 || legacyEvents[0].AchievementID != "camp_abandon" ||
		legacyProfile.Marks != 3000 || legacyProfile.Crowns != 10 {
		t.Fatalf("previously completed campaign did not unlock its achievement: %+v", legacyEvents)
	}
}

func TestBuildMatchRecordAndProgressStats(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result := game.MatchResult{
		Won: true, Tied: true, PlayerSeat: game.SeatX, PlayerScore: 15, OpponentScore: 14,
		Games: []game.MatchGame{{
			Won: true, PlayerSeat: game.SeatX,
			PlayerJail:   []game.Card{{Suit: game.Fang, Face: 6}},
			OpponentJail: []game.Card{{Suit: game.Shield, Face: 4}},
			BountySuit:   game.Fang, Clashes: 2, ScoreAfterR2: 5, OpponentAfterR2: 7,
		}},
	}
	updateAchievementStats(p, result, economy.ModeCPUHard)
	ctx := achievements.NewMatchContext(p, result, economy.ModeIDCPUHard, "X")
	record := buildMatchRecord(ctx, economy.ModeCPUHard, 120, time.Now())
	p.RecordMatch(record)
	if p.TotalGames != 1 || p.WinsAsSeatX != 1 || p.HardCPUWins != 1 ||
		p.ComebackWins != 1 || p.TotalClashes != 2 || p.MaxClashesInGame != 2 || p.TotalMatchTies != 1 {
		t.Fatalf("match stats were not updated: %+v", p)
	}
	updateAchievementStats(p, result, economy.ModeRanked)
	if p.HardCPUWins != 2 {
		t.Fatalf("Ranked win against Hard AI did not count toward The Hard Way: %d", p.HardCPUWins)
	}
	if len(p.MatchHistory) != 1 || p.MatchHistory[0].PlayerSeat != "X" ||
		p.MatchHistory[0].BountySuit != "F" || p.MatchHistory[0].PlayerPrisonerSuits["F"] != 1 ||
		p.MatchHistory[0].MarksEarned != 120 {
		t.Fatalf("match record = %+v", p.MatchHistory)
	}
}
