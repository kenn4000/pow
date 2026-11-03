package app

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func TestRummyTwoPlayerEngineAlternatesAndScoresAfterTwelveTurns(t *testing.T) {
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.UnlockedHeroes = []string{"recruit", "warden", "hunter"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{rng: rand.New(rand.NewSource(619)), playerProfile: playerProfile}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", web.handleState)
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	first, _ := heroes.ByID("warden")
	second, _ := heroes.ByID("hunter")
	web.startWebRummy("hvh", first.ID, second.ID, first, second)
	view := web.view()
	if !view.Started || view.Variant != "rummy" || view.Phase != "rummy-opening" {
		t.Fatalf("new Rummy game = started:%t variant:%q phase:%q", view.Started, view.Variant, view.Phase)
	}
	if view.Rummy == nil || view.Rummy.Parameters.J != 7 || view.Rummy.Parameters.K != 0 ||
		view.Rummy.Parameters.B != 3 || view.Rummy.Parameters.HandSize != 6 ||
		len(view.Rummy.Hand) != 6 || view.Rummy.DrawPileCount != 15 {
		t.Fatalf("new Rummy state = %+v", view.Rummy)
	}
	assertRummyCardAccounting(t, web.session.rummy)

	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
		"action": "rummy-draw-two",
	})
	assertRummyCardAccounting(t, web.session.rummy)
	if view.Phase != "rummy-discard" || len(view.Rummy.Hand) != 8 {
		t.Fatalf("opening draw phase=%q hand=%d", view.Phase, len(view.Rummy.Hand))
	}
	if err := web.applyAction(actionRequest{
		Action: "rummy-discard", FaceUpIndex: 2, BuriedIndex: 2,
	}); err == nil {
		t.Fatal("discard accepted the same card for both piles")
	}
	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
		"action": "rummy-discard", "faceUpIndex": 0, "buriedIndex": 1,
	})
	assertRummyCardAccounting(t, web.session.rummy)
	if view.Phase != "handoff" || view.Actor != 1 || len(view.Rummy.Hand) != 0 ||
		view.Rummy.TurnsCompleted != 1 || len(view.Rummy.FaceUpPile) != 1 {
		t.Fatalf("after opening turn = phase:%q actor:%d hand:%d Rummy:%+v",
			view.Phase, view.Actor, len(view.Rummy.Hand), view.Rummy)
	}
	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "ready"})
	if view.Phase != "rummy-turn" || len(view.Rummy.Hand) != 6 {
		t.Fatalf("Player 2 handoff = phase:%q hand:%d", view.Phase, len(view.Rummy.Hand))
	}
	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
		"action": "rummy-draw-face-up",
	})
	assertRummyCardAccounting(t, web.session.rummy)
	if view.Phase != "handoff" || len(view.Rummy.Hand) != 0 || view.Rummy.TurnsCompleted != 2 {
		t.Fatalf("face-up draw did not complete a turn: phase:%q Rummy:%+v", view.Phase, view.Rummy)
	}

	for steps := 0; steps < 40 && view.Phase != "gameover"; steps++ {
		switch view.Phase {
		case "handoff":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": "ready"})
		case "rummy-turn":
			action := "rummy-draw-two"
			if view.Rummy.CanDrawFaceUp {
				action = "rummy-draw-face-up"
			}
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{"action": action})
		case "rummy-discard":
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "rummy-discard", "faceUpIndex": 0, "buriedIndex": 1,
			})
		default:
			t.Fatalf("unexpected Rummy phase %q", view.Phase)
		}
		assertRummyCardAccounting(t, web.session.rummy)
	}
	if view.Phase != "gameover" || view.Rummy == nil || view.Rummy.TurnsCompleted != 12 ||
		len(view.Rummy.Results) != 2 || view.Winner < -1 || view.Winner > 1 {
		t.Fatalf("Rummy match did not finish with two results: %+v", view)
	}
}

func assertRummyCardAccounting(t *testing.T, rummy *rummySession) {
	t.Helper()
	seen := make(map[game.Card]bool, 27)
	add := func(card game.Card) {
		t.Helper()
		if !card.Valid() || seen[card] {
			t.Fatalf("invalid or duplicate Rummy card %s", card)
		}
		seen[card] = true
	}
	for _, cards := range [][]game.Card{rummy.DrawPile, rummy.FaceUp} {
		for _, card := range cards {
			add(card)
		}
	}
	for _, hand := range rummy.Hands {
		for _, card := range hand {
			add(card)
		}
	}
	if len(seen) != 27 {
		t.Fatalf("Rummy state accounts for %d/27 cards", len(seen))
	}
}

func TestRummyAIOpponentCompletesItsTurnAutomatically(t *testing.T) {
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.UnlockedHeroes = []string{"recruit", "warden", "hunter"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{rng: rand.New(rand.NewSource(620)), playerProfile: playerProfile}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	view := sendWebRequest(t, server.Client(), server.URL+"/api/new", map[string]any{
		"mode": "normal", "variant": "rummy", "playerHeroId": "warden",
	})
	if view.Phase != "rummy-opening" || view.Actor != 0 {
		t.Fatalf("new AI match = phase:%q actor:%d", view.Phase, view.Actor)
	}
	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
		"action": "rummy-draw-two",
	})
	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
		"action": "rummy-discard", "faceUpIndex": 0, "buriedIndex": 1,
	})
	if view.Phase != "rummy-turn" || view.Actor != 0 || view.Rummy.TurnsCompleted != 2 {
		t.Fatalf("CPU did not finish its turn: phase:%q actor:%d turns:%d",
			view.Phase, view.Actor, view.Rummy.TurnsCompleted)
	}
}

func TestRummySessionSnapshotRoundTrips(t *testing.T) {
	first, _ := heroes.ByID("warden")
	second, _ := heroes.ByID("hunter")
	web := &webServer{rng: rand.New(rand.NewSource(621))}
	web.startWebRummy("hvh", first.ID, second.ID, first, second)
	web.session.rummy.Hands[0] = append(web.session.rummy.Hands[0], web.session.rummy.DrawPile[0])
	web.session.rummy.DrawPile = web.session.rummy.DrawPile[1:]
	web.session.phase = "rummy-discard"

	restored := snapshotSession(web.session).restore()
	if restored.variant != "rummy" || restored.phase != "rummy-discard" ||
		len(restored.rummy.Hands[0]) != len(web.session.rummy.Hands[0]) ||
		len(restored.rummy.DrawPile) != len(web.session.rummy.DrawPile) ||
		restored.rummy.Parameters != web.session.rummy.Parameters {
		t.Fatalf("restored Rummy session = %+v", restored)
	}
}

func TestRummyDrawsBuriedCardsInBottomOrder(t *testing.T) {
	first := game.Card{Suit: game.Shield, Face: 1}
	second := game.Card{Suit: game.Fang, Face: 2}
	third := game.Card{Suit: game.Blade, Face: 3}
	faceUp := game.Card{Suit: game.Fang, Face: 4}
	rummy := &rummySession{
		DrawPile: []game.Card{first, second},
		Hands:    [2][]game.Card{{faceUp, third}, nil},
	}

	if err := rummy.discard(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if len(rummy.DrawPile) != 3 || rummy.DrawPile[0] != third {
		t.Fatalf("buried card was not placed at the bottom of the deck: %v", rummy.DrawPile)
	}
	if err := rummy.drawTwoForPlayer(1); err != nil {
		t.Fatal(err)
	}
	if len(rummy.Hands[1]) != 2 || rummy.Hands[1][0] != second || rummy.Hands[1][1] != first {
		t.Fatalf("drawn cards = %v, want %v then %v", rummy.Hands[1], second, first)
	}
	if err := rummy.drawFaceUp(1); err != nil {
		t.Fatal(err)
	}
	if len(rummy.Hands[1]) != 3 || rummy.Hands[1][2] != faceUp ||
		rummy.FaceUp[len(rummy.FaceUp)-1] != third || rummy.deckCount() != 0 {
		t.Fatalf("state after face-up draw = hand %v, face-up %v, deck count %d",
			rummy.Hands[1], rummy.FaceUp, rummy.deckCount())
	}
	if err := rummy.drawFaceUp(1); err == nil {
		t.Fatal("face-up draw succeeded after the deck was exhausted")
	}
}

func TestRummyHighDraftKeepsTwoCardsUntilFinalTurn(t *testing.T) {
	firstHero := heroes.Hero{Draft: 1}
	rummy := newRummySession(firstHero, heroes.Hero{}, rand.New(rand.NewSource(622)))
	if rummy.Parameters.HandSize != 7 || rummy.deckCount() != 13 {
		t.Fatalf("high-Draft deal has hand size %d and deck count %d",
			rummy.Parameters.HandSize, rummy.deckCount())
	}
	web := &webServer{session: &webSession{
		mode: "hvh", phase: "rummy-opening", actor: 0, rummy: rummy,
	}}

	for turn := 0; turn < rummyTurnLimit; turn++ {
		if turn > 0 {
			web.session.phase = "rummy-turn"
			web.session.actor = turn % 2
		}
		state := rummy.webView(web.session, 0, false)
		if state.DrawPileCount < 2 || !state.CanDrawTwo {
			t.Fatalf("turn %d began with %d deck cards and CanDrawTwo=%t",
				turn+1, state.DrawPileCount, state.CanDrawTwo)
		}
		if err := rummy.drawTwoForPlayer(web.session.actor); err != nil {
			t.Fatalf("turn %d draw: %v", turn+1, err)
		}
		if err := rummy.discard(web.session.actor, 0, 1); err != nil {
			t.Fatalf("turn %d discard: %v", turn+1, err)
		}
		if err := web.completeRummyTurn(); err != nil {
			t.Fatalf("turn %d completion: %v", turn+1, err)
		}
	}

	if web.session.phase != "gameover" || rummy.Turns != rummyTurnLimit || rummy.deckCount() != 1 {
		t.Fatalf("after final turn: phase %q, turns %d, deck count %d",
			web.session.phase, rummy.Turns, rummy.deckCount())
	}
}

func TestRummyRestoreMovesLegacyBuriedCardsToDeckBottom(t *testing.T) {
	firstBuried := game.Card{Suit: game.Shield, Face: 1}
	secondBuried := game.Card{Suit: game.Fang, Face: 2}
	drawTop := game.Card{Suit: game.Blade, Face: 3}
	snapshot := &sessionSnapshot{Rummy: &rummySession{
		DrawPile:       []game.Card{drawTop},
		PreviousBuried: []game.Card{firstBuried, secondBuried},
	}}

	restored := snapshot.restore().rummy
	want := []game.Card{firstBuried, secondBuried, drawTop}
	if len(restored.DrawPile) != len(want) || len(restored.PreviousBuried) != 0 {
		t.Fatalf("restored deck = %v, legacy buried cards = %v", restored.DrawPile, restored.PreviousBuried)
	}
	for index, card := range want {
		if restored.DrawPile[index] != card {
			t.Fatalf("restored deck = %v, want %v", restored.DrawPile, want)
		}
	}
}
