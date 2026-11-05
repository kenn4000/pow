package app

import (
	"net/http"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/profile"
)

func TestShopConsumableOffersAndPurchases(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 20000
	p.Crowns = 500
	web := &webServer{playerProfile: p}

	wantOffers := map[string]webShopConsumableOffer{
		"camp-draw-one":     {ID: "camp-draw-one", Quantity: 1, MarksCost: 1500, CrownsCost: 75},
		"camp-draw-three":   {ID: "camp-draw-three", Quantity: 3, MarksCost: 4000, CrownsCost: 200},
		"restart-one":       {ID: "restart-one", Quantity: 1, MarksCost: 400, CrownsCost: 20},
		"restart-six":       {ID: "restart-six", Quantity: 6, MarksCost: 2000, CrownsCost: 100},
		"undo-one":          {ID: "undo-one", Quantity: 1, MarksCost: 250, CrownsCost: 12},
		"undo-ten":          {ID: "undo-ten", Quantity: 10, MarksCost: 2000, CrownsCost: 100},
		"replace-card-one":  {ID: "replace-card-one", Quantity: 1, MarksCost: 180, CrownsCost: 9},
		"replace-card-ten":  {ID: "replace-card-ten", Quantity: 10, MarksCost: 1500, CrownsCost: 75},
		"campaign-skip-one": {ID: "campaign-skip-one", Quantity: 1, MarksCost: 1500, CrownsCost: 75},
	}
	offers := web.shopPage(p).ConsumableOffers
	if len(offers) != len(wantOffers) {
		t.Fatalf("got %d consumable offers, want %d", len(offers), len(wantOffers))
	}
	for _, offer := range offers {
		want, ok := wantOffers[offer.ID]
		if !ok || offer.Quantity != want.Quantity || offer.MarksCost != want.MarksCost ||
			offer.CrownsCost != want.CrownsCost {
			t.Errorf("offer = %+v, want %+v", offer, want)
		}
	}

	purchases := []actionRequest{
		{Action: "buy-consumable", ID: "camp-draw-three", Payment: "marks"},
		{Action: "buy-consumable", ID: "restart-six", Payment: "crowns"},
		{Action: "buy-consumable", ID: "undo-ten", Payment: "marks"},
		{Action: "buy-consumable", ID: "replace-card-one", Payment: "crowns"},
		{Action: "buy-consumable", ID: "campaign-skip-one", Payment: "crowns"},
	}
	for _, purchase := range purchases {
		if _, err := web.applyShopAction(p, purchase); err != nil {
			t.Fatalf("purchase %s: %v", purchase.ID, err)
		}
	}
	if p.DrawsAvailable != 3 || p.RestartMissionTokens != 6 || p.UndoMoveTokens != 10 ||
		p.ReplaceCardTokens != 1 || p.CampaignSkipTokens != 1 ||
		p.Marks != 14000 || p.Crowns != 316 {
		t.Fatalf("purchases left profile in unexpected state: %+v", p)
	}
	if _, err := web.applyShopAction(p, actionRequest{
		Action: "buy-consumable", ID: "camp-draw-one", Payment: "marks",
	}); err == nil {
		t.Fatal("Camp Draw purchase exceeded the storage limit")
	}
	if _, err := web.applyShopAction(p, actionRequest{
		Action: "buy-consumable", ID: "undo-one", Payment: "invalid",
	}); err == nil {
		t.Fatal("consumable purchase accepted an unsupported payment")
	}
}

func TestShopConsumablePurchasePersistsInventory(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 400
	p.Crowns = 9
	path := filepath.Join(t.TempDir(), "profile.json")
	p.SetSaveHandler(func(saved *profile.Profile) error { return saved.SaveTo(path) })
	web := &webServer{playerProfile: p}

	response := callDashboardHandler(t, web.handleShop, http.MethodPost, "/api/shop",
		map[string]string{"action": "buy-consumable", "id": "restart-one", "payment": "marks"})
	if response.Code != http.StatusOK {
		t.Fatalf("purchase status = %d: %s", response.Code, response.Body.String())
	}
	response = callDashboardHandler(t, web.handleShop, http.MethodPost, "/api/shop",
		map[string]string{"action": "buy-consumable", "id": "replace-card-one", "payment": "crowns"})
	if response.Code != http.StatusOK {
		t.Fatalf("Replace Any Card purchase status = %d: %s", response.Code, response.Body.String())
	}
	saved, err := profile.LoadProfileFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Marks != 0 || saved.RestartMissionTokens != 1 ||
		saved.ReplaceCardTokens != 1 || saved.Crowns != 0 {
		t.Fatalf("saved purchase state: marks=%d crowns=%d restart tokens=%d replace tokens=%d",
			saved.Marks, saved.Crowns, saved.RestartMissionTokens, saved.ReplaceCardTokens)
	}
}

func TestCampaignSkipConsumesTokenAndCompletesNextMission(t *testing.T) {
	web := campaignTestServer(t)
	web.playerProfile.CampaignSkipTokens = 1
	energyBefore := web.playerProfile.Energy
	marksBefore := web.playerProfile.Marks

	response := callDashboardHandler(t, web.handleCampaigns, http.MethodPost, "/api/campaigns",
		map[string]any{"action": "skip-mission", "campaignId": "basic", "missionIndex": 1})
	if response.Code != http.StatusOK {
		t.Fatalf("skip status = %d: %s", response.Code, response.Body.String())
	}
	progress := web.playerProfile.Campaigns["basic"]
	if web.playerProfile.CampaignSkipTokens != 0 || progress.MissionsCompleted != 1 ||
		!missionIsComplete(progress, 1) || web.playerProfile.Marks != marksBefore ||
		web.playerProfile.Energy != energyBefore {
		t.Fatalf("skip state: tokens=%d progress=%+v Marks=%d Energy=%d",
			web.playerProfile.CampaignSkipTokens, progress, web.playerProfile.Marks, web.playerProfile.Energy)
	}
	if !campaigns.CanPlayMission(web.playerProfile, "basic", 2) {
		t.Fatal("skipping the first mission did not unlock the next mission")
	}
	if response := callDashboardHandler(t, web.handleCampaigns, http.MethodPost, "/api/campaigns",
		map[string]any{"action": "skip-mission", "campaignId": "basic", "missionIndex": 1}); response.Code != http.StatusBadRequest {
		t.Fatalf("repeated skip status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestInGameCampaignSkipAdvancesWithoutRewards(t *testing.T) {
	web := campaignTestServer(t)
	web.playerProfile.CampaignSkipTokens = 1
	web.playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
	}
	marksBefore, crownsBefore, energyBefore := web.playerProfile.Marks,
		web.playerProfile.Crowns, web.playerProfile.Energy
	response := callDashboardHandler(t, web.handleAction, http.MethodPost, "/api/action",
		map[string]string{"action": "use-campaign-skip-token"})
	if response.Code != http.StatusOK {
		t.Fatalf("in-game skip status = %d: %s", response.Code, response.Body.String())
	}
	progress := web.playerProfile.Campaigns["basic"]
	if web.playerProfile.CampaignSkipTokens != 0 || progress.MissionsCompleted != 1 ||
		!missionIsComplete(progress, 1) || web.session.phase != "gameover" ||
		!web.session.campaign.HasNextMission || web.playerProfile.Marks != marksBefore ||
		web.playerProfile.Crowns != crownsBefore || web.playerProfile.Energy != energyBefore {
		t.Fatalf("in-game skip state: tokens=%d progress=%+v phase=%q campaign=%+v Marks=%d Crowns=%d Energy=%d",
			web.playerProfile.CampaignSkipTokens, progress, web.session.phase, web.session.campaign,
			web.playerProfile.Marks, web.playerProfile.Crowns, web.playerProfile.Energy)
	}
}

func TestSkippingFinalMissionCompletesCampaignWithoutCompletionRewards(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	campaign, ok := campaigns.Find("basic")
	if !ok || len(campaign.Missions) == 0 {
		t.Fatal("basic campaign is unavailable")
	}
	lastMission := campaign.Missions[len(campaign.Missions)-1]
	completed := make([]int, 0, len(campaign.Missions)-1)
	for _, mission := range campaign.Missions[:len(campaign.Missions)-1] {
		completed = append(completed, mission.Index)
	}
	progress := &profile.CampaignProgress{
		CampaignID: campaign.ID, TotalMissions: len(campaign.Missions),
		MissionsCompleted: len(completed), CompletedMissionIDs: completed,
	}
	p.Campaigns = map[string]*profile.CampaignProgress{campaign.ID: progress}
	p.Marks, p.Crowns, p.HeroBoxesAvailable, p.SkillPoints = 4321, 123, 2, 4
	marks, crowns, boxes, points := p.Marks, p.Crowns, p.HeroBoxesAvailable, p.SkillPoints
	unlocked := append([]string(nil), p.UnlockedHeroes...)

	outcome, err := campaigns.SkipMission(p, campaign.ID, lastMission.Index)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.CampaignCompleted || !progress.Completed || progress.CompletionRewardsAwarded ||
		p.Marks != marks || p.Crowns != crowns || p.HeroBoxesAvailable != boxes ||
		p.SkillPoints != points || !reflect.DeepEqual(p.UnlockedHeroes, unlocked) {
		t.Fatalf("final skip awarded rewards or failed progression: outcome=%+v progress=%+v profile=%+v",
			outcome, progress, p)
	}
}

func TestRestartAndUndoTokensWorkInCampaignAndRejectHumanVersusHuman(t *testing.T) {
	web := campaignTestServer(t)
	web.playerProfile.RestartMissionTokens = 1
	web.playerProfile.UndoMoveTokens = 1
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
	}
	initialState, err := cloneWebValue(web.session.state)
	if err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "commit" {
		t.Fatalf("initial mission phase = %q, want commit", web.session.phase)
	}

	response := callDashboardHandler(t, web.handleAction, http.MethodPost, "/api/action",
		map[string]any{"action": "commit", "cards": []int{0, 1, 2}})
	if response.Code != http.StatusOK {
		t.Fatalf("commit status = %d: %s", response.Code, response.Body.String())
	}
	if !web.view().Dashboard.CanUndo {
		t.Fatal("successful move did not expose Undo")
	}
	response = callDashboardHandler(t, web.handleAction, http.MethodPost, "/api/action",
		map[string]string{"action": "use-undo-token"})
	if response.Code != http.StatusOK {
		t.Fatalf("undo status = %d: %s", response.Code, response.Body.String())
	}
	if web.playerProfile.UndoMoveTokens != 0 || web.session.phase != "commit" ||
		!reflect.DeepEqual(web.session.state, initialState) {
		t.Fatalf("undo did not restore the previous move: tokens=%d phase=%q", web.playerProfile.UndoMoveTokens, web.session.phase)
	}

	response = callDashboardHandler(t, web.handleAction, http.MethodPost, "/api/action",
		map[string]string{"action": "use-restart-token"})
	if response.Code != http.StatusOK {
		t.Fatalf("restart status = %d: %s", response.Code, response.Body.String())
	}
	if web.playerProfile.RestartMissionTokens != 0 || web.session.phase != "commit" ||
		!reflect.DeepEqual(web.session.state, initialState) {
		t.Fatalf("restart did not restore the mission: tokens=%d phase=%q", web.playerProfile.RestartMissionTokens, web.session.phase)
	}

	web.playerProfile.RestartMissionTokens = 1
	web.playerProfile.UndoMoveTokens = 1
	hvh := &webServer{
		playerProfile: web.playerProfile,
		session:       &webSession{mode: "hvh", phase: "commit"},
	}
	snapshot, err := cloneWebSession(hvh.session)
	if err != nil {
		t.Fatal(err)
	}
	hvh.session.restartSnapshot = snapshot
	snapshot, err = cloneWebSession(hvh.session)
	if err != nil {
		t.Fatal(err)
	}
	hvh.session.undoSnapshot = snapshot
	for _, action := range []string{"use-restart-token", "use-undo-token"} {
		if err := hvh.applyAction(actionRequest{Action: action}); err == nil {
			t.Errorf("%s was allowed in Human-vs-Human mode", action)
		}
	}
	if web.playerProfile.RestartMissionTokens != 1 || web.playerProfile.UndoMoveTokens != 1 {
		t.Fatal("rejected Human-vs-Human token use consumed inventory")
	}
}

func TestRestartMissionTokenWorksInStandaloneCPUGame(t *testing.T) {
	web := campaignTestServer(t)
	web.playerProfile.RestartMissionTokens = 1
	response := callDashboardHandler(t, web.handleNewGame, http.MethodPost, "/api/new",
		map[string]string{"mode": "normal", "variant": "main", "playerHeroId": "recruit"})
	if response.Code != http.StatusOK {
		t.Fatalf("new CPU game status = %d: %s", response.Code, response.Body.String())
	}
	initialState, err := cloneWebSession(web.session)
	if err != nil {
		t.Fatal(err)
	}
	response = callDashboardHandler(t, web.handleAction, http.MethodPost, "/api/action",
		map[string]string{"action": "use-restart-token"})
	if response.Code != http.StatusOK {
		t.Fatalf("restart status = %d: %s", response.Code, response.Body.String())
	}
	if web.playerProfile.RestartMissionTokens != 0 || web.session.mode != "normal" ||
		!reflect.DeepEqual(web.session.state, initialState.state) {
		t.Fatalf("CPU restart did not restore the match: tokens=%d mode=%q",
			web.playerProfile.RestartMissionTokens, web.session.mode)
	}
}
