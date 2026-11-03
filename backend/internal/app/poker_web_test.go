package app

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func TestPokerSessionDealsSixFromDraftFilteredDeck(t *testing.T) {
	hero := heroes.Hero{ID: "test", Draft: -1}
	poker, err := newPokerSession(hero.ID, hero, game.PokerMissionRules{
		NineBase: 12, BountyBase: 1, Par: 160,
	}, rand.New(rand.NewSource(901)))
	if err != nil {
		t.Fatal(err)
	}
	if poker.DeckFloor != 3 || len(poker.Hand) != pokerHandSize ||
		len(poker.DrawPile)+len(poker.Hand) != 21 || !poker.BountySuit.Valid() {
		t.Fatalf("new poker session = floor:%d hand:%d pile:%d bounty:%s",
			poker.DeckFloor, len(poker.Hand), len(poker.DrawPile), poker.BountySuit)
	}
	for _, card := range append(append([]game.Card(nil), poker.Hand...), poker.DrawPile...) {
		if card.Face < poker.DeckFloor {
			t.Fatalf("card %s is below deck floor %d", card, poker.DeckFloor)
		}
	}
}

func TestPokerPlaceAndRedrawRefillFromDrawPile(t *testing.T) {
	drawBottom := game.Card{Suit: game.Shield, Face: 1}
	drawTop := game.Card{Suit: game.Fang, Face: 2}
	hand := []game.Card{
		{Suit: game.Blade, Face: 1}, {Suit: game.Blade, Face: 2},
		{Suit: game.Blade, Face: 3}, {Suit: game.Blade, Face: 4},
		{Suit: game.Blade, Face: 5}, {Suit: game.Blade, Face: 6},
	}
	placed := hand[0]
	poker := &pokerSession{Hand: hand, DrawPile: []game.Card{drawBottom, drawTop}}
	if err := poker.place(0, 0); err != nil {
		t.Fatal(err)
	}
	if len(poker.Hand) != 6 || len(poker.Slots[0]) != 1 ||
		poker.Slots[0][0] != placed || poker.Hand[5] != drawTop {
		t.Fatalf("place/refill: hand %v, slot %v", poker.Hand, poker.Slots[0])
	}
	discard := poker.Hand[0]
	if err := poker.redraw(0); err != nil {
		t.Fatal(err)
	}
	if len(poker.Hand) != 6 || len(poker.DiscardPile) != 1 ||
		poker.DiscardPile[0] != discard || poker.Hand[5] != drawBottom ||
		len(poker.DrawPile) != 0 {
		t.Fatalf("redraw/refill: hand %v, discard %v, pile %v",
			poker.Hand, poker.DiscardPile, poker.DrawPile)
	}
}

func TestPokerSessionRejectsUnbalancedHandPlacement(t *testing.T) {
	hero := heroes.Hero{ID: "test"}
	poker, err := newPokerSession(hero.ID, hero, game.PokerMissionRules{}, rand.New(rand.NewSource(901)))
	if err != nil {
		t.Fatal(err)
	}
	poker.Slots[0] = make([]game.Card, 1)
	poker.Slots[1] = make([]game.Card, 1)
	view := poker.webView("poker-play")
	if view.CanPlace[0] || view.CanPlace[1] || !view.CanPlace[2] {
		t.Fatal("only Hand C should accept a card when it is below the other hand sizes")
	}
	if err := poker.place(0, 1); err == nil {
		t.Fatal("placing a second card in Hand B before Hand C should be rejected")
	}
	if err := poker.place(0, 2); err != nil {
		t.Fatalf("placing the first card in Hand C should be allowed: %v", err)
	}
}

func TestPokerMissionAwardsMarksOnParAndPersistsProgress(t *testing.T) {
	web := newPokerTestServer(t, 902)
	startingMarks := web.playerProfile.Marks
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "poker", MissionIndex: 1, PlayerHeroID: "warden",
	}); err != nil {
		t.Fatal(err)
	}
	poker := web.session.poker
	if web.session.variant != "poker" || web.session.phase != "poker-play" ||
		len(poker.Hand) != pokerHandSize || len(poker.DrawPile) != 18 ||
		poker.DeckFloor != 2 {
		t.Fatalf("started Poker mission = %+v", web.view().Poker)
	}
	fillPokerStraightFlushes(poker)
	if err := web.completePokerTurn("Test complete."); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "gameover" || web.session.winner != 0 ||
		web.playerProfile.Campaigns["poker"].MissionsCompleted != 1 ||
		web.playerProfile.Marks != startingMarks+400 {
		t.Fatalf("campaign completion: phase:%s winner:%d progress:%+v marks:%d",
			web.session.phase, web.session.winner, web.playerProfile.Campaigns["poker"],
			web.playerProfile.Marks)
	}
	restored := snapshotSession(web.session).restore()
	if restored.poker == nil || restored.poker.LastScore == nil ||
		restored.poker.LastScore.Total != poker.LastScore.Total ||
		restored.campaign.Index != 1 {
		t.Fatalf("restored poker session = %+v", restored)
	}
}

func TestPokerMarathonStreakRequiresSameHero(t *testing.T) {
	web := newPokerTestServer(t, 903)
	completed := make([]int, 11)
	for index := range completed {
		completed[index] = index + 1
	}
	web.playerProfile.Campaigns["poker"] = &profile.CampaignProgress{
		CampaignID: "poker", Started: true, MissionsCompleted: 11,
		TotalMissions: 15, CompletedMissionIDs: completed,
	}
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "poker", MissionIndex: 12, PlayerHeroID: "warden",
	}); err != nil {
		t.Fatal(err)
	}

	finishPokerTestGame(t, web)
	if web.session.phase != "poker-next-game" || web.session.poker.Streak != 1 {
		t.Fatalf("first marathon game = phase:%s streak:%d",
			web.session.phase, web.session.poker.Streak)
	}
	if err := web.startNextPokerGame("hunter"); err != nil {
		t.Fatal(err)
	}
	if web.session.poker.Streak != 0 {
		t.Fatalf("changing hero kept streak %d", web.session.poker.Streak)
	}
	finishPokerTestGame(t, web)
	if web.session.poker.Streak != 1 {
		t.Fatalf("new hero streak = %d, want 1", web.session.poker.Streak)
	}
	for web.session.phase == "poker-next-game" {
		if err := web.startNextPokerGame(""); err != nil {
			t.Fatal(err)
		}
		finishPokerTestGame(t, web)
	}
	if web.session.phase != "gameover" || web.session.winner != 0 ||
		web.session.poker.Streak != 3 || web.session.poker.GamesPlayed != 4 ||
		web.playerProfile.Campaigns["poker"].MissionsCompleted != 12 {
		t.Fatalf("marathon completion: phase:%s winner:%d streak:%d games:%d progress:%+v",
			web.session.phase, web.session.winner, web.session.poker.Streak,
			web.session.poker.GamesPlayed, web.playerProfile.Campaigns["poker"])
	}
}

func newPokerTestServer(t *testing.T, seed int64) *webServer {
	t.Helper()
	player, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	player.UnlockedHeroes = []string{"recruit", "warden", "hunter"}
	player.SetSaveHandler(func(*profile.Profile) error { return nil })
	return &webServer{
		rng: rand.New(rand.NewSource(seed)), playerProfile: player,
	}
}

func fillPokerStraightFlushes(poker *pokerSession) {
	for suit := game.Blade; suit <= game.Shield; suit++ {
		for face := 1; face <= 5; face++ {
			poker.Slots[suit] = append(poker.Slots[suit], game.Card{Suit: suit, Face: face})
		}
	}
}

func finishPokerTestGame(t *testing.T, web *webServer) {
	t.Helper()
	fillPokerStraightFlushes(web.session.poker)
	if err := web.completePokerTurn("Test complete."); err != nil {
		t.Fatal(err)
	}
}
