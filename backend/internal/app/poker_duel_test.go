package app

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

func TestNewPokerDuelGameDealsEachPlayerAnIdenticalDeck(t *testing.T) {
	heroP1, _ := heroes.ByID("warden")
	heroP2, _ := heroes.ByID("treasurer")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	duel, err := newPokerDuelGame(heroP1, heroP2, rand.New(rand.NewSource(7)), now)
	if err != nil {
		t.Fatal(err)
	}
	if duel.SharedStats.Bounty != 1 || duel.SharedStats.Favored != 1 ||
		duel.SharedStats.Shift != 0 || duel.SharedStats.Draft != 0 || duel.SharedStats.DeckFloor != 2 {
		t.Fatalf("unexpected shared stats: %+v", duel.SharedStats)
	}
	if len(duel.Players[0].Hand) != pokerDuelHandSize || len(duel.Players[1].Hand) != pokerDuelHandSize ||
		len(duel.Players[0].DrawPile) != 18 || len(duel.Players[1].DrawPile) != 18 {
		t.Fatalf("unexpected deal or draw piles: hands %d/%d, piles %d/%d",
			len(duel.Players[0].Hand), len(duel.Players[1].Hand),
			len(duel.Players[0].DrawPile), len(duel.Players[1].DrawPile))
	}
	decks := make([]map[game.Card]bool, len(duel.Players))
	for playerIndex, player := range duel.Players {
		seen := make(map[game.Card]bool)
		for _, card := range append(append([]game.Card(nil), player.Hand...), player.DrawPile...) {
			if seen[card] {
				t.Fatalf("duplicate card in deal: %v", card)
			}
			seen[card] = true
		}
		decks[playerIndex] = seen
	}
	if len(decks[0]) != 24 || len(decks[1]) != 24 {
		t.Fatalf("filtered deck sizes are %d/%d, want 24 each", len(decks[0]), len(decks[1]))
	}
	for card := range decks[0] {
		if !decks[1][card] {
			t.Fatalf("second player's deck is missing %s", card)
		}
	}
}

func TestNewPokerDuelGameUsesSharedDraftForTheDeckFloor(t *testing.T) {
	heroP1, _ := heroes.ByID("anchor")
	heroP2, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	duel, err := newPokerDuelGame(heroP1, heroP2, rand.New(rand.NewSource(8)), now)
	if err != nil {
		t.Fatal(err)
	}
	if duel.SharedStats.Draft != 1 || duel.SharedStats.DeckFloor != 1 {
		t.Fatalf("shared Draft floor = %d/%d, want 1/1", duel.SharedStats.Draft, duel.SharedStats.DeckFloor)
	}
	for playerIndex, player := range duel.Players {
		cards := len(player.Hand) + len(player.DrawPile)
		if cards != 27 {
			t.Fatalf("player %d deck contains %d cards at floor 1, want 27", playerIndex, cards)
		}
	}
}

func TestPokerDuelTurnBanksRemainingTimeAndHidesPrivateCards(t *testing.T) {
	heroP1, _ := heroes.ByID("recruit")
	heroP2, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(11))
	duel, err := newPokerDuelGame(heroP1, heroP2, rng, now)
	if err != nil {
		t.Fatal(err)
	}
	duel.ActivePlayer = 0
	duel.Players[0].Table[1] = []game.Card{duel.Players[0].Hand[0]}
	duel.Players[0].Hand = removePokerDuelCard(duel.Players[0].Hand, 0)
	view := duel.webView("poker-duel-play", now, -1)
	if len(view.Players[0].Hand) != pokerDuelHandSize-1 || !view.Players[1].HandHidden ||
		!view.Players[1].Table[1].Hidden || view.Players[1].Table[0].Hidden {
		t.Fatal("active view exposed opponent private cards or hid the public first hand")
	}
	if !view.Players[0].Table[0].CanPlace || view.Players[0].Table[1].CanPlace ||
		!view.Players[0].Table[2].CanPlace {
		t.Fatal("active view offered placements that violate balanced hand sizes")
	}
	duel.ActivePlayer = 1
	duel.TurnStarted = now
	cpuTurnView := duel.webView("poker-duel-thinking", now.Add(5*time.Second), 1)
	if len(cpuTurnView.Players[0].Hand) != len(duel.Players[0].Hand) ||
		!cpuTurnView.Players[1].HandHidden || cpuTurnView.TurnRemainingMS != 10_000 {
		t.Fatal("CPU turn hid the player's hand or failed to advance the turn clock")
	}
	opponentView := duel.webView("poker-duel-play", now, 0)
	if len(opponentView.Players[1].Hand) != len(duel.Players[1].Hand) ||
		!opponentView.Players[0].HandHidden {
		t.Fatal("online view did not keep the viewing player's hand private and visible")
	}
	duel.ActivePlayer = 0
	handoffView := duel.webView("handoff", now, -1)
	if !handoffView.Players[0].HandHidden || !handoffView.Players[1].HandHidden ||
		!handoffView.Players[0].Table[1].Hidden || !handoffView.Players[1].Table[1].Hidden {
		t.Fatal("handoff view exposed private cards")
	}

	if err := duel.takeTurn(0, 0, false, now.Add(4*time.Second), rng); err != nil {
		t.Fatal(err)
	}
	if duel.Players[0].TimeBank != 11*time.Second {
		t.Fatalf("time bank = %s, want 11s", duel.Players[0].TimeBank)
	}
	if duel.ActivePlayer != 1 || duel.TurnNumber != 2 {
		t.Fatalf("turn did not advance: active player %d, turn %d", duel.ActivePlayer, duel.TurnNumber)
	}
}

func TestPokerDuelRejectsUnbalancedHandPlacement(t *testing.T) {
	hero, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(12))
	duel, err := newPokerDuelGame(hero, hero, rng, now)
	if err != nil {
		t.Fatal(err)
	}
	player := &duel.Players[0]
	player.Table[0] = append(player.Table[0], player.Hand[0])
	player.Hand = removePokerDuelCard(player.Hand, 0)
	player.Table[1] = append(player.Table[1], player.Hand[0])
	player.Hand = removePokerDuelCard(player.Hand, 0)
	duel.ActivePlayer = 0
	duel.TurnStarted = now

	if err := duel.takeTurn(0, 1, false, now.Add(time.Second), rng); err == nil {
		t.Fatal("placing a second card in Hand B before Hand C should be rejected")
	}
	if err := duel.takeTurn(0, 2, false, now.Add(time.Second), rng); err != nil {
		t.Fatalf("placing the first card in Hand C should be allowed: %v", err)
	}
}

func TestPokerDuelSessionUsesPrivateHandoffs(t *testing.T) {
	hero, _ := heroes.ByID("warden")
	now := time.Now()
	rng := rand.New(rand.NewSource(13))
	duel, err := newPokerDuelGame(hero, hero, rng, now)
	if err != nil {
		t.Fatal(err)
	}
	duel.ActivePlayer = 0
	duel.TurnStarted = now
	server := &webServer{
		rng: rng,
		session: &webSession{
			mode: "hvh", variant: "poker-duel", pokerDuel: duel,
			phase: "poker-duel-play", actor: 0, winner: -1,
		},
	}
	if err := server.applyAction(actionRequest{Action: "poker-duel-place", Index: 0, Slot: 0}); err != nil {
		t.Fatal(err)
	}
	if server.session.phase != "handoff" || server.session.handoffTarget != 1 ||
		server.session.handoffPhase != "poker-duel-play" {
		t.Fatalf("turn change did not request a private handoff: %+v", server.session)
	}
	if err := server.applyAction(actionRequest{Action: "ready"}); err != nil {
		t.Fatal(err)
	}
	if server.session.phase != "poker-duel-play" || server.session.actor != 1 ||
		server.session.pokerDuel.TurnStarted.Before(now) {
		t.Fatalf("ready action did not start Player 2's clock: %+v", server.session)
	}
}

func TestPokerDuelTimeoutPassesWithoutPlacingCard(t *testing.T) {
	hero, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(19))
	duel, err := newPokerDuelGame(hero, hero, rng, now)
	if err != nil {
		t.Fatal(err)
	}
	duel.ActivePlayer = 0
	hand := append([]game.Card(nil), duel.Players[0].Hand...)
	if err := duel.handleTimeout(now.Add(15*time.Second), rng); err != nil {
		t.Fatal(err)
	}
	if duel.Players[0].TimeBank != 0 || len(duel.Players[0].Table[0]) != 0 ||
		len(duel.Players[0].Hand) != len(hand) ||
		duel.Players[0].Hand[0] != hand[0] ||
		duel.ActivePlayer != 1 || duel.TurnNumber != 2 {
		t.Fatalf("timeout outcome incorrect: bank=%s hand=%v table=%v active=%d turn=%d",
			duel.Players[0].TimeBank, duel.Players[0].Hand, duel.Players[0].Table[0],
			duel.ActivePlayer, duel.TurnNumber)
	}
}

func TestPokerDuelExpiredTurnStillUsesExplicitlyChosenPlacement(t *testing.T) {
	hero, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(20))
	duel, err := newPokerDuelGame(hero, hero, rng, now)
	if err != nil {
		t.Fatal(err)
	}
	duel.ActivePlayer = 0
	card := duel.Players[0].Hand[0]
	if err := duel.takeTurn(0, 2, false, now.Add(15*time.Second), rng); err != nil {
		t.Fatal(err)
	}
	if len(duel.Players[0].Table[2]) != 1 || duel.Players[0].Table[2][0] != card {
		t.Fatal("expired turn did not honor the player's selected card and hand")
	}
}

func TestPokerDuelRedrawOnlyConsumesTheActivePlayersPile(t *testing.T) {
	hero, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(21))
	duel, err := newPokerDuelGame(hero, hero, rng, now)
	if err != nil {
		t.Fatal(err)
	}
	duel.ActivePlayer = 0
	activePileCount := len(duel.Players[0].DrawPile)
	otherPileCount := len(duel.Players[1].DrawPile)
	if err := duel.takeTurn(0, 0, true, now.Add(time.Second), rng); err != nil {
		t.Fatal(err)
	}
	if len(duel.Players[0].DrawPile) != activePileCount-1 || len(duel.Players[1].DrawPile) != otherPileCount ||
		len(duel.Players[0].DiscardPile) != 1 {
		t.Fatalf("redraw changed the wrong pile: draw=%d/%d discards=%d",
			len(duel.Players[0].DrawPile), len(duel.Players[1].DrawPile), len(duel.Players[0].DiscardPile))
	}
	if err := duel.validateState(); err != nil {
		t.Fatal(err)
	}
}

func TestPokerDuelDefaultScoringAndUncappedBankBonus(t *testing.T) {
	hero, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	duel, err := newPokerDuelGame(hero, hero, rand.New(rand.NewSource(22)), now)
	if err != nil {
		t.Fatal(err)
	}
	if duel.Rules.NineBase != 12 || duel.Rules.BountyBase != 1 || duel.Rules.Constraint != "" ||
		duel.Rules.Par != 0 || duel.Rules.StreakCount != 0 || duel.Rules.StreakScore != 0 {
		t.Fatalf("standalone Poker duel has campaign scoring rules: %+v", duel.Rules)
	}
	duel.Players[0].TimeBank = 24 * time.Hour
	score, err := duel.scorePlayer(0)
	if err != nil {
		t.Fatal(err)
	}
	if !score.ConstraintMet || score.BankBonus != 7200 || score.FinalScore != score.BankBonus {
		t.Fatalf("uncapped time bank bonus incorrect: %+v", score)
	}
}

func TestPokerDuelCompletesAsOneStandaloneGame(t *testing.T) {
	hero, _ := heroes.ByID("warden")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	duel, err := newPokerDuelGame(hero, hero, rand.New(rand.NewSource(23)), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := duel.completeGame(); err != nil {
		t.Fatal(err)
	}
	if !duel.GameOver || len(duel.FinalScores) != 2 {
		t.Fatalf("standalone game did not finish with both scores: %+v", duel)
	}
}

func TestPokerDuelWinnerUsesFinalScoreThenRawScore(t *testing.T) {
	tests := []struct {
		name     string
		scores   []pokerDuelPlayerScore
		winner   int
		tiebreak string
	}{
		{
			name: "final score", scores: []pokerDuelPlayerScore{
				{ConstraintMet: true, GameScore: 80, FinalScore: 82},
				{ConstraintMet: true, GameScore: 82, FinalScore: 81},
			}, winner: 0, tiebreak: "final_score",
		},
		{
			name: "raw score", scores: []pokerDuelPlayerScore{
				{ConstraintMet: true, GameScore: 80, FinalScore: 81},
				{ConstraintMet: true, GameScore: 82, FinalScore: 81},
			}, winner: 1, tiebreak: "game_score",
		},
		{
			name: "tie", scores: []pokerDuelPlayerScore{
				{ConstraintMet: true, GameScore: 80, FinalScore: 81},
				{ConstraintMet: true, GameScore: 80, FinalScore: 81},
			}, winner: -1, tiebreak: "game_score_tied",
		},
		{
			name: "one constraint failure", scores: []pokerDuelPlayerScore{
				{ConstraintMet: false}, {ConstraintMet: true},
			}, winner: 1, tiebreak: "constraint",
		},
		{
			name: "both constraints fail", scores: []pokerDuelPlayerScore{
				{ConstraintMet: false}, {ConstraintMet: false},
			}, winner: -1, tiebreak: "both_constraints_failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			winner, tiebreak := pokerDuelWinner(test.scores)
			if winner != test.winner || tiebreak != test.tiebreak {
				t.Fatalf("winner = %d by %q, want %d by %q", winner, tiebreak, test.winner, test.tiebreak)
			}
		})
	}
}
