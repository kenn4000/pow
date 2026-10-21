package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
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
