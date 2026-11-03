package app

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func TestLadderWebGameAndSnapshot(t *testing.T) {
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.UnlockedHeroes = []string{"recruit", "warden", "hunter"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{
		rng: rand.New(rand.NewSource(81)), playerProfile: playerProfile,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	view := sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{
		"mode": "easy", "variant": "ladder", "playerHeroId": "warden",
	})
	if !view.Started || view.Variant != "ladder" || view.Ladder == nil ||
		view.Phase != "ladder-play" || view.Actor != 0 ||
		len(view.Ladder.Hand) == 0 || view.Ladder.OpponentHandCount == 0 {
		t.Fatalf("new Ladder game view = %+v", view)
	}
	if view.Ladder.FinalHands != nil {
		t.Fatal("in-progress Ladder view exposed the opponent's hand")
	}

	encoded, err := json.Marshal(snapshotSession(web.session))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot sessionSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	restored := snapshot.restore()
	if restored.ladder == nil ||
		!reflect.DeepEqual(restored.ladder.State(), web.session.ladder.State()) {
		t.Fatalf("Ladder state did not survive snapshot: restored=%+v current=%+v",
			restored.ladder, web.session.ladder)
	}

	for turn := 0; turn < 35 && view.Phase != "gameover"; turn++ {
		if view.Phase != "ladder-play" || view.Actor != 0 || view.Ladder == nil {
			t.Fatalf("unexpected CPU match state at turn %d: phase=%q actor=%d", turn, view.Phase, view.Actor)
		}
		action := map[string]any{"action": "ladder-pass"}
		for _, card := range view.Ladder.Hand {
			if card.Selectable {
				action = map[string]any{"action": "ladder-play", "index": card.Index}
				break
			}
		}
		if action["action"] == "ladder-pass" && !view.Ladder.CanPass {
			t.Fatal("hand has no legal play but view does not permit passing")
		}
		view = sendWebRequest(t, server.Client(), server.URL+"/api/action", action)
	}
	if view.Phase != "gameover" || view.Ladder == nil || len(view.Ladder.FinalHands) != 2 {
		t.Fatalf("CPU Ladder match did not finish with visible results: %+v", view)
	}
}

func TestLadderTwoPlayerEngineHandoffHidesHands(t *testing.T) {
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.UnlockedHeroes = []string{"recruit"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{rng: rand.New(rand.NewSource(82)), playerProfile: playerProfile}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	playerHero, _ := heroes.ByID("recruit")
	opponentHero := playerHero
	if err := web.startWebLadder("hvh", playerHero.ID, opponentHero.ID, playerHero, opponentHero); err != nil {
		t.Fatal(err)
	}
	view := web.view()
	if view.Phase != "handoff" || view.Ladder == nil || len(view.Ladder.Hand) != 0 {
		t.Fatalf("new two-player Ladder game exposed a hand: %+v", view)
	}
	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "ready"})
	if view.Phase != "ladder-play" || len(view.Ladder.Hand) == 0 {
		t.Fatalf("ready action did not reveal the active hand: %+v", view)
	}
}
