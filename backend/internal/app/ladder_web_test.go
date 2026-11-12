package app

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/ladder"
	"prisonersofwar/internal/profile"
)

func TestLadderOpponentLegalCountUsesResetTargetValue(t *testing.T) {
	chainSuit := game.Blade
	state := ladder.LadderState{
		ChainSuit: &chainSuit, SharedShift: 0, SharedFavor: 0, WildFaceRaw: 8,
	}
	candidate := game.Card{Suit: game.Shield, Face: 5}
	opponentHand := []game.Card{{Suit: game.Fang, Face: 4}}
	if got := ladderOpponentLegalCount(candidate, state, opponentHand); got != 1 {
		t.Fatalf("opponent legal count = %d, want 1", got)
	}
}

func TestLadderHardMissionsUsePerfectInformationMinimax(t *testing.T) {
	campaign, _ := campaigns.ByID("ladder")
	for _, index := range []int{13, 14} {
		t.Run(fmt.Sprintf("mission-%d", index+1), func(t *testing.T) {
			mission := campaign.Missions[index]
			playerHero, _ := heroes.ByID("warden")
			opponentHero := playerHero
			var stats *ladder.SharedStats
			if mission.Ladder.SharedStats != nil {
				stats = &ladder.SharedStats{
					Bounty: mission.Ladder.SharedStats.Bounty, Draft: mission.Ladder.SharedStats.Draft,
					Shift: mission.Ladder.SharedStats.Shift, Favor: mission.Ladder.SharedStats.Favor,
				}
			}
			match, err := ladder.NewCampaignGame(playerHero, opponentHero, ladder.CampaignSetup{
				PlayerHand: mission.Ladder.PlayerHand, CPUHand: mission.Ladder.CPUHand,
				PlayerSize: mission.Ladder.PlayerSize, CPUSize: mission.Ladder.CPUSize,
				WildFace: mission.Ladder.WildFace, BountySuit: mission.Ladder.BountySuit,
				SharedStats: stats,
			}, rand.New(rand.NewSource(74)))
			if err != nil {
				t.Fatal(err)
			}
			if state := match.State(); state.OpeningPending && state.OpeningCard != nil {
				if err := match.PlayOpeningCard(); err != nil {
					t.Fatal(err)
				}
			}
			state := match.State()
			card, ok := ladder.BestMinimaxPlay(match, state.Current)
			if !ok {
				t.Fatal("minimax did not find a legal play in the open-hand mission")
			}
			for _, legal := range match.LegalPlays(state.Current) {
				if card == legal {
					return
				}
			}
			t.Fatalf("minimax selected illegal card %s", card)
		})
	}
}

func TestLadderWebGameAndSnapshot(t *testing.T) {
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.UnlockedHeroes = []string{"recruit", "warden", "hunter"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	playerHero, _ := heroes.ByID("warden")
	opponentHero := playerHero
	var web *webServer
	for seed := int64(1); seed < 1000; seed++ {
		candidate := &webServer{
			rng: rand.New(rand.NewSource(seed)), playerProfile: playerProfile,
		}
		if err := candidate.startWebLadder("easy", playerHero.ID, opponentHero.ID, playerHero, opponentHero); err != nil {
			t.Fatal(err)
		}
		if candidate.session.actor == 0 {
			web = candidate
			break
		}
	}
	if web == nil {
		t.Fatal("could not find a deterministic human opening")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/action", web.handleAction)
	mux.HandleFunc("/api/state", web.handleState)
	server := httptest.NewServer(mux)
	defer server.Close()

	view := web.view()
	if !view.Started || view.Variant != "ladder" || view.Ladder == nil ||
		view.Phase != "ladder-opening-reveal" || view.Actor != 0 ||
		len(view.Ladder.Hand) == 0 || len(view.Ladder.Hand) != view.Ladder.HandCount ||
		view.Ladder.OpponentHandCount == 0 || !view.Ladder.OpeningPending ||
		view.Ladder.ChainCard != nil || len(view.Ladder.PlayedCards) != 0 ||
		time.Until(web.session.ladderReadyAt) < 2800*time.Millisecond {
		t.Fatalf("new Ladder game view = %+v", view)
	}
	for _, card := range view.Ladder.Hand {
		if card.Selectable {
			t.Fatal("opening hand should be visible but not playable")
		}
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
		!reflect.DeepEqual(restored.ladder.State(), web.session.ladder.State()) ||
		!restored.ladderReadyAt.Equal(web.session.ladderReadyAt) {
		t.Fatalf("Ladder state did not survive snapshot: restored=%+v current=%+v",
			restored.ladder, web.session.ladder)
	}

	web.session.ladderReadyAt = time.Now().Add(-time.Millisecond)
	view = getWebState(t, server.Client(), server.URL+"/api/state")
	if view.Phase != "ladder-thinking" || view.Actor != 1 || view.Ladder == nil ||
		view.Ladder.OpeningPending || len(view.Ladder.PlayedCards) != 1 ||
		time.Until(web.session.ladderReadyAt) > time.Second {
		t.Fatalf("opening card was not auto-played before the CPU delay: %+v", view)
	}

	for turn := 0; turn < 35 && view.Phase != "gameover"; turn++ {
		if view.Phase == "ladder-thinking" {
			web.session.ladderReadyAt = time.Now().Add(-time.Millisecond)
			view = getWebState(t, server.Client(), server.URL+"/api/state")
			continue
		}
		if view.Phase == "ladder-opening-choice" {
			if view.Actor != 0 || view.Ladder == nil || len(view.Ladder.Hand) == 0 {
				t.Fatalf("unexpected human opening choice at turn %d: %+v", turn, view)
			}
			view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
				"action": "ladder-open", "index": view.Ladder.Hand[0].Index,
			})
			continue
		}
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
		if view.Phase == "ladder-thinking" &&
			(time.Until(web.session.ladderReadyAt) <= 0 ||
				time.Until(web.session.ladderReadyAt) > time.Second) {
			t.Fatal("CPU action was not delayed by one second")
		}
	}
	if view.Phase != "gameover" || view.Ladder == nil || len(view.Ladder.FinalHands) != 2 ||
		view.Payout == nil {
		t.Fatalf("CPU Ladder match did not finish with visible results: %+v", view)
	}
	for index, hand := range view.Ladder.FinalHands {
		if hand == nil {
			t.Fatalf("final hand %d should be an empty array, not null", index)
		}
	}
}

func getWebState(t *testing.T, client *http.Client, url string) webView {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("state request failed (%d)", response.StatusCode)
	}
	var view webView
	if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	return view
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
	mux.HandleFunc("/api/state", web.handleState)
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
	if view.Phase != "ladder-opening-reveal" || !view.Ladder.OpeningPending ||
		len(view.Ladder.Hand) == 0 || time.Until(web.session.ladderReadyAt) < 2800*time.Millisecond {
		t.Fatalf("ready action did not reveal the opening hand: %+v", view)
	}
	web.session.ladderReadyAt = time.Now().Add(-time.Millisecond)
	view = getWebState(t, server.Client(), server.URL+"/api/state")
	if view.Phase != "handoff" || view.Ladder == nil || len(view.Ladder.Hand) != 0 ||
		view.Ladder.OpeningPending || len(view.Ladder.PlayedCards) != 1 {
		t.Fatalf("opening card was not auto-played before handing off: %+v", view)
	}
}

func TestLadderCPUOpeningIsScheduledAfterOneSecond(t *testing.T) {
	playerHero, _ := heroes.ByID("recruit")
	opponentHero, _ := heroes.ByID("recruit")
	var web *webServer
	for seed := int64(1); seed < 1000; seed++ {
		candidate := &webServer{rng: rand.New(rand.NewSource(seed))}
		if err := candidate.startWebLadder("easy", playerHero.ID, opponentHero.ID, playerHero, opponentHero); err != nil {
			t.Fatal(err)
		}
		if candidate.session.actor == 1 {
			web = candidate
			break
		}
	}

	if web == nil {
		t.Fatal("could not find a deterministic CPU opening")
	}
	if web.session.phase != "ladder-thinking" ||
		time.Until(web.session.ladderReadyAt) <= 0 ||
		time.Until(web.session.ladderReadyAt) > time.Second ||
		!web.session.ladder.State().OpeningPending {
		t.Fatalf("CPU opening was not scheduled for one second later: %+v", web.session)
	}
}

func TestLadderPassGivesHumanChoiceOfNextStack(t *testing.T) {
	playerProfile, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	playerProfile.UnlockedHeroes = []string{"recruit"}
	playerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	wild := game.Card{Suit: game.Fang, Face: 8}
	chainSuit := wild.Suit
	match := ladder.Restore(ladder.LadderState{
		HandA: []game.Card{{Suit: game.Fang, Face: 1}, {Suit: game.Blade, Face: 2}},
		HandB: []game.Card{{Suit: game.Shield, Face: 1}, {Suit: game.Shield, Face: 2}},
		PlayedCards: []game.Card{
			{Suit: game.Blade, Face: 7},
			wild,
		},
		ChainCard: &wild, ChainSuit: &chainSuit, ChainIsWild: true,
		Current: ladder.PlayerB, WildFaceRaw: wild.Face,
	})
	if legal := match.LegalPlays(ladder.PlayerB); len(legal) != 0 {
		t.Fatalf("CPU had a non-wild response to the wild: %v", legal)
	}
	if err := match.PassTurn(ladder.PlayerB); err != nil {
		t.Fatal(err)
	}
	web := &webServer{
		rng: rand.New(rand.NewSource(84)), playerProfile: playerProfile,
		session: &webSession{
			mode: "easy", variant: "ladder", ladder: match, phase: "ladder-play",
			actor: int(ladder.PlayerA), winner: -1,
		},
	}
	web.finishLadderAction(ladder.PlayerB)
	view := web.view()
	if view.Phase != "ladder-opening-choice" || view.Actor != int(ladder.PlayerA) ||
		view.Ladder == nil || !view.Ladder.OpeningChoice || len(view.Ladder.Hand) != 2 {
		t.Fatalf("pass did not prompt the human to choose a new stack: %+v", view)
	}
	for _, card := range view.Ladder.Hand {
		if !card.Selectable {
			t.Fatal("all cards should be selectable when opening a new stack")
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()
	selected := view.Ladder.Hand[1].Card
	view = sendWebRequest(t, server.Client(), server.URL+"/api/action", map[string]any{
		"action": "ladder-open", "index": view.Ladder.Hand[1].Index,
	})
	state := web.session.ladder.State()
	if view.Phase != "ladder-thinking" || state.OpeningPending ||
		state.ChainCard == nil || *state.ChainCard != selected ||
		!reflect.DeepEqual(state.PlayedCards, []game.Card{selected}) ||
		state.Current != ladder.PlayerB {
		t.Fatalf("selected card did not open a new stack for CPU response: view=%+v state=%+v", view, state)
	}
}

func TestLadderCPUChoosesNextStackAfterThinking(t *testing.T) {
	wild := game.Card{Suit: game.Fang, Face: 8}
	chainSuit := wild.Suit
	match := ladder.Restore(ladder.LadderState{
		HandA:       []game.Card{{Suit: game.Blade, Face: 1}, {Suit: game.Shield, Face: 2}},
		HandB:       []game.Card{{Suit: game.Blade, Face: 3}, {Suit: game.Shield, Face: 4}},
		PlayedCards: []game.Card{wild},
		ChainCard:   &wild, ChainSuit: &chainSuit, ChainIsWild: true,
		Current: ladder.PlayerA, WildFaceRaw: wild.Face,
	})
	if err := match.PassTurn(ladder.PlayerA); err != nil {
		t.Fatal(err)
	}
	web := &webServer{
		rng: rand.New(rand.NewSource(85)),
		session: &webSession{
			mode: "easy", variant: "ladder", ladder: match, phase: "ladder-play",
			actor: int(ladder.PlayerA), winner: -1,
		},
	}
	web.finishLadderAction(ladder.PlayerA)
	if err := web.advanceLadderAI(); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "ladder-thinking" ||
		time.Until(web.session.ladderReadyAt) <= 0 ||
		time.Until(web.session.ladderReadyAt) > time.Second {
		t.Fatalf("CPU opening choice was not delayed: %+v", web.session)
	}
	web.session.ladderReadyAt = time.Now().Add(-time.Millisecond)
	if err := web.advanceLadderAI(); err != nil {
		t.Fatal(err)
	}
	state := web.session.ladder.State()
	if state.OpeningPending || state.ChainCard == nil ||
		!containsGameCard(state.HandA, game.Card{Suit: game.Blade, Face: 1}) ||
		state.Current != ladder.PlayerA || len(state.PlayedCards) != 1 ||
		web.session.actor != int(ladder.PlayerA) || web.session.phase != "ladder-play" {
		t.Fatalf("CPU did not choose and open a fresh stack: state=%+v session=%+v", state, web.session)
	}
}

func containsGameCard(cards []game.Card, target game.Card) bool {
	for _, card := range cards {
		if card == target {
			return true
		}
	}
	return false
}
