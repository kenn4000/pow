package app

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/ladder"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/tugwar"
)

func TestReplaceAnyCardTokenAcrossCPUVariants(t *testing.T) {
	hero, ok := heroes.ByID("recruit")
	if !ok {
		t.Fatal("Recruit hero is unavailable")
	}
	type fixture struct {
		session *webSession
		source  string
		index   int
		card    func() game.Card
	}
	tests := []struct {
		name  string
		build func(*testing.T, *rand.Rand) fixture
	}{
		{
			name: "Main",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				state, err := game.NewGame(rng)
				if err != nil {
					t.Fatal(err)
				}
				session := &webSession{mode: "normal", variant: "main", state: state, phase: "commit", actor: 0}
				return fixture{session, "hand", 0, func() game.Card { return session.state.P1.Hand[0] }}
			},
		},
		{
			name: "Fog",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				state, err := game.NewGame(rng)
				if err != nil {
					t.Fatal(err)
				}
				session := &webSession{mode: "normal", variant: "fog", state: state, phase: "commit", actor: 0}
				return fixture{session, "hand", 0, func() game.Card { return session.state.P1.Hand[0] }}
			},
		},
		{
			name: "Puzzler",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				cards := []game.Card{{Suit: game.Blade, Face: 1}, {Suit: game.Fang, Face: 2}}
				puzzle := newPuzzleSession(puzzleDefinition{
					ID: "replace-test", Cards: cards, Par: 1, Direction: "high",
				}, hero)
				puzzle.DrawPile = replacementDeck(cards, rng)
				session := &webSession{mode: "easy", variant: "puzzler", phase: "puzzle-pick", actor: 0, puzzle: puzzle}
				return fixture{session, "puzzle", 0, func() game.Card { return session.puzzle.Definition.Cards[0] }}
			},
		},
		{
			name: "Abandon",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				state, err := newAbandonSessionForMission(hero, rng, 1, 5, 1)
				if err != nil {
					t.Fatal(err)
				}
				session := &webSession{mode: "easy", variant: "abandon", phase: "abandon-reveal", actor: 0, abandon: state}
				return fixture{session, abandonBoardSource, 0, func() game.Card { return session.abandon.Board[0] }}
			},
		},
		{
			name: "Abandon Duel",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				duel, err := newAbandonDuelGame(hero, hero, rng)
				if err != nil {
					t.Fatal(err)
				}
				session := &webSession{
					mode: "normal", variant: "abandon-duel", phase: "abandon-duel-reveal",
					actor: 0, abandonDuel: duel,
				}
				return fixture{session, abandonBoardSource, 0, func() game.Card { return session.abandonDuel.Board[0] }}
			},
		},
		{
			name: "Rummy",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				rummy := newRummySession(hero, hero, rng)
				session := &webSession{mode: "normal", variant: "rummy", phase: "rummy-turn", actor: 0, rummy: rummy}
				return fixture{session, "hand", 0, func() game.Card { return session.rummy.Hands[0][0] }}
			},
		},
		{
			name: "Ladder",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				match, err := ladder.NewLadderGame(hero, hero, nil, rng.Int63())
				if err != nil {
					t.Fatal(err)
				}
				state := match.State()
				state.Current = ladder.PlayerA
				match = ladder.Restore(state)
				session := &webSession{mode: "normal", variant: "ladder", phase: "ladder-play", actor: 0, ladder: match}
				return fixture{session, "hand", 0, func() game.Card { return session.ladder.State().HandA[0] }}
			},
		},
		{
			name: "Poker",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				poker, err := newPokerSession(hero.ID, hero, game.PokerMissionRules{}, rng)
				if err != nil {
					t.Fatal(err)
				}
				session := &webSession{mode: "easy", variant: "poker", phase: "poker-play", actor: 0, poker: poker}
				return fixture{session, "hand", 0, func() game.Card { return session.poker.Hand[0] }}
			},
		},
		{
			name: "Poker Duel",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				draw := game.Card{Suit: game.Shield, Face: 9}
				hand := game.Card{Suit: game.Blade, Face: 1}
				duel := &pokerDuelGame{
					ActivePlayer: 0, TurnNumber: 1, TurnStarted: time.Now().Add(-time.Second),
					Winner: -1,
					Players: [2]pokerDuelPlayer{
						{Hand: []game.Card{hand}, DrawPile: []game.Card{draw}},
						{},
					},
				}
				session := &webSession{mode: "normal", variant: "poker-duel", phase: "poker-duel-play", actor: 0, pokerDuel: duel}
				return fixture{session, "hand", 0, func() game.Card { return session.pokerDuel.Players[0].Hand[0] }}
			},
		},
		{
			name: "Tug War",
			build: func(t *testing.T, rng *rand.Rand) fixture {
				match, err := tugwar.NewGame(hero, hero, rng)
				if err != nil {
					t.Fatal(err)
				}
				session := &webSession{mode: "normal", variant: "tug-war", phase: "tug-war-play", actor: 0, tugWar: match}
				return fixture{session, "hand", 0, func() game.Card { return session.tugWar.State.Hands[0][0] }}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(42))
			testFixture := tt.build(t, rng)
			before := testFixture.card()
			phaseBefore := testFixture.session.phase
			p, err := profile.New(time.Now())
			if err != nil {
				t.Fatal(err)
			}
			p.ReplaceCardTokens = 1
			p.SetSaveHandler(func(*profile.Profile) error { return nil })
			web := &webServer{playerProfile: p, session: testFixture.session, rng: rng}
			response := callDashboardHandler(t, web.handleAction, "POST", "/api/action",
				map[string]any{"action": "use-replace-token", "source": testFixture.source, "index": testFixture.index})
			if response.Code != 200 {
				t.Fatalf("replacement status = %d: %s", response.Code, response.Body.String())
			}
			if after := testFixture.card(); after == before {
				t.Fatalf("replacement left the card unchanged: %s", before)
			}
			if p.ReplaceCardTokens != 0 || web.session.phase != phaseBefore {
				t.Fatalf("replacement state: tokens=%d phase=%q", p.ReplaceCardTokens, web.session.phase)
			}
		})
	}
}

func TestQuickBuyTokenUsesCrownsOnly(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marks, p.Crowns = 5000, 9
	p.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{
		playerProfile: p,
		session:       &webSession{mode: "normal", variant: "main", phase: "commit"},
	}
	response := callDashboardHandler(t, web.handleAction, "POST", "/api/action",
		map[string]string{"action": "quick-buy-token", "id": "replace-card-one"})
	if response.Code != 200 {
		t.Fatalf("quick buy status = %d: %s", response.Code, response.Body.String())
	}
	if p.ReplaceCardTokens != 1 || p.Crowns != 0 || p.Marks != 5000 {
		t.Fatalf("quick buy used wrong currency or quantity: tokens=%d Crowns=%d Marks=%d",
			p.ReplaceCardTokens, p.Crowns, p.Marks)
	}
}

func TestReplaceAnyCardTokenRejectsHumanVersusHuman(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	state, err := game.NewGame(rng)
	if err != nil {
		t.Fatal(err)
	}
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.ReplaceCardTokens = 1
	p.SetSaveHandler(func(*profile.Profile) error { return nil })
	web := &webServer{
		playerProfile: p, rng: rng,
		session: &webSession{mode: "hvh", variant: "main", state: state, phase: "commit", actor: 0},
	}
	response := callDashboardHandler(t, web.handleAction, "POST", "/api/action",
		map[string]any{"action": "use-replace-token", "source": "hand", "index": 0})
	if response.Code != 400 {
		t.Fatalf("Human-vs-Human replacement status = %d, want 400: %s", response.Code, response.Body.String())
	}
	if p.ReplaceCardTokens != 1 {
		t.Fatalf("rejected replacement consumed a token: %d", p.ReplaceCardTokens)
	}
}
