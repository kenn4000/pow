package app

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func TestAssignInitialSeatsPreservesCardAccounting(t *testing.T) {
	for _, seat := range []game.Seat{game.SeatX, game.SeatY} {
		state, err := game.NewGame(rand.New(rand.NewSource(int64(seat + 80))))
		if err != nil {
			t.Fatal(err)
		}
		if err := assignInitialSeats(&state, seat); err != nil {
			t.Fatal(err)
		}
		wantP1 := 5
		if seat == game.SeatX {
			wantP1 = 6
		}
		if len(state.P1.Hand) != wantP1 || len(state.P2.Hand) != 11-wantP1 {
			t.Fatalf("seat %s hand sizes = %d/%d", seat, len(state.P1.Hand), len(state.P2.Hand))
		}
		if err := state.Validate(); err != nil {
			t.Fatalf("state after assigning seat %s: %v", seat, err)
		}
	}
}

func TestCampaignEnergyCostSkillOnlyCoversFirstMission(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.SkillNodes["campaign_free"] = true
	if got := campaignMissionEnergyCost(p, campaigns.Mission{Index: 1}); got != 0 {
		t.Fatalf("first mission energy cost = %d, want 0", got)
	}
	if got := campaignMissionEnergyCost(p, campaigns.Mission{Index: 2}); got != 1 {
		t.Fatalf("later mission energy cost = %d, want 1", got)
	}
	p.SkillNodes["campaign_free"] = false
	if got := campaignMissionEnergyCost(p, campaigns.Mission{Index: 1}); got != 1 {
		t.Fatalf("first mission without skill costs %d, want 1", got)
	}
}

func TestHeroAcquisitionEventAwardsVarietyTier(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.UnlockedHeroes = append(p.UnlockedHeroes, "warden", "hunter", "anchor")
	events := evaluateAchievementEvent(p, achievements.EventHeroAcquired, "campaign")
	tierFound := false
	for _, event := range events {
		tierFound = tierFound || (event.AchievementID == "hero_variety" && event.Tier == 1)
	}
	if !tierFound || p.Marks != 500 {
		t.Fatalf("campaign acquisition events = %+v; marks=%d", events, p.Marks)
	}
}

func TestDumpEconomyProducesAuditableJSON(t *testing.T) {
	var output bytes.Buffer
	if err := dumpEconomyConfig(&output); err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(output.Bytes(), &config); err != nil {
		t.Fatalf("dump is not JSON: %v", err)
	}
	for _, section := range []string{"payouts", "energy", "camp_draw_and_catalog", "skills", "chromas", "bundles", "achievements", "achievement_sets"} {
		if _, ok := config[section]; !ok {
			t.Errorf("dump is missing %q", section)
		}
	}
}

func TestBuyHeroWithCrownsAndRecordHeroProgress(t *testing.T) {
	p := &profile.Profile{
		Crowns:         1200,
		UnlockedHeroes: []string{"warden", "hunter", "anchor"},
		HeroPlayCounts: make(map[string]int),
		HeroWinCounts:  make(map[string]int),
	}
	treasurer, _ := heroes.ByID("treasurer")
	if err := heroes.BuyHeroWithCrowns(p, treasurer.ID); err != nil {
		t.Fatal(err)
	}
	if p.Crowns != 0 || p.Marks != 0 || !containsHeroID(p.UnlockedHeroes, treasurer.ID) {
		t.Fatalf("purchase left %d Crowns, %d Marks and heroes %v", p.Crowns, p.Marks, p.UnlockedHeroes)
	}
	if err := p.BuyHeroWithCrowns("recruit"); err == nil {
		t.Fatal("Recruit must not be purchasable")
	}

	result := game.MatchResult{Games: []game.MatchGame{
		{Won: true, PlayerHeroID: "treasurer"},
		{Won: false, PlayerHeroID: "treasurer"},
	}}
	recordHeroProgress(p, result, "treasurer", "hunter")
	if p.HeroPlayCounts["treasurer"] != 2 || p.HeroWinCounts["treasurer"] != 1 ||
		p.LastHeroPicked != "treasurer" || p.OpponentLastHeroID != "hunter" ||
		p.LastOpponentHeroID != "hunter" || len(p.LastOpponentHeroIDs) != 1 ||
		p.LastOpponentHeroIDs[0] != "hunter" {
		t.Fatalf("hero progression not recorded: %+v", p)
	}
}
