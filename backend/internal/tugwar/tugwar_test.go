package tugwar

import (
	"math/rand"
	"testing"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

func testGame() *Game {
	return &Game{State: GameState{
		BountySuit: game.Blade,
		Winner:     -1,
		Current:    0,
		Round:      1,
		Piles: [NumPiles]Pile{
			{Cards: []game.Card{{Suit: game.Blade, Face: 3}}},
			{Cards: []game.Card{{Suit: game.Fang, Face: 3}}},
			{Cards: []game.Card{{Suit: game.Shield, Face: 3}}},
		},
	}}
}

func TestNewGameDealsAndDerivesHeroStats(t *testing.T) {
	first, _ := heroes.ByID("recruit")
	second, _ := heroes.ByID("hunter")
	g, err := NewGame(first, second, rand.New(rand.NewSource(7)))
	if err != nil {
		t.Fatal(err)
	}
	if g.State.S != 0 || g.State.F != 0 || g.State.B != 3 {
		t.Fatalf("derived stats = S:%d F:%d B:%d, want 0, 0, 3", g.State.S, g.State.F, g.State.B)
	}
	if g.State.Pushes != [2]int{1, 1} {
		t.Fatalf("pushes = %v, want [1 1]", g.State.Pushes)
	}
	if len(g.State.Hands[0]) != HandSize || len(g.State.Hands[1]) != HandSize ||
		len(g.State.Deck) != 11 || len(g.State.Piles) != NumPiles {
		t.Fatalf("unexpected setup sizes: hands %d/%d, deck %d, piles %d",
			len(g.State.Hands[0]), len(g.State.Hands[1]), len(g.State.Deck), len(g.State.Piles))
	}
	seen := make(map[game.Card]bool, 26)
	add := func(card game.Card) {
		t.Helper()
		if seen[card] {
			t.Fatalf("duplicate card in setup: %v", card)
		}
		seen[card] = true
	}
	for _, hand := range g.State.Hands {
		for _, card := range hand {
			add(card)
		}
	}
	for _, pile := range g.State.Piles {
		for _, card := range pile.Cards {
			add(card)
		}
	}
	for _, card := range g.State.Deck {
		add(card)
	}
	if len(seen) != 26 {
		t.Fatalf("setup contains %d cards, want 26", len(seen))
	}
}

func TestNewGameStartsAtBaseBountyWithoutHeroModifiers(t *testing.T) {
	recruit, _ := heroes.ByID("recruit")
	g, err := NewGame(recruit, recruit, rand.New(rand.NewSource(8)))
	if err != nil {
		t.Fatal(err)
	}
	if g.State.B != 1 {
		t.Fatalf("base bounty = %d, want 1", g.State.B)
	}
}

func TestNewCampaignGameUsesScriptedSetupAndCampaignStatFormula(t *testing.T) {
	recruit, _ := heroes.ByID("recruit")
	turtle, _ := heroes.ByID("anchor")
	mission := CampaignMission{
		PlayerHand: []game.Card{{Suit: game.Shield, Face: 2}},
		CPUHand:    []game.Card{{Suit: game.Fang, Face: 3}},
		Piles: [NumPiles]Pile{
			{Cards: []game.Card{{Suit: game.Blade, Face: 1}}},
			{Cards: []game.Card{{Suit: game.Fang, Face: 2}}},
			{Cards: []game.Card{{Suit: game.Shield, Face: 3}}},
		},
		Deck:       []game.Card{{Suit: game.Shield, Face: 1}, {Suit: game.Blade, Face: 2}},
		BountySuit: game.Blade, Pushes: [2]int{3, 0}, WinCondition: WinByScore,
	}
	match, err := NewCampaignGame(recruit, turtle, rand.New(rand.NewSource(21)), mission)
	if err != nil {
		t.Fatal(err)
	}
	if match.State.B != -1 || match.State.Pushes != [2]int{3, 0} ||
		match.State.Deck[len(match.State.Deck)-1] != mission.Deck[0] {
		t.Fatalf("scripted campaign stats/deck = B:%d pushes:%v deck:%v",
			match.State.B, match.State.Pushes, match.State.Deck)
	}
	if err := match.Apply(Action{Kind: ActionFlipDeck, PileIndex: 0}); err != nil {
		t.Fatal(err)
	}
	if match.State.Piles[0].Cards[len(match.State.Piles[0].Cards)-1] != mission.Deck[0] ||
		!match.State.BountyRevealed {
		t.Fatalf("first scripted deck flip = %+v", match.State)
	}
}

func TestCampaignSeriesUsesStandardDealWithCampaignStatFormula(t *testing.T) {
	recruit, _ := heroes.ByID("recruit")
	mission := CampaignMission{RefillHands: true, SeriesGames: 3, SeriesWinsNeeded: 2}
	match, err := NewCampaignGame(recruit, recruit, rand.New(rand.NewSource(22)), mission)
	if err != nil {
		t.Fatal(err)
	}
	if len(match.State.Hands[0]) != HandSize || len(match.State.Hands[1]) != HandSize ||
		len(match.State.Deck) != 11 || match.State.B != 0 ||
		match.State.Pushes[0] != match.State.Pushes[1] {
		t.Fatalf("campaign series standard setup = %+v", match.State)
	}
}

func TestNewGameClampsHeroStats(t *testing.T) {
	gorilla, _ := heroes.ByID("ascendant")
	treasurer, _ := heroes.ByID("treasurer")
	bodybuilder, _ := heroes.ByID("herald")
	g, err := NewGame(gorilla, gorilla, rand.New(rand.NewSource(11)))
	if err != nil {
		t.Fatal(err)
	}
	if g.State.S != 3 || g.State.Pushes != [2]int{1, 1} {
		t.Fatalf("upper shift/draft clamps = S:%d pushes:%v, want S:3 pushes:[1 1]", g.State.S, g.State.Pushes)
	}
	g, err = NewGame(treasurer, treasurer, rand.New(rand.NewSource(12)))
	if err != nil {
		t.Fatal(err)
	}
	if g.State.B != -1 {
		t.Fatalf("lower bounty clamp = %d, want -1", g.State.B)
	}
	g, err = NewGame(bodybuilder, bodybuilder, rand.New(rand.NewSource(13)))
	if err != nil {
		t.Fatal(err)
	}
	if g.State.B != 3 {
		t.Fatalf("upper bounty clamp = %d, want 3", g.State.B)
	}
}

func TestCardComparisonIsStrictAndUsesHeroStats(t *testing.T) {
	blade := game.Card{Suit: game.Blade, Face: 4}
	paper := game.Card{Suit: game.Fang, Face: 3}
	if got := CardValue(blade, paper, 2, 1); got != 13 {
		t.Fatalf("winning suit value = %d, want 13", got)
	}
	if got := CardValue(paper, blade, 2, 1); got != 5 {
		t.Fatalf("non-winning suit value = %d, want 5", got)
	}
	if CardBeatsTop(blade, blade, 0, 0) {
		t.Fatal("equal cards must not win")
	}
}

func TestIllegalHandPlayDoesNotConsumeCardOrTurn(t *testing.T) {
	g := testGame()
	g.State.Piles[0].Zone = -1
	card := game.Card{Suit: game.Blade, Face: 1}
	g.State.Hands[0] = []game.Card{card}
	g.State.Deck = []game.Card{{Suit: game.Fang, Face: 9}}
	before := len(g.State.Piles[0].Cards)
	if g.CanPlayCard(0, 0, 0) || g.Apply(Action{Kind: ActionPlayCard, CardIndex: 0, PileIndex: 0}) == nil {
		t.Fatal("illegal hand play was accepted")
	}
	if len(g.State.Hands[0]) != 1 || g.State.Hands[0][0] != card || len(g.State.Discard) != 0 ||
		len(g.State.Piles[0].Cards) != before || g.State.Piles[0].Zone != -1 || g.State.Current != 0 || len(g.Log) != 0 {
		t.Fatal("illegal hand play mutated the game")
	}
}

func TestFailedDeckFlipStillAppendsCard(t *testing.T) {
	g := testGame()
	g.State.Piles[0].Zone = -1
	g.State.Deck = []game.Card{{Suit: game.Shield, Face: 1}}
	if err := g.Apply(Action{Kind: ActionFlipDeck, PileIndex: 0}); err != nil {
		t.Fatal(err)
	}
	pile := g.State.Piles[0]
	if len(pile.Cards) != 2 || pile.Cards[1] != (game.Card{Suit: game.Shield, Face: 1}) || pile.Zone != -1 {
		t.Fatalf("failed flip result = %+v, want card appended without zone movement", pile)
	}
	if g.Log[0].Succeeded {
		t.Fatal("deck flip should have failed")
	}
}

func TestPushMovesOneZoneAndConsumesOnePush(t *testing.T) {
	g := testGame()
	g.State.Pushes[0] = 1
	g.State.Deck = []game.Card{{Suit: game.Fang, Face: 2}}
	if !g.CanSpendPush(0, 1) {
		t.Fatal("push to neutral pile should be legal")
	}
	if err := g.Apply(Action{Kind: ActionSpendPush, PileIndex: 1}); err != nil {
		t.Fatal(err)
	}
	if g.State.Piles[1].Zone != -1 || g.State.Pushes[0] != 0 {
		t.Fatalf("push state = zone %d, pushes %d", g.State.Piles[1].Zone, g.State.Pushes[0])
	}
	if g.CanSpendPush(0, 1) {
		t.Fatal("cannot push a pile already in the player's zone")
	}
}

func TestSnakeLeadAndRefillAfterBothRoundActions(t *testing.T) {
	g := testGame()
	g.State.Hands[0] = make([]game.Card, 5)
	g.State.Hands[1] = make([]game.Card, 5)
	g.State.Deck = []game.Card{
		{Suit: game.Blade, Face: 1}, {Suit: game.Blade, Face: 2},
		{Suit: game.Blade, Face: 4}, {Suit: game.Blade, Face: 5},
	}
	g.State.Pushes = [2]int{1, 1}
	if err := g.Apply(Action{Kind: ActionSpendPush, PileIndex: 0}); err != nil {
		t.Fatal(err)
	}
	if g.State.Current != 1 || g.State.Round != 1 {
		t.Fatalf("after first action: current %d, round %d", g.State.Current, g.State.Round)
	}
	if err := g.Apply(Action{Kind: ActionSpendPush, PileIndex: 1}); err != nil {
		t.Fatal(err)
	}
	if g.State.Current != 1 || g.State.Round != 2 {
		t.Fatalf("round 2 should lead with player 2: current %d, round %d", g.State.Current, g.State.Round)
	}
	if len(g.State.Hands[0]) != HandSize || len(g.State.Hands[1]) != HandSize ||
		g.Log[1].Refilled != [2]int{1, 1} || len(g.State.Deck) != 2 {
		t.Fatalf("round refill mismatch: hands %d/%d, refill %v, deck %d",
			len(g.State.Hands[0]), len(g.State.Hands[1]), g.Log[1].Refilled, len(g.State.Deck))
	}
}

func TestPassRequiresNoActionAndTwoPassesEndGame(t *testing.T) {
	g := testGame()
	g.State.Deck = []game.Card{{Suit: game.Fang, Face: 1}}
	if g.CanPass() {
		t.Fatal("cannot pass while the deck has cards")
	}
	g.State.Deck = nil
	if !g.CanPass() {
		t.Fatal("should pass when the deck is empty and no hand play wins")
	}
	if err := g.Apply(Action{Kind: ActionPass}); err != nil {
		t.Fatal(err)
	}
	if g.State.Over || g.State.Passes != 1 || g.State.Current != 1 {
		t.Fatalf("first pass state = over:%v passes:%d current:%d", g.State.Over, g.State.Passes, g.State.Current)
	}
	if err := g.Apply(Action{Kind: ActionPass}); err != nil {
		t.Fatal(err)
	}
	if !g.State.Over || len(g.Log) != 2 {
		t.Fatalf("second pass did not end game: over %v, log length %d", g.State.Over, len(g.Log))
	}
}

func TestNonPassResetsPassCount(t *testing.T) {
	g := testGame()
	g.State.Deck = nil
	g.State.Hands[1] = []game.Card{{Suit: game.Blade, Face: 9}}
	if err := g.Apply(Action{Kind: ActionPass}); err != nil {
		t.Fatal(err)
	}
	if err := g.Apply(Action{Kind: ActionPlayCard, CardIndex: 0, PileIndex: 0}); err != nil {
		t.Fatal(err)
	}
	if g.State.Passes != 0 || g.State.Over {
		t.Fatalf("non-pass did not reset the pass count: passes %d, over %v", g.State.Passes, g.State.Over)
	}
}

func TestScoringUsesZoneCardsAsFirstTiebreak(t *testing.T) {
	g := testGame()
	g.State.BountySuit, g.State.B = game.Blade, 1
	g.State.Piles = [NumPiles]Pile{
		{Zone: -1, Cards: []game.Card{{Suit: game.Fang, Face: 1}, {Suit: game.Shield, Face: 2}}},
		{Zone: 1, Cards: []game.Card{{Suit: game.Blade, Face: 3}}},
		{Zone: 0, Cards: []game.Card{{Suit: game.Shield, Face: 1}}},
	}
	result := g.Result()
	if result.Scores != [2]int{2, 2} || result.ZoneCards != [2]int{2, 1} ||
		result.Winner != 0 || result.Tiebreak != "zone-cards" {
		t.Fatalf("unexpected tiebreak result: %+v", result)
	}
	g.State.Piles[0].Zone, g.State.Piles[1].Zone = 0, 0
	result = g.Result()
	if result.Winner != -1 || result.Tiebreak != "draw" {
		t.Fatalf("equal score and card count should draw: %+v", result)
	}
}

func TestCPUUsesPushThenWinningPlayThenFlipPriorities(t *testing.T) {
	g := testGame()
	g.State.Hands[0] = []game.Card{{Suit: game.Blade, Face: 9}}
	g.State.Deck = []game.Card{{Suit: game.Fang, Face: 1}}
	g.State.Pushes[0] = 1
	action := g.ChooseCPUAction()
	if action.Kind != ActionSpendPush {
		t.Fatalf("CPU action = %+v, want push before a winning hand play", action)
	}

	g.State.Pushes[0] = 0
	action = g.ChooseCPUAction()
	if action.Kind != ActionPlayCard {
		t.Fatalf("CPU action = %+v, want winning hand play", action)
	}

	g.State.Hands[0] = nil
	g.State.Piles[0].Cards = []game.Card{{Suit: game.Blade, Face: 3}, {Suit: game.Fang, Face: 4}}
	g.State.Piles[1].Cards = []game.Card{{Suit: game.Fang, Face: 2}}
	action = g.ChooseCPUAction()
	if action.Kind != ActionFlipDeck || action.PileIndex != 0 {
		t.Fatalf("CPU action = %+v, want flip on tallest pile", action)
	}
}

func TestCPUPassesWhenNoDeckOrWinningPlay(t *testing.T) {
	g := testGame()
	g.State.Hands[0] = []game.Card{{Suit: game.Blade, Face: 1}}
	g.State.Deck = nil
	if action := g.ChooseCPUAction(); action.Kind != ActionPass {
		t.Fatalf("CPU action = %+v, want pass", action)
	}
}

func TestCPUActionsRemainLegalUntilGameEnds(t *testing.T) {
	recruit, _ := heroes.ByID("recruit")
	for seed := int64(1); seed <= 10; seed++ {
		g, err := NewGame(recruit, recruit, rand.New(rand.NewSource(seed)))
		if err != nil {
			t.Fatal(err)
		}
		for actions := 0; actions < 200 && !g.State.Over; actions++ {
			action := g.ChooseCPUAction()
			if err := g.Apply(action); err != nil {
				t.Fatalf("seed %d action %d (%+v): %v", seed, actions, action, err)
			}
		}
		if !g.State.Over {
			t.Fatalf("seed %d did not finish in 200 actions", seed)
		}
	}
}
