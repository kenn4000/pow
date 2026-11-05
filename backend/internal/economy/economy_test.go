package economy

import (
	"testing"
	"time"

	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

func TestCalculatePayoutSkillsDailyCapAndCampDraw(t *testing.T) {
	now := time.Date(2025, time.May, 4, 10, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.SkillPoints = 2
	if err := skills.UnlockNodeWithSkillPoint(p, "quartermaster"); err != nil {
		t.Fatal(err)
	}
	if err := skills.UnlockNodeWithSkillPoint(p, "first-blood"); err != nil {
		t.Fatal(err)
	}
	p.WinsSinceCampDraw = lootbox.WinsPerCampDraw - 1
	first, err := CalculatePayout(MatchResult{WinnerSeat: 0, PlayerSeat: 0, PlayerScore: 20}, ModeCPUEasy, p, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if !first.PlayerWon || !first.FirstWinBonus || !first.CampDrawAwarded || first.TotalMarks != 225 {
		t.Fatalf("first payout = %+v", first)
	}
	if p.DrawsAvailable != 1 || p.WinsSinceCampDraw != 0 || p.TotalWins != 1 {
		t.Fatalf("updated profile = draws %d, win progress %d, wins %d",
			p.DrawsAvailable, p.WinsSinceCampDraw, p.TotalWins)
	}
	p.DailyWins = DailyScoreBonusWinCap
	capped, err := CalculatePayout(MatchResult{WinnerSeat: 0, PlayerSeat: 0, PlayerScore: 99}, ModeCPUEasy, p, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if !capped.PlayerWon || capped.ScoreBonusMarks != 0 || capped.TotalMarks != 110 {
		t.Fatalf("win payout after cap = %+v", capped)
	}
}

func TestCalculatePayoutModesAndDailyReset(t *testing.T) {
	now := time.Date(2025, time.June, 2, 10, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}

	p.SkillPoints = 4
	if err := skills.UnlockNodeWithSkillPoint(p, "quartermaster"); err != nil {
		t.Fatal(err)
	}
	if err := skills.UnlockNodeWithSkillPoint(p, "streak"); err != nil {
		t.Fatal(err)
	}
	if err := skills.UnlockNodeWithSkillPoint(p, "field-warden"); err != nil {
		t.Fatal(err)
	}
	p.DailyWins = 8
	p.DailyWinResetAt = now.Add(-time.Minute)
	p.CurrentStreak = 3
	result := MatchResult{WinnerSeat: 1, PlayerSeat: 1, PlayerScore: 10}
	payout, err := CalculatePayout(result, ModeRanked, p, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if !payout.PlayerWon || payout.ModeMultiplier != RankedMultiplier+0.15 || payout.TotalMarks != 214 ||
		p.DailyWins != 1 || p.CurrentStreak != 4 {
		t.Fatalf("ranked payout/profile = %+v; daily %d, streak %d", payout, p.DailyWins, p.CurrentStreak)
	}
}

func TestCalculatePayoutRejectsMismatchedSeatAndUnknownMode(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result := MatchResult{WinnerSeat: 0, PlayerSeat: 0}
	if _, err := CalculatePayout(result, ModeCPUEasy, p, 1, time.Now()); err == nil {
		t.Fatal("mismatched player seat should fail")
	}
	if _, err := CalculatePayout(result, Mode("custom"), p, 0, time.Now()); err == nil {
		t.Fatal("unknown mode should fail")
	}
}

func TestCampDrawThresholdFavorAndStoredDrawCap(t *testing.T) {
	now := time.Date(2026, time.March, 5, 10, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}

	p.SkillNodes["camp-lore"] = true
	p.WinsSinceCampDraw = lootbox.WinsPerCampDraw - 2
	p.DrawsAvailable = lootbox.MaxDrawsStored
	result, err := CalculatePayout(
		MatchResult{WinnerSeat: 0, PlayerSeat: 0},
		ModeCPUEasy, p, 0, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.CampDrawAwarded || p.DrawsAvailable != lootbox.MaxDrawsStored ||
		p.WinsSinceCampDraw != 0 || lootbox.RequiredWins(p) != 8 {
		t.Fatalf("capped Camp Draw payout=%+v draws=%d progress=%d threshold=%d",
			result, p.DrawsAvailable, p.WinsSinceCampDraw, lootbox.RequiredWins(p))
	}

	p.DrawsAvailable--
	p.WinsSinceCampDraw = lootbox.WinsPerCampDraw - 2
	result, err = CalculatePayout(
		MatchResult{WinnerSeat: 0, PlayerSeat: 0},
		ModeCPUEasy, p, 0, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.CampDrawAwarded || p.DrawsAvailable != lootbox.MaxDrawsStored {
		t.Fatalf("Camp Lore draw payout=%+v draws=%d", result, p.DrawsAvailable)
	}
}

func TestMultiplayerPayoutVariantsDailyBonusAndAIScaling(t *testing.T) {
	now := time.Date(2026, time.June, 8, 12, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.SkillNodes["daily-triumph"] = true
	first, err := CalculateMultiplayerPayout("main", 10, true, false, true, "easy", ModeCPUEasy, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.BaseMarks != 330 || first.TotalMarks != 165 ||
		first.MatchCrowns != 2 || first.DailyWinBonusCrowns != 20 ||
		first.TotalCrowns != 22 || !first.FirstWinOfDay || p.DailyWins != 1 {
		t.Fatalf("first Easy AI payout = %+v, daily wins %d", first, p.DailyWins)
	}
	loss, err := CalculateMultiplayerPayout("poker-duel", 8, false, false, false, "hard", ModeCPUHard, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if loss.BaseMarks != 52 || loss.TotalMarks != 46 || loss.TotalCrowns != 0 {
		t.Fatalf("Poker loss payout = %+v", loss)
	}
	tie, err := CalculateMultiplayerPayout("ladder", 0, false, true, false, "", ModeFriendCasual, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if tie.BaseMarks != 300 || tie.TotalMarks != 300 || tie.MatchCrowns != 3 {
		t.Fatalf("draw win-payout = %+v", tie)
	}
}

func TestMultiplayerPayoutClampsScoreAndRejectsUnknownVariant(t *testing.T) {
	now := time.Now()
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	payout, err := CalculateMultiplayerPayout("puzzle-battle", -10, false, false, false, "", ModeFriendCasual, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if payout.PlayerScore != 0 || payout.TotalMarks != 25 {
		t.Fatalf("clamped Puzzler payout = %+v", payout)
	}
	if _, err := CalculateMultiplayerPayout("unknown", 1, false, false, false, "", ModeFriendCasual, p, now); err == nil {
		t.Fatal("unknown multiplayer variant accepted")
	}
}
