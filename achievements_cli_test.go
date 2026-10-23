package main

import (
	"testing"
	"time"

	"prisonersofwar/achievements"
	"prisonersofwar/economy"
	"prisonersofwar/game"
	"prisonersofwar/profile"
)

func TestApplyUnlockEventAppliesRewardsAndDeduplicatesBadges(t *testing.T) {
	p := &profile.Profile{}
	event := achievements.UnlockEvent{
		Marks: 100, Crowns: 5, SkillPoints: 1, ChromaPick: 1, HeroBoxes: 2, Badge: "badge_test",
	}
	applyUnlockEvent(p, event)
	applyUnlockEvent(p, event)
	if p.Marks != 200 || p.Crowns != 10 || p.SkillPoints != 2 ||
		p.ChromaPickTokens != 2 || p.HeroBoxesAvailable != 4 {
		t.Fatalf("rewards were not applied per event: %+v", p)
	}
	if len(p.Badges) != 1 || p.Badges[0] != "badge_test" {
		t.Fatalf("badge duplication: %v", p.Badges)
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
