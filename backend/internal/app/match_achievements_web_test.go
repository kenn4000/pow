package app

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
