package app

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

// playWebMatchToEnd drives a browser Human-vs-AI match through every phase.
func playWebMatchToEnd(t *testing.T, client *http.Client, base, mode string) webView {
	t.Helper()
	view := sendWebRequest(t, client, base+"/api/new", map[string]any{"mode": mode})
	for step := 0; step < 80 && view.Phase != "gameover"; step++ {
		action := map[string]any{}
		switch view.Phase {
		case "draft":
			action = map[string]any{"action": "draft-pick", "index": 0}
			if view.Actor != 0 {
				action = map[string]any{"action": "draft-ai-pick"}
			}
		case "commit":
			action = map[string]any{"action": "commit", "cards": []int{0, 1, 2}}
		case "attack":
			action = map[string]any{"action": "attack", "index": 0}
		case "defend":
			action = map[string]any{"action": "defend", "index": 0}
		case "result":
			action = map[string]any{"action": "continue"}
		default:
			t.Fatalf("unexpected phase %q", view.Phase)
		}
		view = sendWebRequest(t, client, base+"/api/action", action)
	}
	if view.Phase != "gameover" {
		t.Fatalf("match did not finish; stopped in phase %q", view.Phase)
	}
	return view
}

// Browser matches must credit the same statistics that drive achievements
// (seat wins, per-mode streaks, games played) as campaign and terminal play.
func TestBrowserMatchesCreditSeatWinsAndMatchCounts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	web := &webServer{rng: rand.New(rand.NewSource(11)), playerProfile: p}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/new", web.handleNewGame)
	mux.HandleFunc("/api/action", web.handleAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	const matches = 8
	for i := 0; i < matches; i++ {
		playWebMatchToEnd(t, server.Client(), server.URL, "easy")
	}
	if p.TotalGames != matches {
		t.Fatalf("TotalGames = %d, want %d (counted once per match)", p.TotalGames, matches)
	}
	if p.TotalWins+p.TotalLosses != matches {
		t.Fatalf("wins %d + losses %d != %d matches", p.TotalWins, p.TotalLosses, matches)
	}
	if p.TotalWins == 0 {
		t.Fatal("seed produced no wins; choose a seed that exercises seat-win credit")
	}
	if got := p.WinsAsSeatX + p.WinsAsSeatY; got != p.TotalWins {
		t.Fatalf("seat wins X=%d + Y=%d = %d, want TotalWins %d", p.WinsAsSeatX, p.WinsAsSeatY, got, p.TotalWins)
	}
	if p.BestWinStreakByMode[0] == 0 {
		t.Fatalf("Easy-mode win streak was not tracked: %v", p.BestWinStreakByMode)
	}
}

func TestScoresAfterRoundUsesLastExchangeOfThatRound(t *testing.T) {
	web := &webServer{session: &webSession{battles: []webBattleRecord{
		{Round: 1, Scores: [2]int{3, 0}},
		{Round: 2, Scores: [2]int{3, 5}},
		{Round: 2, Scores: [2]int{7, 5}},
		{Round: 3, Scores: [2]int{7, 12}},
	}}}
	if got := web.scoresAfterRound(2); got != [2]int{7, 5} {
		t.Fatalf("scoresAfterRound(2) = %v, want [7 5]", got)
	}
	web.session.battles = nil
	if got := web.scoresAfterRound(2); got != [2]int{} {
		t.Fatalf("scoresAfterRound with no battles = %v, want zeros", got)
	}
}

func TestOnlineBattleAchievementResultTracksCanonicalModeAndPlayerView(t *testing.T) {
	web := &webServer{session: &webSession{
		variant: "main",
		state: game.GameState{
			Bounty:    game.Card{Suit: game.Blade},
			Modifiers: game.GameModifiers{},
			P1:        game.Player{Seat: game.SeatX, Jail: []game.Card{{Suit: game.Fang}}},
			P2:        game.Player{Seat: game.SeatY, Jail: []game.Card{{Suit: game.Blade}}},
		},
	}}
	result := onlineBattleAchievementResult(web, 1, "warden", true, false, [2]int{4, 8})
	if result.PlayerSeat != game.SeatY || result.PlayerScore != 8 || result.OpponentScore != 4 ||
		len(result.Games) != 1 || result.Games[0].PlayerHeroID != "warden" ||
		result.Games[0].PlayerJail[0].Suit != game.Blade || result.Games[0].OpponentJail[0].Suit != game.Fang {
		t.Fatalf("online match result did not use the player's perspective: %+v", result)
	}

	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.TotalWins = 1
	updateAchievementStatsForGameMode(p, result, economy.ModeRanked, "poker-duel", time.Now())
	ctx := achievements.NewMatchContext(p, result, economy.ModeIDRanked, result.PlayerSeat.String())
	ctx.GameMode = "poker-duel"
	events := achievements.EvaluateAll(ctx)
	if p.AchievementCounters["mode_wins_poker"] != 1 ||
		p.AchievementCounters["ranked_matches_played"] != 1 ||
		!hasAppAchievementUnlock(events, "mode_poker") {
		t.Fatalf("online achievement counters/events = %v / %+v", p.AchievementCounters, events)
	}
}

func hasAppAchievementUnlock(events []achievements.UnlockEvent, id string) bool {
	for _, event := range events {
		if event.AchievementID == id {
			return true
		}
	}
	return false
}
