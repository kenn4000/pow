package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/profile"
)

func TestLandingPageRendersNewAndProgressedPlayerStates(t *testing.T) {
	now := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.Energy = 0
	p.EnergyUpdatedAt = now
	p.AdsResetAt = now.Add(24 * time.Hour)
	p.DrawsAvailable = 0
	p.WinsSinceCampDraw = 0
	p.SkillPoints = 0
	var output bytes.Buffer
	if err := renderLandingPage(&output, p, now); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, expected := range []string{
		"PRISONERS OF WAR", "Featured chroma: none today", "[1] PLAY",
		"[3] Shop", "[4] Skill Tree", "[5] Achievements",
		"Login Streak: Day 0 of 30", "Camp Draw: 0 / 10 wins",
		"Energy: 0/10 (next in 10:00)", "[W]atch Ad (+3, 5 left today)",
		"[6] Profile", "[7] Settings", "[8] Heroes", "[9] Campaigns", "[Q] Quit",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("landing page missing %q:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "[2] Camp Draw") || strings.Contains(rendered, "(0 new)") {
		t.Fatalf("landing page showed unavailable/empty actions:\n%s", rendered)
	}

	p.Energy = 5
	p.DrawsAvailable = 2
	if err := p.GrantBox("gold", 1); err != nil {
		t.Fatal(err)
	}
	p.WinsSinceCampDraw = 4
	p.SkillPoints = 3
	p.Achievements["first_blood"] = true
	p.AchievementTiers["mode_main"] = 1
	p.SeenAchievementIDs = []string{"first_blood"}
	output.Reset()
	if err := renderLandingPage(&output, p, now); err != nil {
		t.Fatal(err)
	}
	rendered = output.String()
	for _, expected := range []string{
		"[2] Camp Draw (2)", "[4] Skill Tree (3 STP)",
		"[5] Achievements (1 new)", "Camp Draw: 4 / 10 wins",
		"Energy: 5/10 (next in 10:00)", "Pending Boxes: 1",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("progressed landing page missing %q:\n%s", expected, rendered)
		}
	}
	if got := unseenAchievementIDs(p); len(got) != 1 || got[0] != "mode_main" {
		t.Fatalf("unseen achievements = %v, want [mode_main]", got)
	}
	markAchievementsSeen(p)
	if got := unseenAchievementIDs(p); len(got) != 0 {
		t.Fatalf("marking gallery viewed left unseen achievements: %v", got)
	}
}

func TestLandingPageHidesWatchAdWhenUnavailable(t *testing.T) {
	now := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.Energy = p.MaxEnergy
	p.AdsResetAt = now.Add(24 * time.Hour)
	var output bytes.Buffer
	if err := renderLandingPage(&output, p, now); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "[W]atch Ad") {
		t.Fatal("Watch Ad should be hidden at full energy")
	}

	p.Energy = p.MaxEnergy - 1
	p.AdsWatchedToday = energy.DailyAdLimit
	output.Reset()
	if err := renderLandingPage(&output, p, now); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "[W]atch Ad") {
		t.Fatal("Watch Ad should be hidden when the daily ad limit is reached")
	}
}
