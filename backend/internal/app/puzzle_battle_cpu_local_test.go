package app

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/puzzlebattle"
)

func TestLocalPuzzleBattleCPURewardsHalfPayout(t *testing.T) {
	t.Setenv("BASE_PATH", "")
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.Marks = 500
	playerProfile.DefaultHeroID = "recruit"
	playerProfile.UnlockedHeroes = []string{"recruit"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{playerProfile: playerProfile, rng: rand.New(rand.NewSource(42))}
	server := httptest.NewServer(web.routes())
	defer server.Close()
	client := testAccountClient{t: t, client: server.Client(), base: server.URL}

	var started puzzleBattleResponse
	if err := json.Unmarshal(client.request("POST", "/puzzle-battle/cpu", map[string]string{
		"action": "start", "heroId": "recruit",
	}, http.StatusCreated), &started); err != nil {
		t.Fatal(err)
	}
	if !started.CPUPractice || started.Match == nil ||
		started.Match.View.Status != puzzlebattle.PhaseRoundOne {
		t.Fatalf("local Easy CPU match did not start: %+v", started)
	}
	if playerProfile.Energy != playerProfile.MaxEnergy-1 {
		t.Fatalf("local CPU Puzzle Battle Energy = %d/%d, want one spent",
			playerProfile.Energy, playerProfile.MaxEnergy)
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
		t.Fatalf("local CPU did not automatically resolve round one: %+v", roundTwo.Match)
	}
	marksBeforePayout := playerProfile.Marks
	completedBytes := submitPairs([][2]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}})
	var completed puzzleBattleResponse
	if err := json.Unmarshal(completedBytes, &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Match == nil || completed.Match.View.Status != puzzlebattle.PhaseComplete {
		t.Fatalf("local Easy CPU match did not complete: %+v", completed.Match)
	}
	if completed.Payout == nil || completed.Payout.Multiplier != 0.5 || completed.Payout.Marks <= 0 {
		t.Fatalf("local Easy CPU payout = %+v, want a positive payout at 50%%", completed.Payout)
	}
	if playerProfile.Marks < marksBeforePayout+completed.Payout.Marks {
		t.Fatalf("local profile marks = %d, want at least %d", playerProfile.Marks, marksBeforePayout+completed.Payout.Marks)
	}
	var reloaded puzzleBattleResponse
	if err := json.Unmarshal(client.request("GET", "/puzzle-battle/cpu", nil, http.StatusOK), &reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Payout == nil || *reloaded.Payout != *completed.Payout {
		t.Fatalf("reloaded local CPU payout = %+v, want %+v", reloaded.Payout, completed.Payout)
	}
}
