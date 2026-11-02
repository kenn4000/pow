package app

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

func TestWebGameFlowRedactsHandsAndAutoPlaysThirdBattle(t *testing.T) {
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.UnlockedHeroes = []string{"recruit", "warden", "hunter"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{rng: rand.New(rand.NewSource(51)), playerProfile: playerProfile}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", web.handleState)
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	view := sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{
		"mode": "hvh", "playerHeroId": "warden", "opponentHeroId": "hunter",
	})
	if !view.Started || view.Phase != "handoff" {
		t.Fatalf("new game state = started:%t phase:%q", view.Started, view.Phase)
	}
	if view.Modifiers.Reckoning == [3]int{} || view.PlayerHero == "" || view.OpponentHero == "" {
		t.Fatalf("game start omitted hero modifiers: %+v", view)
	}
	if len(view.Players[0].Hand) != 0 || len(view.Players[1].Hand) != 0 {
		t.Fatal("draft handoff exposed private dealt cards")
	}

	attacks, defenses, draftPicks := 0, 0, 0
	draftCompletionChecked := false
	for step := 0; step < 100 && view.Phase != "gameover"; step++ {
		switch view.Phase {
		case "draft":
			if len(view.DraftPool) != 8-draftPicks || len(view.DraftHand) == 0 {
				t.Fatalf("draft view pick %d has pool=%d hand=%d", draftPicks+1, len(view.DraftPool), len(view.DraftHand))
			}
			action := map[string]any{"action": "draft-pick", "index": 0}
			if view.Mode != "hvh" && view.Actor != 0 {
				action = map[string]any{"action": "draft-ai-pick"}
			}
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", action)
			draftPicks++
		case "commit":
			if draftPicks == 6 && !draftCompletionChecked {
				x, _ := web.session.state.PlayerAtSeat(game.SeatX)
				y, _ := web.session.state.PlayerAtSeat(game.SeatY)
				xPlayer, _ := web.session.state.Player(x)
				yPlayer, _ := web.session.state.Player(y)
				if len(xPlayer.Hand) != 5 || len(yPlayer.Hand) != 5 ||
					len(web.session.state.Deck) != 14 || len(web.session.state.Discard) != 2 {
					t.Fatalf("draft completed with X=%d Y=%d draw pile=%d removed=%d, want 5/5/14/2",
						len(xPlayer.Hand), len(yPlayer.Hand), len(web.session.state.Deck),
						len(web.session.state.Discard))
				}
				draftCompletionChecked = true
			}
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
			if view.BattleAttack == nil || view.BattleDefend == nil || view.BattleOutcome == nil {
				t.Fatal("battle result omitted its revealed cards or outcome")
			}
			outcome := game.ResolveWithModifiers(view.BattleAttack.Card, view.BattleDefend.Card, view.Modifiers)
			if view.BattleOutcome.AttackerValue != outcome.AttackerValue ||
				view.BattleOutcome.DefenderValue != outcome.DefenderValue ||
				view.BattleOutcome.Winner != outcome.Winner {
				t.Fatalf("battle outcome = %+v, want %+v", view.BattleOutcome, outcome)
			}
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "continue"})
		default:
			t.Fatalf("unexpected UI phase %q", view.Phase)
		}
	}
	if view.Phase != "gameover" {
		t.Fatalf("game did not finish; stopped in phase %q", view.Phase)
	}
	if view.Payout == nil || view.Payout.TotalMarks <= 0 {
		t.Fatalf("game over response omitted Mark payout: %+v", view.Payout)
	}
	if attacks != 6 || defenses != 6 {
		t.Fatalf("manual attack/defense actions = %d/%d; third battles should be automatic", attacks, defenses)
	}
	if draftPicks != 6 || playerProfile.DraftStats.TotalDraftsCompleted != 1 {
		t.Fatalf("draft picks=%d profile stats=%d, want 6/1", draftPicks, playerProfile.DraftStats.TotalDraftsCompleted)
	}
	if got := len(web.session.state.P1.Jail) + len(web.session.state.P2.Jail) + web.session.state.Clashes; got != 9 {
		t.Fatalf("resolved %d of 9 battles", got)
	}
	if err := web.session.state.Validate(); err != nil {
		t.Fatalf("finished game state is invalid: %v", err)
	}

	view = sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{"mode": "hard"})
	if view.Phase != "draft" || view.DraftMode < 1 || view.DraftMode > 3 ||
		len(view.DraftPool) != 2+2*view.DraftMode || view.Mode != "hard" ||
		len(view.Players[1].Hand) != 0 || view.Players[1].HiddenHand == 0 {
		t.Fatal("Recruit AI game did not start a valid draft or exposed the AI hand")
	}
}

func TestWebSkinAssetsAreServedWithImageHeaders(t *testing.T) {
	web := &webServer{}
	request := httptest.NewRequest(http.MethodGet, "/skins/shop/tribal/red/small_idle.png", nil)
	response := httptest.NewRecorder()
	web.handleSkinAsset(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q, want image/png", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Fatalf("cache control = %q", got)
	}
	if response.Body.Len() == 0 {
		t.Fatal("skin image response is empty")
	}
	original, err := webFiles.ReadFile("shop/tribal/red/small_idle.png")
	if err != nil {
		t.Fatalf("read source skin image: %v", err)
	}
	if !bytes.Equal(response.Body.Bytes(), original) {
		t.Fatal("skin image response changed the source image")
	}
	for _, assetPath := range []string{
		"/skins/shop/conq/lime/small_idle.png",
		"/skins/shop/conq/gold/large_win.png",
		"/skins/hero_icons/gorilla.png",
		"/skins/hero_icons/coach.png",
		"/skins/hero_icons/cpu_easy.png",
		"/skins/shop/alien/yellow/large_idle.png",
		"/skins/shop/amazon/green/large_idle.png",
		"/skins/shop/bee/silver/large_idle.png",
		"/skins/shop/cat/brown/large_idle.png",
		"/skins/shop/dog/gray/large_idle.png",
		"/skins/shop/dragon/ruby/large_idle.png",
		"/skins/shop/dwarf/purple/large_idle.png",
		"/skins/shop/orc/white/large_idle.png",
		"/skins/shop/reptile/pink/large_idle.png",
		"/skins/shop/zulu/orange/large_idle.png",
		"/skins/icons/marks.png",
		"/skins/icons/crowns.png",
		"/skins/icons/energy.png",
		"/skins/icons/wooden_chest.png",
		"/skins/icons/silver_chest.png",
		"/skins/icons/golden_chest.png",
		"/skins/icons/prismatic_chest.png",
		"/skins/icons/hero_chest.png",
	} {
		request = httptest.NewRequest(http.MethodGet, assetPath, nil)
		response = httptest.NewRecorder()
		web.handleSkinAsset(response, request)
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Errorf("asset %q status=%d body-size=%d", assetPath, response.Code, response.Body.Len())
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/skins/%2e%2e/web/index.html", nil)
	response = httptest.NewRecorder()
	web.handleSkinAsset(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestWebSkillBranchIconsAreServed(t *testing.T) {
	web := &webServer{}
	for _, name := range []string{"BattleRewards.png", "Campaign.png", "CampDraw.png", "Energy.png", "HeroAq.png", "HeroTactics.png"} {
		request := httptest.NewRequest(http.MethodGet, "/skills/icons/"+name, nil)
		response := httptest.NewRecorder()
		web.handleSkillIcon(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || response.Body.Len() == 0 {
			t.Errorf("icon %q status=%d content-type=%q body-size=%d", name, response.Code,
				response.Header().Get("Content-Type"), response.Body.Len())
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/skills/icons/../skills.go", nil)
	response := httptest.NewRecorder()
	web.handleSkillIcon(response, request)
	if response.Code != http.StatusNotFound {
		t.Errorf("traversal status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAwardWebMatchMarksAppliesWinAndQuartermasterMultipliers(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 250
	p.SkillNodes["quartermaster"] = true

	win := awardWebMatchMarks(p, 20, true)
	if win.BaseMarks != 70 || win.WinMultiplier != 3 || win.SkillBonusPct != 10 ||
		win.TotalMarks != 231 || p.Marks != 481 {
		t.Fatalf("win payout/profile = %+v / %d", win, p.Marks)
	}

	loss := awardWebMatchMarks(p, 20, false)
	if loss.BaseMarks != 70 || loss.WinMultiplier != 1 || loss.SkillBonusPct != 10 ||
		loss.TotalMarks != 77 || p.Marks != 558 {
		t.Fatalf("loss payout/profile = %+v / %d", loss, p.Marks)
	}
}

func TestAwardWebMatchMarksWithoutQuartermaster(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 50
	payout := awardWebMatchMarks(p, 17, true)
	if payout.BaseMarks != 67 || payout.WinMultiplier != 3 ||
		payout.SkillBonusPct != 0 || payout.TotalMarks != 201 || p.Marks != 251 {
		t.Fatalf("payout/profile = %+v / %d", payout, p.Marks)
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

func TestSpecialVariantsRequireCampaignMission(t *testing.T) {
	for _, variant := range []string{"puzzler", "abandon"} {
		t.Run(variant, func(t *testing.T) {
			web := &webServer{rng: rand.New(rand.NewSource(57)), defaultMode: "easy"}
			request := httptest.NewRequest(http.MethodPost, "/api/new", bytes.NewBufferString(
				`{"mode":"easy","variant":"`+variant+`","playerHeroId":"recruit"}`,
			))
			response := httptest.NewRecorder()
			web.handleNewGame(response, request)
			if response.Code != http.StatusBadRequest ||
				!bytes.Contains(response.Body.Bytes(), []byte("only available as campaign missions")) {
				t.Fatalf("special variant status/body = %d / %s", response.Code, response.Body.String())
			}
			if web.session != nil {
				t.Fatalf("standalone %s request started a session", variant)
			}
		})
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
		dashboard.DefaultHeroID != p.DefaultHeroID || len(dashboard.OwnedChromas) != 3 ||
		len(dashboard.OwnedChromaDetails) != 3 ||
		dashboard.Marks != p.Marks || dashboard.Crowns != p.Crowns ||
		dashboard.Energy != 7 || dashboard.MaxEnergy != p.MaxEnergy ||
		dashboard.AdsLeft != energy.DailyAdLimit-2 ||
		dashboard.LoginStreakCurrent != 12 || dashboard.LoginStreakBest != 30 ||
		dashboard.WinsSinceCampDraw != 4 || dashboard.WinsPerCampDraw != 10 ||
		dashboard.DrawsAvailable != 3 || dashboard.PendingBoxes != 2 || dashboard.SkillPoints != 1 ||
		dashboard.UnseenAchievements != 1 {
		t.Fatalf("dashboard data = %+v", dashboard)
	}
	for _, id := range []string{"crimson", "cobalt", "sunflower"} {
		found := false
		for _, ownedID := range dashboard.OwnedChromas {
			if ownedID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("dashboard did not expose owned chroma %q", id)
		}
		detailFound := false
		for _, chroma := range dashboard.OwnedChromaDetails {
			if chroma.ID == id {
				detailFound = true
				break
			}
		}
		if !detailFound {
			t.Errorf("dashboard did not expose details for owned chroma %q", id)
		}
	}
}

func TestWebDashboardFeatureActionsUseTemporaryPreviewProfile(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 1000
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
		map[string]string{"action": "buy-chroma", "id": "conq-lime"})
	if shopResponse.Code != http.StatusOK {
		t.Fatalf("chroma purchase status = %d: %s", shopResponse.Code, shopResponse.Body.String())
	}
	if p.Marks != 1000 || web.previewProfile.Marks != previewMarksBeforeShop-600 ||
		!web.previewProfile.OwnedChromas["conq-lime"] {
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

func TestShopChromaPurchasePersistsCurrencyAndOwnership(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 600
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	p.SetSaveHandler(func(saved *profile.Profile) error { return saved.SaveTo(profilePath) })
	web := &webServer{playerProfile: p, rng: rand.New(rand.NewSource(72))}

	response := callDashboardHandler(t, web.handleShop, http.MethodPost, "/api/shop",
		map[string]string{"action": "buy-chroma", "id": "conq-lime"})
	if response.Code != http.StatusOK {
		t.Fatalf("chroma purchase status = %d: %s", response.Code, response.Body.String())
	}
	var page webShopPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if p.Marks != 0 || page.Dashboard.Marks != 0 || !p.OwnedChromas["conq-lime"] {
		t.Fatalf("purchase did not update currency and ownership: marks=%d owned=%t dashboard=%+v",
			p.Marks, p.OwnedChromas["conq-lime"], page.Dashboard)
	}
	shopHasOwnedChroma := false
	for _, item := range page.Chromas {
		if item.Chroma.ID == "conq-lime" {
			shopHasOwnedChroma = item.Owned
			break
		}
	}
	if !shopHasOwnedChroma {
		t.Fatal("shop response did not mark the purchased chroma as owned")
	}
	dashboardHasOwnedChroma := false
	for _, chroma := range page.Dashboard.OwnedChromaDetails {
		if chroma.ID == "conq-lime" {
			dashboardHasOwnedChroma = true
			break
		}
	}
	if !dashboardHasOwnedChroma {
		t.Fatal("shop response did not include the purchased chroma in gameplay appearance choices")
	}
	if page.Dashboard.EquippedChromas["B"].ID == "conq-lime" {
		t.Fatal("purchasing a chroma should not automatically assign it to a suit")
	}
	saved, err := profile.LoadProfileFrom(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Marks != 0 || !saved.OwnedChromas["conq-lime"] {
		t.Fatalf("saved purchase state = marks:%d owned:%t", saved.Marks, saved.OwnedChromas["conq-lime"])
	}
}

func TestShopChromaFamilyPurchasePersistsDiscountAndOwnership(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 4000
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	p.SetSaveHandler(func(saved *profile.Profile) error { return saved.SaveTo(profilePath) })
	web := &webServer{playerProfile: p, rng: rand.New(rand.NewSource(73))}

	response := callDashboardHandler(t, web.handleShop, http.MethodPost, "/api/shop",
		map[string]string{"action": "buy-chroma-family", "id": "alien"})
	if response.Code != http.StatusOK {
		t.Fatalf("family purchase status = %d: %s", response.Code, response.Body.String())
	}
	var page webShopPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if p.Marks != page.Dashboard.Marks || p.TotalChromasPurchased != 3 {
		t.Fatalf("family purchase did not apply discount and ownership: marks=%d purchased=%d dashboard=%+v",
			p.Marks, p.TotalChromasPurchased, page.Dashboard)
	}
	for _, id := range []string{"alien-blue", "alien-red", "alien-yellow"} {
		if !p.OwnedChromas[id] {
			t.Errorf("family purchase did not grant %q", id)
		}
	}
	saved, err := profile.LoadProfileFrom(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Marks != p.Marks || saved.TotalChromasPurchased != 3 || !saved.OwnedChromas["alien-yellow"] {
		t.Fatalf("saved family purchase = marks:%d purchased:%d alien-yellow:%t",
			saved.Marks, saved.TotalChromasPurchased, saved.OwnedChromas["alien-yellow"])
	}
}

func TestShopBoxPurchasesChargeCurrencyAndGrantInventory(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks = 5000
	p.Crowns = 2000
	web := &webServer{}

	for _, boxType := range []string{"wooden", "silver", "hero", "gold"} {
		if _, err := web.applyShopAction(p, actionRequest{Action: "buy-box", BoxType: boxType}); err != nil {
			t.Fatalf("buy %s box: %v", boxType, err)
		}
	}
	if p.Marks != 2000 || p.Crowns != 1000 ||
		p.PendingBoxes["wooden"] != 1 || p.PendingBoxes["silver"] != 1 ||
		p.PendingBoxes["gold"] != 1 || p.HeroBoxesAvailable != 1 {
		t.Fatalf("box purchase inventory = marks:%d crowns:%d boxes:%v hero boxes:%d",
			p.Marks, p.Crowns, p.PendingBoxes, p.HeroBoxesAvailable)
	}
	offers := web.shopPage(p).BoxOffers
	if len(offers) != 4 {
		t.Fatalf("shop returned %d box offers, want 4", len(offers))
	}
	wantOffers := []webShopBoxOffer{
		{Type: "wooden", Currency: "marks", Cost: 1000, Available: 1},
		{Type: "silver", Currency: "marks", Cost: 1000, Available: 1},
		{Type: "hero", Currency: "marks", Cost: 1000, Available: 1},
		{Type: "gold", Currency: "crowns", Cost: 1000, Available: 1},
	}
	for index, offer := range offers {
		want := wantOffers[index]
		if offer.Type != want.Type || offer.Currency != want.Currency ||
			offer.Cost != want.Cost || offer.Available != want.Available {
			t.Errorf("box offer %d = %+v, want %+v", index, offer, want)
		}
	}

	p.Marks = 999
	if _, err := web.applyShopAction(p, actionRequest{Action: "buy-box", BoxType: "wooden"}); err == nil {
		t.Fatal("wooden box purchase succeeded without enough Marks")
	}
	if p.Marks != 999 || p.PendingBoxes["wooden"] != 1 {
		t.Fatalf("failed purchase changed profile: marks:%d wooden:%d", p.Marks, p.PendingBoxes["wooden"])
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
	if campaignResponse.Code != http.StatusOK || len(campaignPage.Campaigns) != 14 {
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

func TestAIDraftPickWaitsForSeparateAction(t *testing.T) {
	state, err := game.NewDraftGameWithModifiers(
		rand.New(rand.NewSource(57)), game.GameModifiers{Reckoning: [3]int{1, 1, 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	state.P1.Seat, state.P2.Seat = game.SeatX, game.SeatY
	web := &webServer{
		rng: rand.New(rand.NewSource(58)),
		session: &webSession{
			state: state, mode: "hard", phase: "draft", actor: 1,
		},
	}
	web.session.message = web.draftTurnMessage(web.session.actor)
	poolSize := len(web.session.state.Draft.Pool)

	if err := web.advanceAI(); err != nil {
		t.Fatal(err)
	}
	if got := len(web.session.state.Draft.Pool); got != poolSize {
		t.Fatalf("AI draft pool changed before its delayed action: got %d, want %d", got, poolSize)
	}
	if got, want := web.session.message, "Hard CPU is choosing a card from the public pool."; got != want {
		t.Fatalf("AI draft prompt = %q, want %q", got, want)
	}
	if err := web.applyAction(actionRequest{Action: "draft-ai-pick"}); err != nil {
		t.Fatal(err)
	}
	if got := len(web.session.state.Draft.Pool); got != poolSize-1 {
		t.Fatalf("AI draft pool after pick = %d, want %d", got, poolSize-1)
	}
	if web.session.actor != 0 {
		t.Fatalf("next draft actor = %d, want human player 0", web.session.actor)
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
		case "draft":
			action := map[string]any{"action": "draft-pick", "index": 0}
			if view.Actor != 0 {
				action = map[string]any{"action": "draft-ai-pick"}
			}
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", action)
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

func TestRoundThreeSeatAssignmentIsDeterministic(t *testing.T) {
	tests := []struct {
		name      string
		p1Jail    []game.Card
		p2Jail    []game.Card
		wantSeatY int
	}{
		{name: "P1 lower scorer takes second position", p1Jail: []game.Card{{Suit: game.Blade, Face: 1}}, p2Jail: []game.Card{{Suit: game.Blade, Face: 9}}, wantSeatY: 0},
		{name: "P2 lower scorer takes second position", p1Jail: []game.Card{{Suit: game.Blade, Face: 9}}, p2Jail: []game.Card{{Suit: game.Blade, Face: 1}}, wantSeatY: 1},
		{name: "original second player takes second position on tie", wantSeatY: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, err := game.NewGame(rand.New(rand.NewSource(1)))
			if err != nil {
				t.Fatal(err)
			}
			state.Bounty.Suit = game.Shield
			state.P1.Seat, state.P2.Seat = game.SeatY, game.SeatX
			state.P1.Jail, state.P2.Jail = test.p1Jail, test.p2Jail

			if got := assignRoundThreeSeats(&state); got != test.wantSeatY {
				t.Fatalf("player assigned to second position = %d, want %d", got, test.wantSeatY)
			}
			wantP1 := game.SeatX
			wantP2 := game.SeatX
			if test.wantSeatY == 0 {
				wantP1 = game.SeatY
			} else {
				wantP2 = game.SeatY
			}
			if state.P1.Seat != wantP1 || state.P2.Seat != wantP2 {
				t.Fatalf("seat assignment = %v/%v, want %v/%v", state.P1.Seat, state.P2.Seat, wantP1, wantP2)
			}
		})
	}
}
