package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProfileV1MigratesToCurrentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	legacy, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"player_id":      "legacy-player",
		"marks":          123,
		"energy":         4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != SchemaVersion || p.Marks != 123 || p.Energy != 4 {
		t.Fatalf("migrated profile = version %d, Marks %d, energy %d", p.SchemaVersion, p.Marks, p.Energy)
	}
	if p.Achievements == nil || p.CompletedSets == nil || p.WinStreakByMode == nil {
		t.Fatal("migration did not initialize achievement maps")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted["schema_version"] != float64(SchemaVersion) {
		t.Fatalf("persisted migration version = %v", persisted["schema_version"])
	}
}

func TestRecordMatchPrependsAndTruncates(t *testing.T) {
	p := &Profile{}
	for i := 0; i < MatchHistoryLimit+3; i++ {
		p.RecordMatch(MatchRecord{
			PlayerScore:         i,
			PlayerPrisonerSuits: map[string]int{"B": i},
		})
	}
	if len(p.MatchHistory) != MatchHistoryLimit || p.MatchHistory[0].PlayerScore != MatchHistoryLimit+2 {
		t.Fatalf("history length/newest score = %d/%d", len(p.MatchHistory), p.MatchHistory[0].PlayerScore)
	}
	p.MatchHistory[0].PlayerPrisonerSuits["B"] = -1
	if p.MatchHistory[1].PlayerPrisonerSuits["B"] == -1 {
		t.Fatal("record maps unexpectedly share storage")
	}
}

func TestApplyLoginStreakCases(t *testing.T) {
	first := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.Local)
	p := &Profile{}
	p.ApplyLogin(first)
	if p.LoginStreakCurrent != 1 || p.LoginStreakBest != 1 {
		t.Fatalf("first login streak = %d/%d", p.LoginStreakCurrent, p.LoginStreakBest)
	}
	p.ApplyLogin(first.Add(2 * time.Hour))
	if p.LoginStreakCurrent != 1 {
		t.Fatalf("same-day login changed streak to %d", p.LoginStreakCurrent)
	}
	p.ApplyLogin(first.AddDate(0, 0, 1))
	p.ApplyLogin(first.AddDate(0, 0, 2))
	if p.LoginStreakCurrent != 3 || p.LoginStreakBest != 3 {
		t.Fatalf("consecutive login streak = %d/%d", p.LoginStreakCurrent, p.LoginStreakBest)
	}
	p.ApplyLogin(first.AddDate(0, 0, 4))
	if p.LoginStreakCurrent != 1 || p.LoginStreakBest != 3 {
		t.Fatalf("skipped-day login streak = %d/%d", p.LoginStreakCurrent, p.LoginStreakBest)
	}
}
