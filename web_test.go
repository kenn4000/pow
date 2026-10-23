package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"prisonersofwar/energy"
	"prisonersofwar/profile"
)

func TestWebGameFlowRedactsHandsAndAutoPlaysThirdBattle(t *testing.T) {
	web := &webServer{rng: rand.New(rand.NewSource(51))}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", web.handleState)
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	view := sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{"mode": "hvh"})
	if !view.Started || view.Phase != "commit" {
		t.Fatalf("new game state = started:%t phase:%q", view.Started, view.Phase)
	}
	if view.Modifiers.Reckoning == [3]int{} || view.PlayerHero == "" || view.OpponentHero == "" {
		t.Fatalf("game start omitted hero modifiers: %+v", view)
	}
	if len(view.Players[0].Hand) == 0 || len(view.Players[1].Hand) != 0 || view.Players[1].HiddenHand == 0 {
		t.Fatal("initial view did not reveal only Player 1's hand")
	}
	for _, player := range view.Players {
		want := 5
		if player.Color == "White" {
			want = 6
		}
		handCount := len(player.Hand)
		if handCount == 0 {
			handCount = player.HiddenHand
		}
		if handCount != want {
			t.Fatalf("%s player has %d cards at setup, want %d", player.Color, handCount, want)
		}
	}

	attacks, defenses := 0, 0
	for step := 0; step < 80 && view.Phase != "gameover"; step++ {
		switch view.Phase {
		case "commit":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "commit", "cards": []int{0, 1, 2},
			})
		case "handoff":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "ready"})
		case "attack":
			attacks++
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "attack", "index": 0,
			})
		case "defend":
			defenses++
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "defend", "index": 0,
			})
		case "result":
			if view.Players[view.Actor].HiddenHand != 0 {
				t.Fatal("active player's hand was hidden during the battle result")
			}
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "continue"})
		case "seat":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "seat", "seat": "X"})
		default:
			t.Fatalf("unexpected UI phase %q", view.Phase)
		}
	}
	if view.Phase != "gameover" {
		t.Fatalf("game did not finish; stopped in phase %q", view.Phase)
	}
	if attacks != 6 || defenses != 6 {
		t.Fatalf("manual attack/defense actions = %d/%d; third battles should be automatic", attacks, defenses)
	}
	if got := len(web.session.state.P1.Jail) + len(web.session.state.P2.Jail) + web.session.state.Clashes; got != 9 {
		t.Fatalf("resolved %d of 9 battles", got)
	}
	if err := web.session.state.Validate(); err != nil {
		t.Fatalf("finished game state is invalid: %v", err)
	}

	view = sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{"mode": "hard"})
	if view.Phase != "commit" || view.Mode != "hard" || len(view.Players[1].Hand) != 0 || view.Players[1].HiddenHand == 0 {
		t.Fatal("new AI game did not reset or exposed the AI hand")
	}
}

func TestWebHeroSelectionRejectsLockedHeroes(t *testing.T) {
	web := &webServer{rng: rand.New(rand.NewSource(55))}
	request := httptest.NewRequest(http.MethodPost, "/api/new", bytes.NewBufferString(
		`{"mode":"hvh","playerHeroId":"hunter","opponentHeroId":"hater"}`,
	))
	response := httptest.NewRecorder()
	web.handleNewGame(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("locked hero request status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if web.session != nil {
		t.Fatal("invalid hero selection started a game")
	}
}

func sendWebRequest(t *testing.T, client *http.Client, url string, body any) webView {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var payload map[string]string
		_ = json.NewDecoder(response.Body).Decode(&payload)
		t.Fatalf("request failed (%d): %s", response.StatusCode, payload["error"])
	}
	var view webView
	if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	return view
}

func TestWebStateEndpointReturnsWaitingState(t *testing.T) {
	web := &webServer{rng: rand.New(rand.NewSource(52))}
	request := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	response := httptest.NewRecorder()
	web.handleState(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var view webView
	if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if view.Started {
		t.Fatalf("expected a waiting game state, got phase %q", view.Phase)
	}
}

func TestWebWaitingStateIncludesLiveDashboardData(t *testing.T) {
	now := time.Now()
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.PlayerID = "commander123456"
	p.Marks = 12340
	p.Crowns = 145
	p.Energy = 7
	p.EnergyUpdatedAt = now
	p.AdsWatchedToday = 2
	p.LoginStreakCurrent = 12
	p.LoginStreakBest = 30
	p.WinsSinceCampDraw = 4
	p.DrawsAvailable = 3
	p.PendingBoxes["wooden"] = 2
	p.SkillPoints = 1
	p.AchievementTiers["aggressor"] = 1
	web := &webServer{playerProfile: p}

	dashboard := web.view().Dashboard
	if dashboard == nil {
		t.Fatal("waiting state omitted dashboard data")
	}
	if dashboard.Commander != p.PlayerID[:8] || dashboard.Title != heroName(p.DefaultHeroID) ||
		dashboard.Marks != p.Marks || dashboard.Crowns != p.Crowns ||
		dashboard.Energy != 7 || dashboard.MaxEnergy != p.MaxEnergy ||
		dashboard.AdsLeft != energy.DailyAdLimit-2 ||
		dashboard.LoginStreakCurrent != 12 || dashboard.LoginStreakBest != 30 ||
		dashboard.WinsSinceCampDraw != 4 || dashboard.WinsPerCampDraw != 10 ||
		dashboard.DrawsAvailable != 3 || dashboard.PendingBoxes != 2 || dashboard.SkillPoints != 1 ||
		dashboard.UnseenAchievements != 1 {
		t.Fatalf("dashboard data = %+v", dashboard)
	}
}

func TestWebDashboardFeatureActionsUseTemporaryPreviewProfile(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 500
	p.AchievementTiers["aggressor"] = 1
	web := &webServer{rng: rand.New(rand.NewSource(71)), playerProfile: p}

	campResponse := callDashboardHandler(t, web.handleCampDraw, http.MethodGet, "/api/camp-draw?preview=dashboard", nil)
	var campPage webCampDrawPage
	if err := json.Unmarshal(campResponse.Body.Bytes(), &campPage); err != nil {
		t.Fatal(err)
	}
	if campPage.DrawsAvailable != 1 {
		t.Fatalf("preview Camp Draws = %d, want 1", campPage.DrawsAvailable)
	}
	campResponse = callDashboardHandler(t, web.handleCampDraw, http.MethodPost, "/api/camp-draw?preview=dashboard",
		map[string]string{"action": "start-draw"})
	if campResponse.Code != http.StatusOK {
		t.Fatalf("Camp Draw status = %d: %s", campResponse.Code, campResponse.Body.String())
	}
	if err := json.Unmarshal(campResponse.Body.Bytes(), &campPage); err != nil {
		t.Fatal(err)
	}
	if !campPage.Started || campPage.Prompt == nil || campPage.Result != nil || campPage.DrawsAvailable != 1 {
		t.Fatalf("Camp Draw did not begin interactively: %+v", campPage)
	}
	for exchange := 0; exchange < 3; exchange++ {
		campResponse = callDashboardHandler(t, web.handleCampDraw, http.MethodPost, "/api/camp-draw?preview=dashboard",
			map[string]any{"action": "play-card", "index": 0})
		if campResponse.Code != http.StatusOK {
			t.Fatalf("Camp Draw exchange %d status = %d: %s", exchange+1, campResponse.Code, campResponse.Body.String())
		}
		if err := json.Unmarshal(campResponse.Body.Bytes(), &campPage); err != nil {
			t.Fatal(err)
		}
	}
	if campPage.Result == nil || len(campPage.Result.Exchanges) != 3 || campPage.DrawsAvailable != 0 {
		t.Fatalf("Camp Draw did not finish after three selections: %+v", campPage)
	}
	if p.DrawsAvailable != 0 {
		t.Fatalf("preview Camp Draw changed saved profile draws to %d", p.DrawsAvailable)
	}

	skillResponse := callDashboardHandler(t, web.handleSkills, http.MethodPost, "/api/skills?preview=dashboard",
		map[string]string{"action": "unlock", "id": "quartermaster", "payment": "skill-point"})
	if skillResponse.Code != http.StatusOK {
		t.Fatalf("skill unlock status = %d: %s", skillResponse.Code, skillResponse.Body.String())
	}
	var skillPage webSkillsPage
	if err := json.Unmarshal(skillResponse.Body.Bytes(), &skillPage); err != nil {
		t.Fatal(err)
	}
	if skillPage.Dashboard.SkillPoints != 1 || !web.previewProfile.SkillNodes["quartermaster"] {
		t.Fatalf("preview skill unlock did not apply: %+v", skillPage)
	}
	if p.SkillPoints != 0 || p.SkillNodes["quartermaster"] {
		t.Fatal("preview skill unlock changed saved profile")
	}

	previewMarksBeforeShop := web.previewProfile.Marks
	shopResponse := callDashboardHandler(t, web.handleShop, http.MethodPost, "/api/shop?preview=dashboard",
		map[string]string{"action": "buy-chroma", "id": "crimson"})
	if shopResponse.Code != http.StatusOK {
		t.Fatalf("chroma purchase status = %d: %s", shopResponse.Code, shopResponse.Body.String())
	}
	if p.Marks != 500 || web.previewProfile.Marks != previewMarksBeforeShop-250 ||
		!web.previewProfile.OwnedChromas["crimson"] {
		t.Fatalf("preview shop purchase affected wrong profile: saved=%d preview=%d", p.Marks, web.previewProfile.Marks)
	}

	achievementResponse := callDashboardHandler(t, web.handleAchievements, http.MethodGet,
		"/api/achievements?preview=dashboard", nil)
	if achievementResponse.Code != http.StatusOK {
		t.Fatalf("achievements status = %d: %s", achievementResponse.Code, achievementResponse.Body.String())
	}
	var achievementPage webAchievementsPage
	if err := json.Unmarshal(achievementResponse.Body.Bytes(), &achievementPage); err != nil {
		t.Fatal(err)
	}
	if achievementPage.Dashboard.UnseenAchievements != 0 ||
		len(web.previewProfile.SeenAchievementIDs) != 1 || len(p.SeenAchievementIDs) != 0 {
		t.Fatalf("achievement view state saved to wrong profile: %+v", achievementPage.Dashboard)
	}

	heroResponse := callDashboardHandler(t, web.handleHeroes, http.MethodPost, "/api/heroes?preview=dashboard",
		map[string]string{"action": "select", "id": "recruit"})
	if heroResponse.Code != http.StatusOK || p.DefaultHeroID != profile.DefaultHeroID {
		t.Fatalf("hero selection response=%d saved default=%q: %s", heroResponse.Code, p.DefaultHeroID, heroResponse.Body.String())
	}
}

func callDashboardHandler(
	t *testing.T,
	handler http.HandlerFunc,
	method, url string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, url, &requestBody)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}

func TestWebProfileCampaignAndHistoryPagesUseProfileData(t *testing.T) {
	now := time.Date(2026, time.May, 12, 12, 0, 0, 0, time.Local)
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.PlayerID = "commander123456"
	p.TotalGames = 3
	p.TotalWins = 2
	p.TotalLosses = 1
	p.CurrentStreak = 2
	p.BestStreak = 4
	p.Campaigns["basic"] = &profile.CampaignProgress{
		CampaignID: "basic", Started: true, MissionsCompleted: 1,
		TotalMissions: 10, CompletedMissionIDs: []int{1},
	}
	p.RecordMatch(profile.MatchRecord{
		At: now, Mode: 0, PlayerScore: 15, OppScore: 8, Won: true,
		BountySuit: "Blades", PlayerPrisoners: 4, Clashes: 1, MarksEarned: 120,
	})
	web := &webServer{playerProfile: p}

	profileResponse := callDashboardHandler(t, web.handleProfile, http.MethodGet, "/api/profile", nil)
	var commanderPage webCommanderPage
	if err := json.Unmarshal(profileResponse.Body.Bytes(), &commanderPage); err != nil {
		t.Fatal(err)
	}
	if profileResponse.Code != http.StatusOK || commanderPage.TotalGames != 3 ||
		commanderPage.TotalWins != 2 || commanderPage.BestStreak != 4 ||
		commanderPage.PlayerID != p.PlayerID {
		t.Fatalf("commander profile did not reflect player stats: status=%d page=%+v",
			profileResponse.Code, commanderPage)
	}

	campaignResponse := callDashboardHandler(t, web.handleCampaigns, http.MethodGet, "/api/campaigns", nil)
	var campaignPage webCampaignPage
	if err := json.Unmarshal(campaignResponse.Body.Bytes(), &campaignPage); err != nil {
		t.Fatal(err)
	}
	if campaignResponse.Code != http.StatusOK || len(campaignPage.Campaigns) != 2 {
		t.Fatalf("campaign page status=%d campaigns=%d",
			campaignResponse.Code, len(campaignPage.Campaigns))
	}
	basic := campaignPage.Campaigns[0]
	if basic.ID != "basic" || !basic.Unlocked || !basic.Started ||
		basic.MissionsCompleted != 1 || !basic.Missions[0].Completed {
		t.Fatalf("campaign page did not reflect campaign progress: %+v", basic)
	}

	historyResponse := callDashboardHandler(t, web.handleHistory, http.MethodGet, "/api/history", nil)
	var historyPage webHistoryPage
	if err := json.Unmarshal(historyResponse.Body.Bytes(), &historyPage); err != nil {
		t.Fatal(err)
	}
	if historyResponse.Code != http.StatusOK || historyPage.TotalGames != 3 ||
		historyPage.TotalWins != 2 || len(historyPage.Matches) != 1 ||
		historyPage.Matches[0].ModeName != "AI · Easy" ||
		historyPage.Matches[0].Record.MarksEarned != 120 {
		t.Fatalf("history page did not reflect match data: status=%d page=%+v",
			historyResponse.Code, historyPage)
	}

	featured := dailyFeaturedShopItem(p, now)
	if featured == nil || featured.Date != now.Format("2006-01-02") {
		t.Fatalf("daily featured item = %+v", featured)
	}
	if other := dailyFeaturedShopItem(p, now.Add(5*time.Hour)); other == nil || other.ID != featured.ID {
		t.Fatalf("daily shop item changed during the same date: first=%+v later=%+v", featured, other)
	}
	p.OwnedChromas[featured.ID] = true
	if owned := dailyFeaturedShopItem(p, now); owned == nil || owned.ID != featured.ID || !owned.Owned {
		t.Fatalf("daily featured item did not show owned state: %+v", owned)
	}
}

func TestHardAICompletesGameAndContinueActionsAdvance(t *testing.T) {
	web := &webServer{rng: rand.New(rand.NewSource(53))}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	view := sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{"mode": "hard"})
	continues := 0
	for step := 0; step < 60 && view.Phase != "gameover"; step++ {
		switch view.Phase {
		case "commit":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "commit", "cards": []int{0, 1, 2},
			})
		case "attack":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "attack", "index": 0,
			})
		case "defend":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "defend", "index": 0,
			})
		case "result":
			continues++
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "continue"})
		case "seat":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "seat", "seat": "X"})
		default:
			t.Fatalf("unexpected Human-vs-AI phase %q", view.Phase)
		}
		if len(view.Players) != 2 || len(view.Players[1].Hand) != 0 {
			t.Fatal("AI hand was exposed in the browser view")
		}
		if view.Phase == "result" && view.Players[0].HiddenHand != 0 {
			t.Fatal("Human player's own hand was hidden during the battle result")
		}
	}
	if view.Phase != "gameover" {
		t.Fatalf("Human-vs-AI game did not finish; stopped in phase %q", view.Phase)
	}
	if continues != 9 {
		t.Fatalf("continue action advanced %d battles, want 9", continues)
	}
	if err := web.session.state.Validate(); err != nil {
		t.Fatalf("finished AI game state is invalid: %v", err)
	}
}

func TestHumanCanChooseRoundThreeSeat(t *testing.T) {
	for seed := int64(1); seed <= 32; seed++ {
		web := &webServer{rng: rand.New(rand.NewSource(seed))}
		mux := http.NewServeMux()
		mux.HandleFunc("/api/new", web.handleNewGame)
		mux.HandleFunc("/api/action", web.handleAction)
		server := httptest.NewServer(mux)

		view := sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{"mode": "hard"})
		for step := 0; step < 60 && view.Phase != "gameover"; step++ {
			switch view.Phase {
			case "commit":
				view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
					"action": "commit", "cards": []int{0, 1, 2},
				})
			case "attack":
				view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
					"action": "attack", "index": 0,
				})
			case "defend":
				view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
					"action": "defend", "index": 0,
				})
			case "result":
				view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "continue"})
			case "seat":
				if view.Actor != 0 {
					t.Fatalf("AI seat choice was exposed to the player at seed %d", seed)
				}
				view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
					"action": "seat", "seat": "Y",
				})
				server.Close()
				if view.Phase != "commit" || view.Players[0].Seat != "Y" || view.Players[1].Seat != "X" {
					t.Fatalf("seat choice state = phase %q, seats %q/%q", view.Phase, view.Players[0].Seat, view.Players[1].Seat)
				}
				return
			default:
				t.Fatalf("unexpected Human-vs-AI phase %q at seed %d", view.Phase, seed)
			}
		}
		server.Close()
	}
	t.Fatal("no test game presented the human with the Round 3 seat choice")
}
