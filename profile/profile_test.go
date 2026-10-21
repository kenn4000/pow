package profile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSaveProfileRoundTripAndSchemaValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "profile.json")
	p, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.PlayerID == "" || p.Energy != DefaultMaxEnergy {
		t.Fatalf("new profile = ID %q, energy %d", p.PlayerID, p.Energy)
	}
	p.Marks = 1234
	if err := p.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	p.Marks = 0
	if err := p.SaveTo(path); err != nil {
		t.Fatalf("replace existing profile: %v", err)
	}
	loaded, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PlayerID != p.PlayerID || loaded.Marks != 0 {
		t.Fatalf("round-trip values = ID %q, Marks %d", loaded.PlayerID, loaded.Marks)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":99,"player_id":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProfileFrom(path); err == nil {
		t.Fatal("expected unsupported schema version to fail")
	}
}

func TestApplyEnergyTickUsesWholeRechargeIntervals(t *testing.T) {
	now := time.Date(2025, time.March, 3, 12, 0, 0, 0, time.UTC)
	p, err := New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.Energy = 5
	p.EnergyUpdatedAt = now
	p.ApplyEnergyTick(now.Add(2*DefaultEnergyRecharge + DefaultEnergyRecharge/2))
	if p.Energy != 7 {
		t.Fatalf("energy = %d, want 7", p.Energy)
	}
	if got := p.EnergyUpdatedAt; !got.Equal(now.Add(2 * DefaultEnergyRecharge)) {
		t.Fatalf("energy update timestamp = %s", got)
	}
	p.ApplyEnergyTick(now.Add(10 * DefaultEnergyRecharge))
	if p.Energy != p.MaxEnergy || !p.EnergyUpdatedAt.Equal(now.Add(10*DefaultEnergyRecharge)) {
		t.Fatalf("full-energy state = %d at %s", p.Energy, p.EnergyUpdatedAt)
	}
}

func TestSchemaV1MigrationInitializesAchievementFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	legacy := `{"schema_version":1,"player_id":"legacy-player","marks":321,"total_games":7}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != SchemaVersion || p.Marks != 321 || p.TotalGames != 7 {
		t.Fatalf("migrated profile = schema %d, marks %d, games %d", p.SchemaVersion, p.Marks, p.TotalGames)
	}
	if p.MatchHistory == nil || p.Achievements == nil || p.AchievementTiers == nil ||
		p.AchievementCounters == nil || p.CompletedSets == nil || p.WinStreakByMode == nil ||
		p.BestWinStreakByMode == nil {
		t.Fatal("migration did not initialize achievement maps and history")
	}
	if !p.ColorOn || !p.ConfirmQuit || p.SeenAchievementIDs == nil {
		t.Fatal("v1 migration did not initialize landing-page settings")
	}
	reloaded, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.SchemaVersion != SchemaVersion || reloaded.PlayerID != "legacy-player" {
		t.Fatalf("persisted migrated profile = schema %d, player %q", reloaded.SchemaVersion, reloaded.PlayerID)
	}
}

func TestSchemaV2MigrationDefaultsSettingsAndPersistsToggles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	legacy := `{"schema_version":2,"player_id":"v2-player","marks":42}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != SchemaVersion || !p.ColorOn || !p.ConfirmQuit || p.SeenAchievementIDs == nil {
		t.Fatalf("v2 migration settings = schema %d, color %t, confirm %t, seen nil %t",
			p.SchemaVersion, p.ColorOn, p.ConfirmQuit, p.SeenAchievementIDs == nil)
	}
	p.ColorOn = false
	p.ConfirmQuit = false
	p.SeenAchievementIDs = []string{"first_blood"}
	if err := p.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ColorOn || reloaded.ConfirmQuit || len(reloaded.SeenAchievementIDs) != 1 ||
		reloaded.SeenAchievementIDs[0] != "first_blood" {
		t.Fatalf("settings did not persist: color %t, confirm %t, seen %v",
			reloaded.ColorOn, reloaded.ConfirmQuit, reloaded.SeenAchievementIDs)
	}
}

func TestSchemaV3MigrationInitializesPendingBoxes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	legacy := `{"schema_version":3,"player_id":"v3-player","camp_draws_available":2,"camp_draw_wins":6}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != SchemaVersion || p.PendingBoxes == nil ||
		p.DrawsAvailable != 2 || p.WinsSinceCampDraw != 6 {
		t.Fatalf("v3 migration = schema %d, pending nil %t, draws %d, wins %d",
			p.SchemaVersion, p.PendingBoxes == nil, p.DrawsAvailable, p.WinsSinceCampDraw)
	}
}

func TestGrantBoxAndTotalPendingBoxes(t *testing.T) {
	p := &Profile{}
	if err := p.GrantBox("silver", 2); err != nil {
		t.Fatal(err)
	}
	if err := p.GrantBox("diamond", 1); err != nil {
		t.Fatal(err)
	}
	if p.TotalPendingBoxes() != 3 {
		t.Fatalf("pending box total = %d, want 3", p.TotalPendingBoxes())
	}
	if err := p.GrantBox("bronze", 1); err == nil {
		t.Fatal("unknown box type should fail")
	}
	if err := p.GrantBox("wooden", -1); err == nil {
		t.Fatal("negative grant should fail")
	}
}

func TestRecordMatchPrependsTruncatesAndCopiesSuitCounts(t *testing.T) {
	p := &Profile{}
	suitCounts := map[string]int{"B": 2}
	for i := 0; i < MatchHistoryLimit+1; i++ {
		p.RecordMatch(MatchRecord{PlayerScore: i, PlayerPrisonerSuits: suitCounts})
	}
	suitCounts["B"] = 99
	if len(p.MatchHistory) != MatchHistoryLimit || p.MatchHistory[0].PlayerScore != MatchHistoryLimit ||
		p.MatchHistory[MatchHistoryLimit-1].PlayerScore != 1 {
		t.Fatalf("history bounds/order incorrect: len=%d first=%d last=%d",
			len(p.MatchHistory), p.MatchHistory[0].PlayerScore, p.MatchHistory[MatchHistoryLimit-1].PlayerScore)
	}
	if p.MatchHistory[0].PlayerPrisonerSuits["B"] != 2 {
		t.Fatal("record retained a caller-owned suit-count map")
	}
}

func TestApplyLoginFirstSameDayConsecutiveAndSkippedDates(t *testing.T) {
	p := &Profile{}
	first := time.Date(2025, time.June, 1, 12, 0, 0, 0, time.UTC)
	p.ApplyLogin(first)
	if p.LoginStreakCurrent != 1 || p.LoginStreakBest != 1 {
		t.Fatalf("first login streak = %d/%d", p.LoginStreakCurrent, p.LoginStreakBest)
	}
	p.ApplyLogin(first.Add(time.Hour))
	if p.LoginStreakCurrent != 1 || p.LoginStreakBest != 1 {
		t.Fatalf("same-day login changed streak = %d/%d", p.LoginStreakCurrent, p.LoginStreakBest)
	}
	p.ApplyLogin(first.AddDate(0, 0, 1))
	if p.LoginStreakCurrent != 2 || p.LoginStreakBest != 2 {
		t.Fatalf("consecutive login streak = %d/%d", p.LoginStreakCurrent, p.LoginStreakBest)
	}
	p.ApplyLogin(first.AddDate(0, 0, 3))
	if p.LoginStreakCurrent != 1 || p.LoginStreakBest != 2 {
		t.Fatalf("skipped-day login streak = %d/%d", p.LoginStreakCurrent, p.LoginStreakBest)
	}
}
