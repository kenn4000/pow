package app

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

func pokerDuelCPUFixture(t *testing.T, mode string, seed int64) (*webServer, time.Time) {
	t.Helper()
	hero, ok := heroes.ByID("warden")
	if !ok {
		t.Fatal("warden hero is unavailable")
	}
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(seed))
	duel, err := newPokerDuelGame(hero, hero, rng, now)
	if err != nil {
		t.Fatal(err)
	}
	duel.ActivePlayer = 1
	duel.TurnStarted = now
	return &webServer{
		rng: rng,
		session: &webSession{
			mode: mode, variant: "poker-duel", pokerDuel: duel,
			phase: "poker-duel-thinking", actor: 1, winner: -1,
		},
	}, now
}

func TestPokerDuelCPUChoosesOnlyLegalActions(t *testing.T) {
	server, _ := pokerDuelCPUFixture(t, "normal", 101)
	duel := server.session.pokerDuel
	action, hasAction, err := choosePokerDuelCPUAction(duel, "normal", rand.New(rand.NewSource(102)))
	if err != nil {
		t.Fatal(err)
	}
	if !hasAction {
		t.Fatal("CPU had no action despite an available hand and draw pile")
	}
	switch action.Action {
	case "poker-duel-place":
		if action.Index < 0 || action.Index >= len(duel.Players[1].Hand) ||
			action.Slot < 0 || action.Slot >= len(duel.Players[1].Table) ||
			!game.CanPlacePokerCard(duel.Players[1].Table, action.Slot) {
			t.Fatalf("CPU selected an illegal placement: %+v", action)
		}
	case "poker-duel-redraw":
		if action.Index < 0 || action.Index >= len(duel.Players[1].Hand) ||
			len(duel.Players[1].DrawPile) == 0 {
			t.Fatalf("CPU selected an illegal redraw: %+v", action)
		}
	default:
		t.Fatalf("CPU selected unknown action %q", action.Action)
	}
}

func TestPokerDuelCPUUsesValidatedTurnAndBanksRemainingTime(t *testing.T) {
	server, now := pokerDuelCPUFixture(t, "hard", 103)
	duel := server.session.pokerDuel
	action, hasAction, err := choosePokerDuelCPUAction(duel, "hard", rand.New(rand.NewSource(104)))
	if err != nil {
		t.Fatal(err)
	}
	if !hasAction {
		t.Fatal("CPU had no legal action")
	}
	server.session.phase = "poker-duel-play"
	if err := server.applyPokerDuelTurn(action, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	if duel.Players[1].TimeBank != 11*time.Second {
		t.Fatalf("CPU time bank = %s, want 11s", duel.Players[1].TimeBank)
	}
	if duel.TurnNumber != 2 || duel.ActivePlayer != 0 || server.session.phase != "poker-duel-play" {
		t.Fatalf("CPU turn did not advance to the human: turn=%d active=%d phase=%q",
			duel.TurnNumber, duel.ActivePlayer, server.session.phase)
	}
	if err := duel.validateState(); err != nil {
		t.Fatalf("CPU produced invalid game state: %v", err)
	}
}

func TestPokerDuelCPUWaitsForPersistedThinkTimeAndRestores(t *testing.T) {
	server, _ := pokerDuelCPUFixture(t, "hard", 110)
	duel := server.session.pokerDuel
	started := time.Now()
	duel.TurnStarted = started
	server.session.phase = "poker-duel-play"
	if err := server.schedulePokerDuelCPU(); err != nil {
		t.Fatal(err)
	}
	readyAt := server.session.pokerDuelCPUReadyAt
	if server.session.phase != "poker-duel-thinking" ||
		readyAt.Before(started.Add(2*time.Second)) ||
		readyAt.After(started.Add(6*time.Second)) {
		t.Fatalf("unexpected CPU schedule: phase=%q ready=%v", server.session.phase, readyAt)
	}
	restored := snapshotSession(server.session).restore()
	if !restored.pokerDuelCPUReadyAt.Equal(readyAt) {
		t.Fatalf("restored CPU ready time = %v, want %v", restored.pokerDuelCPUReadyAt, readyAt)
	}
	if err := server.applyPokerDuelCPUAction(); err == nil {
		t.Fatal("CPU acted before its think delay expired")
	}

	readyAt = time.Now().Add(-time.Second)
	server.session.pokerDuelCPUReadyAt = readyAt
	duel.TurnStarted = readyAt.Add(-3 * time.Second)
	if err := server.applyPokerDuelCPUAction(); err != nil {
		t.Fatal(err)
	}
	if duel.Players[1].TimeBank != 12*time.Second {
		t.Fatalf("CPU time bank = %s, want 12s from a 3s simulated think", duel.Players[1].TimeBank)
	}
	if !server.session.pokerDuelCPUReadyAt.IsZero() {
		t.Fatal("CPU ready time was not cleared after acting")
	}
}

func TestPokerDuelCPUViewKeepsPlayersHandVisibleAndCPUCardsHidden(t *testing.T) {
	server, now := pokerDuelCPUFixture(t, "easy", 111)
	view := server.session.pokerDuel.webView("poker-duel-thinking", now, 1)
	if !view.Players[1].HandHidden || !view.Players[1].Table[1].Hidden ||
		len(view.Players[0].Hand) == 0 || view.Players[0].HandHidden {
		t.Fatal("CPU thinking view hid the player's hand or exposed the CPU's private cards")
	}
	if view.Players[1].Table[0].Hidden {
		t.Fatal("CPU's public first hand was hidden")
	}
}

func TestPokerDuelRejectsHumanActionsDuringCPUturn(t *testing.T) {
	server, _ := pokerDuelCPUFixture(t, "easy", 105)
	server.session.phase = "poker-duel-play"
	if err := server.applyPokerDuelAction(actionRequest{Action: "poker-duel-place", Index: 0, Slot: 0}); err == nil {
		t.Fatal("accepted a human-submitted action for the CPU player")
	}
}

func TestPokerDuelCPUDoneAndDifficultyProfiles(t *testing.T) {
	server, _ := pokerDuelCPUFixture(t, "easy", 106)
	server.session.pokerDuel.Players[1].Done = true
	if _, hasAction, err := choosePokerDuelCPUAction(
		server.session.pokerDuel, "easy", rand.New(rand.NewSource(107)),
	); err != nil || hasAction {
		t.Fatalf("done CPU returned action=%t, err=%v", hasAction, err)
	}

	tests := []struct {
		mode        string
		minimum     time.Duration
		maximum     time.Duration
		errorRate   float64
		threshold   float64
		fullScoring bool
	}{
		{mode: "easy", minimum: 5 * time.Second, maximum: 12 * time.Second, errorRate: 0.15, threshold: 5},
		{mode: "normal", minimum: 4 * time.Second, maximum: 8 * time.Second, errorRate: 0.05, threshold: 10, fullScoring: true},
		{mode: "hard", minimum: 2 * time.Second, maximum: 6 * time.Second, threshold: 15, fullScoring: true},
	}
	for _, test := range tests {
		profile, err := pokerDuelCPUProfileForMode(test.mode)
		if err != nil {
			t.Fatal(err)
		}
		if profile.minThink != test.minimum || profile.maxThink != test.maximum ||
			profile.errorRate != test.errorRate || profile.redrawThreshold != test.threshold ||
			profile.useFullScoring != test.fullScoring {
			t.Errorf("%s profile = %+v", test.mode, profile)
		}
		for seed := int64(0); seed < 100; seed++ {
			duration, err := pokerDuelCPUThinkingDuration(test.mode, rand.New(rand.NewSource(seed)))
			if err != nil {
				t.Fatal(err)
			}
			if duration < test.minimum || duration > test.maximum || duration > 15*time.Second {
				t.Errorf("%s think duration %s outside expected range", test.mode, duration)
			}
		}
	}
}

func TestPokerDuelCPUEasyErrorRateVariesActions(t *testing.T) {
	server, _ := pokerDuelCPUFixture(t, "easy", 108)
	seen := make(map[string]bool)
	for seed := int64(0); seed < 500; seed++ {
		action, hasAction, err := choosePokerDuelCPUAction(
			server.session.pokerDuel, "easy", rand.New(rand.NewSource(seed)),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !hasAction {
			t.Fatal("CPU unexpectedly had no legal action")
		}
		seen[action.Action+":"+string(rune('0'+action.Index))+":"+string(rune('0'+action.Slot))] = true
	}
	if len(seen) < 2 {
		t.Fatal("Easy CPU error rate did not produce varied actions")
	}
}

func TestPokerDuelCPUFullHandEvaluationMatchesScoring(t *testing.T) {
	server, _ := pokerDuelCPUFixture(t, "hard", 109)
	duel := server.session.pokerDuel
	hand := []game.Card{
		{Suit: game.Blade, Face: 4},
		{Suit: game.Fang, Face: 4},
		{Suit: game.Shield, Face: 4},
		{Suit: game.Blade, Face: 5},
		{Suit: game.Fang, Face: 5},
	}
	got, err := pokerDuelCPUHandValue(duel, hand, 0)
	if err != nil {
		t.Fatal(err)
	}
	var hands [3][]game.Card
	hands[0] = hand
	want, err := game.ScorePokerTable(hands, duel.sharedHeroParameters(), duel.BountySuit, duel.Rules)
	if err != nil {
		t.Fatal(err)
	}
	if got != want.Hands[0].Total {
		t.Fatalf("full-hand heuristic = %d, want exact hand score %d", got, want.Hands[0].Total)
	}
}
