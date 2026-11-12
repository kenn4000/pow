package app

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"prisonersofwar/internal/puzzlebattle"
)

func newPuzzleBattleTestServer(t *testing.T) (*webServer, func() testAccountClient) {
	t.Helper()
	db := integrationDatabase(t)
	t.Setenv("BASE_PATH", "")
	t.Setenv("COOKIE_SECURE", "false")
	serverApp := &webServer{db: db, defaultMode: "easy"}
	server := httptest.NewServer(serverApp.routes())
	t.Cleanup(server.Close)
	newClient := func() testAccountClient {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		return testAccountClient{t: t, client: &http.Client{Jar: jar}, base: server.URL}
	}
	return serverApp, newClient
}

func puzzleBattleSignup(t *testing.T, client testAccountClient, email string) account {
	t.Helper()
	data := client.request("POST", "/auth/signup", credentials{Email: email, Password: "a-long-test-password"}, http.StatusOK)
	var response struct {
		User account `json:"user"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	return response.User
}

func TestPuzzleBattleMatchmakingAndParticipantPrivacy(t *testing.T) {
	appServer, newClient := newPuzzleBattleTestServer(t)
	first, second, spectator := newClient(), newClient(), newClient()
	firstUser := puzzleBattleSignup(t, first, "first@example.com")
	secondUser := puzzleBattleSignup(t, second, "second@example.com")
	puzzleBattleSignup(t, spectator, "spectator@example.com")

	var queued puzzleBattleResponse
	if err := json.Unmarshal(second.request("POST", "/puzzle-battle", map[string]string{
		"action": "queue", "heroId": "recruit",
	}, http.StatusOK), &queued); err != nil {
		t.Fatal(err)
	}
	if queued.Status != "queued" {
		t.Fatalf("second queue status = %q, want queued", queued.Status)
	}

	var created puzzleBattleResponse
	if err := json.Unmarshal(first.request("POST", "/puzzle-battle", map[string]string{
		"action": "queue", "heroId": "recruit",
	}, http.StatusCreated), &created); err != nil {
		t.Fatal(err)
	}
	if created.Match == nil || created.Status != "waiting" {
		t.Fatalf("matchmaking response = %+v", created)
	}
	matchID := created.Match.View.MatchID
	if created.Match.View.Player != 1 {
		t.Fatalf("newly queued player seat = %d, want 1", created.Match.View.Player)
	}
	for _, playerID := range []string{firstUser.ID, secondUser.ID} {
		var remaining int
		if err := appServer.db.QueryRow(
			`SELECT (profile->>'energy')::integer FROM player_data WHERE user_id=$1`, playerID,
		).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining != 9 {
			t.Errorf("online Puzzle Battle player %s has %d Energy, want 9", playerID, remaining)
		}
	}

	var opponent puzzleBattleResponse
	if err := json.Unmarshal(second.request("GET", "/puzzle-battle", nil, http.StatusOK), &opponent); err != nil {
		t.Fatal(err)
	}
	if opponent.Match == nil || opponent.Match.View.Player != 0 ||
		opponent.Match.View.PlayerIDs != [2]string{secondUser.ID, firstUser.ID} {
		t.Fatalf("opponent match view = %+v", opponent.Match)
	}

	spectator.request("POST", "/puzzle-battle/"+matchID+"/action", map[string]any{
		"action": "ready",
	}, http.StatusNotFound)
}

func TestRankedPuzzleBattleChargesTwoEnergyOnMatchCreation(t *testing.T) {
	appServer, newClient := newPuzzleBattleTestServer(t)
	first, second := newClient(), newClient()
	firstUser := puzzleBattleSignup(t, first, "ranked-energy-first@example.com")
	secondUser := puzzleBattleSignup(t, second, "ranked-energy-second@example.com")
	first.request("POST", "/puzzle-battle", map[string]string{
		"action": "queue", "heroId": "recruit", "matchMode": "ranked",
	}, http.StatusOK)
	second.request("POST", "/puzzle-battle", map[string]string{
		"action": "queue", "heroId": "recruit", "matchMode": "ranked",
	}, http.StatusCreated)
	for _, playerID := range []string{firstUser.ID, secondUser.ID} {
		var remaining int
		if err := appServer.db.QueryRow(
			`SELECT (profile->>'energy')::integer FROM player_data WHERE user_id=$1`, playerID,
		).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining != 8 {
			t.Errorf("ranked Puzzle Battle player %s has %d Energy, want 8", playerID, remaining)
		}
	}
}

func TestPuzzleBattlePairingsRemainPrivateUntilCompletion(t *testing.T) {
	appServer, newClient := newPuzzleBattleTestServer(t)
	first, second := newClient(), newClient()
	puzzleBattleSignup(t, first, "private-first@example.com")
	puzzleBattleSignup(t, second, "private-second@example.com")

	second.request("POST", "/puzzle-battle", map[string]string{
		"action": "queue", "heroId": "recruit",
	}, http.StatusOK)
	var created puzzleBattleResponse
	if err := json.Unmarshal(first.request("POST", "/puzzle-battle", map[string]string{
		"action": "queue", "heroId": "recruit",
	}, http.StatusCreated), &created); err != nil {
		t.Fatal(err)
	}
	matchID := created.Match.View.MatchID
	for _, client := range []testAccountClient{first, second} {
		client.request("POST", "/puzzle-battle/"+matchID+"/action", map[string]string{
			"action": "ready",
		}, http.StatusOK)
	}

	var matchStateBytes []byte
	if err := appServer.db.QueryRow("SELECT state FROM puzzle_battle_matches WHERE id=$1", matchID).Scan(&matchStateBytes); err != nil {
		t.Fatal(err)
	}
	var matchState puzzlebattle.Match
	if err := json.Unmarshal(matchStateBytes, &matchState); err != nil {
		t.Fatal(err)
	}
	if matchState.CountdownEndsAt == nil {
		t.Fatal("both ready players did not start a countdown")
	}
	past := time.Now().UTC().Add(-time.Second)
	matchState.CountdownEndsAt = &past
	matchState.UpdatedAt = time.Now().UTC().Add(-2 * time.Second)
	updatedState, err := json.Marshal(matchState)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appServer.db.Exec("UPDATE puzzle_battle_matches SET state=$1 WHERE id=$2", updatedState, matchID); err != nil {
		t.Fatal(err)
	}

	var active puzzleBattleResponse
	if err := json.Unmarshal(first.request("GET", "/puzzle-battle", nil, http.StatusOK), &active); err != nil {
		t.Fatal(err)
	}
	if active.Match == nil || active.Match.View.Status != "round1" {
		t.Fatalf("match status after countdown = %+v", active.Match)
	}
	if active.Match.View.RequiredPairs != 3 {
		t.Fatalf("round one requires %d pairs, want 3", active.Match.View.RequiredPairs)
	}
	actor := first
	if active.Match.View.Player != active.Match.View.Controller {
		actor = second
	}
	actor.request("POST", "/puzzle-battle/"+matchID+"/action", map[string]any{
		"action": "submit-pair", "firstIndex": 0, "secondIndex": 1,
	}, http.StatusOK)

	actorViewClient, otherViewClient := actor, first
	if actor == first {
		otherViewClient = second
	}
	var privateView puzzleBattleResponse
	if err := json.Unmarshal(actorViewClient.request("GET", "/puzzle-battle", nil, http.StatusOK), &privateView); err != nil {
		t.Fatal(err)
	}
	if len(privateView.Match.View.OwnPairs) != 1 {
		t.Fatalf("acting player's pair selection missing: %+v", privateView.Match.View.OwnPairs)
	}
	var otherView puzzleBattleResponse
	if err := json.Unmarshal(otherViewClient.request("GET", "/puzzle-battle", nil, http.StatusOK), &otherView); err != nil {
		t.Fatal(err)
	}
	if len(otherView.Match.View.OwnPairs) != 0 || otherView.Match.View.OpponentSubmitted {
		t.Fatalf("opponent selection leaked through private view: %+v", otherView.Match.View)
	}

	submitPairs := func(client testAccountClient, pairs [][2]int) []byte {
		t.Helper()
		var data []byte
		for _, pair := range pairs {
			data = client.request("POST", "/puzzle-battle/"+matchID+"/action", map[string]any{
				"action": "submit-pair", "firstIndex": pair[0], "secondIndex": pair[1],
			}, http.StatusOK)
		}
		return data
	}
	submitPairs(actor, [][2]int{{2, 3}, {4, 5}})
	submitPairs(otherViewClient, [][2]int{{0, 1}, {2, 3}, {4, 5}})
	submitPairs(actor, [][2]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}})
	completedBytes := submitPairs(otherViewClient, [][2]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}})
	var completed puzzleBattleResponse
	if err := json.Unmarshal(completedBytes, &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Match == nil || completed.Match.View.Status != puzzlebattle.PhaseComplete {
		t.Fatalf("match did not complete after the final pair: %+v", completed.Match)
	}
	if completed.Payout == nil || completed.Payout.Marks <= 0 {
		t.Fatalf("completed match response omitted the player's reward payout: %+v", completed.Payout)
	}
	var reloaded puzzleBattleResponse
	if err := json.Unmarshal(otherViewClient.request("GET", "/puzzle-battle", nil, http.StatusOK), &reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Payout == nil || reloaded.Payout.Marks != completed.Payout.Marks ||
		reloaded.Payout.Crowns != completed.Payout.Crowns {
		t.Fatalf("reloaded match payout = %+v, want %+v", reloaded.Payout, completed.Payout)
	}
}

func TestPuzzleBattleEasyCPURewardsHalfPayout(t *testing.T) {
	appServer, newClient := newPuzzleBattleTestServer(t)
	client := newClient()
	user := puzzleBattleSignup(t, client, "easy-cpu@example.com")

	var started puzzleBattleResponse
	if err := json.Unmarshal(client.request("POST", "/puzzle-battle/cpu", map[string]string{
		"action": "start", "heroId": "recruit",
	}, http.StatusCreated), &started); err != nil {
		t.Fatal(err)
	}
	if !started.CPUPractice || started.Match == nil ||
		started.Match.View.Status != puzzlebattle.PhaseRoundOne {
		t.Fatalf("Easy CPU match did not start: %+v", started)
	}
	var remaining int
	if err := appServer.db.QueryRow(
		`SELECT (profile->>'energy')::integer FROM player_data WHERE user_id=$1`, user.ID,
	).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 9 {
		t.Fatalf("Easy CPU Puzzle Battle has %d Energy, want 9", remaining)
	}
	matchID := started.Match.View.MatchID
	submitPairs := func(pairs [][2]int) []byte {
		t.Helper()
		var data []byte
		for _, pair := range pairs {
			data = client.request("POST", "/puzzle-battle/cpu/"+matchID+"/action", map[string]any{
				"action": "submit-pair", "firstIndex": pair[0], "secondIndex": pair[1],
			}, http.StatusOK)
		}
		return data
	}
	roundTwoBytes := submitPairs([][2]int{{0, 1}, {2, 3}, {4, 5}})
	var roundTwo puzzleBattleResponse
	if err := json.Unmarshal(roundTwoBytes, &roundTwo); err != nil {
		t.Fatal(err)
	}
	if roundTwo.Match == nil || roundTwo.Match.View.Status != puzzlebattle.PhaseRoundTwo {
		t.Fatalf("CPU did not automatically resolve round one: %+v", roundTwo.Match)
	}

	completedBytes := submitPairs([][2]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}})
	var completed puzzleBattleResponse
	if err := json.Unmarshal(completedBytes, &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Match == nil || completed.Match.View.Status != puzzlebattle.PhaseComplete {
		t.Fatalf("Easy CPU match did not complete: %+v", completed.Match)
	}
	if completed.Payout == nil || completed.Payout.Multiplier != 0.5 || completed.Payout.Marks <= 0 {
		t.Fatalf("Easy CPU payout = %+v, want a positive payout at 50%%", completed.Payout)
	}
	var reloaded puzzleBattleResponse
	if err := json.Unmarshal(client.request("GET", "/puzzle-battle/cpu", nil, http.StatusOK), &reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Payout == nil || *reloaded.Payout != *completed.Payout {
		t.Fatalf("reloaded Easy CPU payout = %+v, want %+v", reloaded.Payout, completed.Payout)
	}
}
