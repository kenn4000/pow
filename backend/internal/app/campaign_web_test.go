package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func campaignTestServer(t *testing.T) *webServer {
	t.Helper()
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.SetSaveHandler(func(*profile.Profile) error { return nil })
	if err := (&app{}).initializeProgressionProfile(p); err != nil {
		t.Fatal(err)
	}
	return &webServer{playerProfile: p, rng: rand.New(rand.NewSource(42)), defaultMode: "easy"}
}

func finishTestMission(t *testing.T, web *webServer) {
	t.Helper()
	for step := 0; step < 100 && web.session.phase != "gameover"; step++ {
		request := actionRequest{Action: web.session.phase, Index: 0, Cards: []int{0, 1, 2}}
		switch request.Action {
		case "draft":
			if web.session.actor == 0 {
				request.Action = "draft-pick"
			} else {
				request.Action = "draft-ai-pick"
			}
		case "result":
			request.Action = "continue"
		}
		if err := web.applyAction(request); err != nil {
			t.Fatalf("%s: %v", request.Action, err)
		}
		if err := web.advanceAI(); err != nil {
			t.Fatal(err)
		}
		snapshot := snapshotSession(web.session)
		web.session = snapshot.restore()
	}
	if web.session.phase != "gameover" {
		t.Fatal("mission did not finish")
	}
}

func TestWebCampaignCompletionReplayAndSnapshots(t *testing.T) {
	web := campaignTestServer(t)
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 2}); err == nil {
		t.Fatal("locked mission was allowed")
	}
	energy := web.playerProfile.Energy
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
	}
	if web.session.variant != "main" || web.session.campaign.Variant != "main" {
		t.Fatalf("standard campaign variant = %q / %q", web.session.variant, web.session.campaign.Variant)
	}
	if web.playerProfile.Energy != energy-1 {
		t.Fatal("mission energy was not charged")
	}
	finishTestMission(t, web)
	if web.session.winner != 0 || !missionIsComplete(web.playerProfile.Campaigns["basic"], 1) {
		t.Fatal("first mission did not award completion")
	}
	if len(web.session.battles) != 3 || len(web.session.achievementUpdates) == 0 {
		t.Fatal("mission omitted its persisted battle log or achievement delta")
	}
	if web.session.payout.TotalMarks != 100 {
		t.Fatalf("mission payout = %d", web.session.payout.TotalMarks)
	}
	marks, games, remaining := web.playerProfile.Marks, web.playerProfile.TotalGames, web.playerProfile.Energy
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
	}
	if !web.session.campaign.Replay {
		t.Fatal("completed mission was not marked as practice")
	}
	finishTestMission(t, web)
	if web.playerProfile.Marks != marks || web.playerProfile.TotalGames != games || web.playerProfile.Energy != remaining {
		t.Fatal("practice changed rewards, statistics or energy")
	}
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 2}); err != nil {
		t.Fatal(err)
	}
	finishTestMission(t, web)
	if web.session.state.Clashes != 1 {
		t.Fatalf("scripted mirror produced %d clashes", web.session.state.Clashes)
	}
	if err := web.session.state.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestWebCampaignListingMapsSpecialCatalogAndDisablesHeroStubs(t *testing.T) {
	web := campaignTestServer(t)
	response := httptest.NewRecorder()
	web.handleCampaigns(response, httptest.NewRequest(http.MethodGet, "/api/campaigns", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("campaign listing status = %d: %s", response.Code, response.Body.String())
	}
	var page webCampaignPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]webCampaignSummary, len(page.Campaigns))
	for _, campaign := range page.Campaigns {
		byID[campaign.ID] = campaign
	}
	abandon := byID["abandon"]
	targets := []int{35, 36, 37, 44, 45, 46, 53, 54, 55}
	pairCounts := []int{4, 4, 4, 5, 5, 5, 6, 6, 6}
	boardCounts := []int{11, 12, 13, 11, 12, 13, 11, 12, 13}
	if abandon.Variant != "abandon" || len(abandon.Missions) != 15 {
		t.Fatalf("abandon campaign listing = %+v", abandon)
	}
	for index, mission := range abandon.Missions[:9] {
		if mission.Variant != "abandon" || mission.Target != targets[index] ||
			mission.PairCount != pairCounts[index] || mission.CardCount != boardCounts[index] ||
			mission.Name != fmt.Sprintf("Mission %d", index+1) || !mission.Enabled ||
			mission.RewardMarks != 400 {
			t.Errorf("abandon API mission %d = %+v", index+1, mission)
		}
		if mission.CanPlay != (index == 0) {
			t.Errorf("abandon mission %d canPlay = %v", index+1, mission.CanPlay)
		}
	}
	for _, mission := range abandon.Missions[9:] {
		if mission.Enabled || mission.CanPlay || mission.Status != "Coming Soon" {
			t.Errorf("Abandon stub mission = %+v", mission)
		}
	}
	puzzler := byID["puzzler"]
	puzzleIDs := []string{
		"gp6_balanced", "gp6_hunters_edge", "gp6_bounty_trap", "gp6_narrow_gate",
		"gp6_bodybuilders_wall", "gp8_balanced", "gp8_bounty_climb", "gp8_forgiving",
		"gp8_gamblers_puzzle", "gp8_tight", "gp10_balanced", "gp10_gorilla_threat",
		"gp10_turtle_lead", "gp10_low_score", "gp10_everyone_solves", "gp12_balanced",
		"gp12_dual_threat", "gp12_shift_dominant", "gp12_forgiving", "gp12_inverted",
	}
	if puzzler.Variant != "puzzler" || len(puzzler.Missions) != len(puzzleIDs) {
		t.Fatalf("puzzler campaign listing = %+v", puzzler)
	}
	for index, mission := range puzzler.Missions {
		if mission.VariantID != puzzleIDs[index] || mission.Variant != "puzzler" ||
			mission.Target <= 0 || mission.CardCount <= 0 || mission.RewardMarks != 400 {
			t.Errorf("puzzler API mission %d = %+v", index+1, mission)
		}
	}
	for _, id := range []string{"warden", "hunter", "treasurer", "ascendant", "anchor", "gambler", "diplomat", "ascetic", "herald", "hater"} {
		campaign := byID[id]
		if campaign.ID == "" || campaign.Status != "Coming Soon" ||
			campaign.Description != "Coming Soon" || len(campaign.Missions) != 1 {
			t.Errorf("hero campaign %q metadata = %+v", id, campaign)
			continue
		}
		mission := campaign.Missions[0]
		if mission.Name != "Coming Soon" || mission.Description != "Coming Soon" ||
			mission.Status != "Coming Soon" || mission.Enabled || mission.CanPlay {
			t.Errorf("hero campaign %q mission = %+v", id, mission)
		}
		if err := web.prepareWebMission(actionRequest{CampaignID: id, MissionIndex: 1}); err == nil {
			t.Errorf("Coming Soon campaign %q launched", id)
		}
	}
}

func TestSuccessfulSpecialCampaignMissionAwardsMarksOnlyOnce(t *testing.T) {
	web := campaignTestServer(t)
	beforeMarks, beforeEnergy := web.playerProfile.Marks, web.playerProfile.Energy
	body, _ := json.Marshal(map[string]any{
		"campaignId": "puzzler", "missionIndex": 1, "playerHeroId": "recruit",
	})
	response := httptest.NewRecorder()
	web.handleNewGame(response, httptest.NewRequest(http.MethodPost, "/api/new", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("campaign launch status = %d: %s", response.Code, response.Body.String())
	}
	if web.session.variant != "puzzler" || web.session.campaign == nil ||
		web.session.campaign.Variant != "puzzler" ||
		web.session.campaign.VariantID != "gp6_balanced" {
		t.Fatalf("campaign puzzle session context = %+v", web.session)
	}
	snapshotJSON, err := json.Marshal(snapshotSession(web.session))
	if err != nil {
		t.Fatal(err)
	}
	var persisted sessionSnapshot
	if err := json.Unmarshal(snapshotJSON, &persisted); err != nil {
		t.Fatal(err)
	}
	restored := persisted.restore()
	if restored.campaign == nil || restored.campaign.Variant != "puzzler" ||
		restored.campaign.VariantID != "gp6_balanced" {
		t.Fatalf("campaign context was not persisted: %+v", restored.campaign)
	}
	hero, _ := heroes.ByID("recruit")
	web.session.puzzle = newPuzzleSession(puzzleDefinition{
		ID: "gp6_balanced", Cards: []game.Card{
			{Suit: game.Shield, Face: 2}, {Suit: game.Shield, Face: 2},
		},
		Par: 0, Direction: "low",
	}, hero)
	for _, index := range []int{0, 1} {
		if err := web.applyAction(actionRequest{Action: "puzzle-pick", Index: index}); err != nil {
			t.Fatal(err)
		}
	}
	if web.session.phase != "gameover" || web.session.winner != 0 ||
		!missionIsComplete(web.playerProfile.Campaigns["puzzler"], 1) ||
		!campaigns.CanPlayMission(web.playerProfile, "puzzler", 2) {
		t.Fatalf("successful puzzle did not advance campaign: phase=%s progress=%+v",
			web.session.phase, web.playerProfile.Campaigns["puzzler"])
	}
	if web.playerProfile.Marks != beforeMarks+400 || web.playerProfile.Energy != beforeEnergy-1 {
		t.Fatalf("puzzle campaign reward/energy = marks %d→%d, energy %d→%d",
			beforeMarks, web.playerProfile.Marks, beforeEnergy, web.playerProfile.Energy)
	}
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "puzzler", MissionIndex: 1, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	if !web.session.campaign.Replay {
		t.Fatal("completed puzzle mission was not marked as a replay")
	}
	web.finishSpecialCampaignMission(true)
	if web.playerProfile.Marks != beforeMarks+400 {
		t.Fatalf("puzzle replay changed Marks to %d, want %d", web.playerProfile.Marks, beforeMarks+400)
	}
}

func TestAbandonCampaignLaunchUsesRandomMissionAndPersistsCampaignContext(t *testing.T) {
	web := campaignTestServer(t)
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "abandon", MissionIndex: 1, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	if web.session.variant != "abandon" || web.session.abandon == nil ||
		web.session.abandon.RulesVersion != abandonRulesVersion ||
		web.session.abandon.PairCount != 4 || len(web.session.abandon.Board) != 11 ||
		web.session.abandon.Target != 35 || web.session.phase != "abandon-reveal" ||
		web.session.campaign == nil || web.session.campaign.Variant != "abandon" ||
		web.session.campaign.VariantID != "" || !web.session.campaign.HasNextMission {
		t.Fatalf("abandon campaign launch = %+v", web.session)
	}
	restored := snapshotSession(web.session).restore()
	if restored.campaign == nil || restored.campaign.VariantID != "" ||
		restored.abandon.RulesVersion != abandonRulesVersion ||
		len(restored.abandon.PersonalPile) != 5 {
		t.Fatalf("abandon campaign snapshot = %+v", restored)
	}
	marks := web.playerProfile.Marks
	web.session.abandon.Finished, web.session.abandon.Won = true, true
	web.updateAbandonPhase()
	if !missionIsComplete(web.playerProfile.Campaigns["abandon"], 1) ||
		!campaigns.CanPlayMission(web.playerProfile, "abandon", 2) ||
		web.playerProfile.Marks != marks+400 || !web.session.campaign.HasNextMission {
		t.Fatalf("successful Abandon completion did not advance with 400 Marks: %+v",
			web.playerProfile.Campaigns["abandon"])
	}
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "abandon", MissionIndex: 1, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	if !web.session.campaign.Replay {
		t.Fatal("completed Abandon mission was not marked as a replay")
	}
	web.finishSpecialCampaignMission(true)
	if web.playerProfile.Marks != marks+400 {
		t.Fatalf("Abandon replay changed Marks to %d, want %d", web.playerProfile.Marks, marks+400)
	}
}

func TestAbandonWebActionsRequireFlipThenTwoCards(t *testing.T) {
	web := campaignTestServer(t)
	if err := web.prepareWebMission(actionRequest{CampaignID: "abandon", MissionIndex: 1, PlayerHeroID: "recruit"}); err != nil {
		t.Fatal(err)
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: abandonBoardSource, Index: 0}); err == nil {
		t.Fatal("pairing was allowed before the reveal phase")
	}
	hiddenIndex := -1
	for index, revealed := range web.session.abandon.Revealed {
		if !revealed {
			hiddenIndex = index
			break
		}
	}
	if hiddenIndex < 0 {
		t.Fatal("random board has no face-down card")
	}
	if err := web.applyAction(actionRequest{Action: "abandon-flip", Index: hiddenIndex}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-pair-first" || !web.session.abandon.Revealed[hiddenIndex] {
		t.Fatalf("reveal action did not advance to pairing: phase=%q", web.session.phase)
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: abandonBoardSource, Index: 0}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-pair-second" {
		t.Fatalf("first card selection phase = %q", web.session.phase)
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: abandonBoardSource, Index: 0}); err == nil {
		t.Fatal("same card was allowed twice in one pairing")
	}
	secondSource, secondIndex := abandonPileSource, -1
	for index, revealed := range web.session.abandon.PileRevealed {
		if !revealed && !web.session.abandon.PileUsed[index] {
			secondIndex = index
			break
		}
	}
	if secondIndex < 0 {
		secondSource = abandonBoardSource
		for index, revealed := range web.session.abandon.Revealed {
			if index != 0 && !revealed && !web.session.abandon.Used[index] {
				secondIndex = index
				break
			}
		}
	}
	if secondIndex < 0 {
		t.Fatal("no hidden second card was available")
	}
	if err := web.applyAction(actionRequest{Action: "abandon-reveal-pair-card", Source: secondSource, Index: secondIndex}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-pair-second" || len(web.session.abandon.Pairings) != 0 {
		t.Fatalf("revealing the second card resolved the pair: phase=%q pairs=%d",
			web.session.phase, len(web.session.abandon.Pairings))
	}
	revealedView := web.session.abandon.webView()
	secondPile := revealedView.PersonalPile
	if secondSource == abandonBoardSource {
		secondPile = revealedView.Board
	}
	if secondPile[secondIndex].Card == nil {
		t.Fatal("revealed second card was not included in the Abandon view")
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: secondSource, Index: secondIndex}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-reveal" || len(web.session.abandon.Pairings) != 1 {
		t.Fatalf("pair action did not advance to next turn: phase=%q pairs=%d",
			web.session.phase, len(web.session.abandon.Pairings))
	}
}

func TestStandalonePuzzlerRequiresCampaignMission(t *testing.T) {
	web := campaignTestServer(t)
	body, _ := json.Marshal(map[string]any{
		"mode": "easy", "variant": "puzzler", "puzzleId": "gp6_balanced",
		"playerHeroId": "recruit",
	})
	response := httptest.NewRecorder()
	web.handleNewGame(response, httptest.NewRequest(http.MethodPost, "/api/new", bytes.NewReader(body)))
	if response.Code != http.StatusBadRequest || web.session != nil {
		t.Fatalf("standalone puzzler status/session = %d / %+v: %s",
			response.Code, web.session, response.Body.String())
	}
}

func TestEveryWebCampaignMissionCompletesWithRestoredState(t *testing.T) {
	for _, campaign := range campaigns.All() {
		for _, mission := range campaign.Missions {
			if mission.Variant != "" || mission.Disabled {
				continue
			}
			t.Run(campaign.ID+"/"+mission.Name, func(t *testing.T) {
				web := campaignTestServer(t)
				for _, prerequisite := range campaigns.All() {
					if prerequisite.ID == campaign.ID {
						break
					}
					for _, previous := range prerequisite.Missions {
						if _, err := campaigns.CompleteMission(web.playerProfile, prerequisite.ID, previous.Index, 0); err != nil {
							t.Fatal(err)
						}
					}
				}
				for _, previous := range campaign.Missions {
					if previous.Index >= mission.Index {
						break
					}
					if _, err := campaigns.CompleteMission(web.playerProfile, campaign.ID, previous.Index, 0); err != nil {
						t.Fatal(err)
					}
				}
				if err := web.prepareWebMission(actionRequest{CampaignID: campaign.ID, MissionIndex: mission.Index}); err != nil {
					t.Fatal(err)
				}
				finishTestMission(t, web)
				if err := web.session.state.Validate(); err != nil {
					t.Fatal(err)
				}
				wantRounds := 3
				if mission.Type == campaigns.SingleRound {
					wantRounds = 1
				}
				if web.session.state.Round != wantRounds {
					t.Fatalf("finished in round %d, want %d", web.session.state.Round, wantRounds)
				}
				if mission.Name == "No Draft Duel" && web.session.battles[0].Attacker == "You" {
					t.Fatal("No Draft Duel did not make the player defend first")
				}
				if mission.Name == "Harsh" && web.session.battles[0].Attacker != "You" {
					t.Fatal("Harsh did not make the player attack first in Round 1")
				}
				if mission.PlayerAttacksFirstAll {
					for round := 0; round < 3; round++ {
						if got := web.session.battles[round*3].Attacker; got != "You" {
							t.Errorf("round %d first attacker = %q, want You", round+1, got)
						}
					}
				}
			})
		}
	}
}
