package app

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/modifiers"
)

func TestPuzzleCatalogHasAllRequestedEntriesAndSizes(t *testing.T) {
	type metadata struct {
		size   int
		par    int
		bounty game.Suit
	}
	expected := map[string]metadata{
		"gp6_balanced": {6, 14, game.Blade}, "gp6_hunters_edge": {6, 16, game.Fang},
		"gp6_bounty_trap": {6, 27, game.Shield}, "gp6_narrow_gate": {6, 17, game.Fang},
		"gp6_bodybuilders_wall": {6, 20, game.Blade},
		"gp8_balanced":          {8, 27, game.Blade}, "gp8_bounty_climb": {8, 29, game.Fang},
		"gp8_forgiving": {8, 20, game.Fang}, "gp8_gamblers_puzzle": {8, 34, game.Shield},
		"gp8_tight":     {8, 34, game.Fang},
		"gp10_balanced": {10, 32, game.Fang}, "gp10_gorilla_threat": {10, 30, game.Fang},
		"gp10_turtle_lead": {10, 36, game.Fang}, "gp10_low_score": {10, 29, game.Fang},
		"gp10_everyone_solves": {10, 36, game.Fang},
		"gp12_balanced":        {12, 52, game.Blade}, "gp12_dual_threat": {12, 54, game.Blade},
		"gp12_shift_dominant": {12, 34, game.Blade}, "gp12_forgiving": {12, 51, game.Blade},
		"gp12_inverted": {12, 38, game.Fang},
	}
	options := puzzleOptions()
	if len(options) != len(expected) {
		t.Fatalf("puzzle options = %d, want %d", len(options), len(expected))
	}
	for _, option := range options {
		want, ok := expected[option.ID]
		if !ok {
			t.Errorf("unexpected puzzle %q", option.ID)
			continue
		}
		if len(option.Cards) != want.size || option.CardCount != want.size {
			t.Errorf("puzzle %q has %d cards, want %d", option.ID, len(option.Cards), want.size)
		}
		if option.Par != want.par || option.Bounty != want.bounty || option.Direction != "high" ||
			option.Ways <= 0 || option.Hint == "" {
			t.Errorf("puzzle %q lost supplied metadata: %+v", option.ID, option)
		}
		delete(expected, option.ID)
	}
	if len(expected) != 0 {
		t.Errorf("missing puzzles: %v", expected)
	}
}

func TestPuzzlePairingCaptureTieAndInclusiveTarget(t *testing.T) {
	hero := heroes.Hero{ID: "test", Bounty: -2, Shift: 1, Favored: 2}
	if got := heroModifiers(hero); got.Bounty != -1 || got.Shift != hero.Shift ||
		got.Favored != hero.Favored || got.Reckoning != modifiers.DefaultReckoning {
		t.Fatalf("puzzle modifiers = %+v, want selected hero modifiers", got)
	}
	if got := heroModifiers(heroes.Hero{ID: "recruit"}).Bounty; got != 1 {
		t.Fatalf("default bounty modifier = %d, want 1", got)
	}
	if got := heroModifiers(heroes.Hero{ID: "high-bounty", Bounty: 3}).Bounty; got != 3 {
		t.Fatalf("high bounty modifier = %d, want capped 3", got)
	}
	definition := puzzleDefinition{
		ID: "test", Cards: []game.Card{{Suit: game.Shield, Face: 2}, {Suit: game.Fang, Face: 2}},
		Bounty: game.Shield, Par: 2, Direction: "high",
	}
	puzzle := newPuzzleSession(definition, heroes.Hero{ID: "recruit"})
	if puzzle.StartedAt.IsZero() || puzzle.webView().StartedAt.IsZero() {
		t.Fatal("new puzzle session omitted its start time")
	}
	if err := puzzle.pick(0); err != nil {
		t.Fatal(err)
	}
	if err := puzzle.pick(1); err != nil {
		t.Fatal(err)
	}
	if len(puzzle.Captured) != 1 || puzzle.Captured[0] != definition.Cards[0] ||
		puzzle.Score != 3 || !puzzle.Won {
		t.Fatalf("pairing result = captured:%v score:%d won:%t", puzzle.Captured, puzzle.Score, puzzle.Won)
	}
	if len(puzzle.Pairings) != 1 || puzzle.Pairings[0].Outcome.Winner != 1 ||
		puzzle.Pairings[0].Outcome.Captured != definition.Cards[0] {
		t.Fatalf("pairing outcome = %+v", puzzle.Pairings)
	}
	puzzleView := puzzle.webView()
	if len(puzzleView.PairResults) != 1 || puzzleView.PairResults[0].Captured == nil ||
		*puzzleView.PairResults[0].Captured != definition.Cards[0] || puzzleView.PairResults[0].Score != 3 {
		t.Fatalf("puzzle pair presentation = %+v, want captured card and +3 score", puzzleView.PairResults)
	}

	tieDefinition := puzzleDefinition{
		ID: "tie", Cards: []game.Card{{Suit: game.Fang, Face: 2}, {Suit: game.Fang, Face: 2}},
		Bounty: game.Blade, Par: 0, Direction: "low",
	}
	tie := newPuzzleSession(tieDefinition, heroes.Hero{ID: "recruit"})
	if err := tie.pick(0); err != nil {
		t.Fatal(err)
	}
	if err := tie.pick(1); err != nil {
		t.Fatal(err)
	}
	if len(tie.Captured) != 0 || tie.Pairings[0].Outcome.Winner != -1 || !tie.Won {
		t.Fatalf("tie captured cards or missed inclusive low target: %+v", tie)
	}
	tieView := tie.webView()
	if len(tieView.PairResults) != 1 || tieView.PairResults[0].Captured != nil ||
		tieView.PairResults[0].Score != 0 {
		t.Fatalf("tie presentation = %+v, want no captured card and zero points", tieView.PairResults)
	}
}

func TestPuzzlePairPresentationShowsIncrementalScore(t *testing.T) {
	first := game.Card{Suit: game.Shield, Face: 2}
	second := game.Card{Suit: game.Shield, Face: 5}
	puzzle := &puzzleSession{
		Definition: puzzleDefinition{
			Cards:  []game.Card{first, {Suit: game.Blade, Face: 1}, second, {Suit: game.Fang, Face: 1}},
			Bounty: game.Blade,
		},
		Modifiers: heroModifiers(heroes.Hero{ID: "recruit"}),
		Used:      []bool{true, true, true, true},
		Pairings: []puzzlePairing{
			{Attacker: first, Defender: game.Card{Suit: game.Blade, Face: 1}, Outcome: game.Outcome{Winner: 1}},
			{Attacker: second, Defender: game.Card{Suit: game.Fang, Face: 1}, Outcome: game.Outcome{Winner: 1}},
		},
		Captured: []game.Card{first, second},
	}
	results := puzzle.webView().PairResults
	if len(results) != 2 || results[0].Score != 2 || results[1].Score != 8 {
		t.Fatalf("pair score deltas = %+v, want +2 then +8", results)
	}
}

func TestAbandonSetupRandomizesCardsAndRevealsOnlyRequiredCards(t *testing.T) {
	hero := heroes.Hero{Draft: 1}
	seenBoards := map[string]bool{}
	for seed := int64(0); seed < 80; seed++ {
		state, err := newAbandonSessionForMission(hero, rand.New(rand.NewSource(seed)), 5, 12, 45)
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Board) != 12 || len(state.PersonalPile) != 5 || state.PairCount != 5 ||
			state.Target != 45 || state.RulesVersion != abandonRulesVersion {
			t.Fatalf("setup sizes/rules = board %d pile %d pairs %d target %d version %d",
				len(state.Board), len(state.PersonalPile), state.PairCount, state.Target, state.RulesVersion)
		}
		seen := map[game.Card]bool{state.Bounty: true}
		suits := [3]bool{}
		for _, card := range state.PersonalPile {
			if seen[card] {
				t.Fatalf("duplicate personal card %s", card)
			}
			seen[card] = true
			suits[card.Suit] = true
		}
		for suit, present := range suits {
			if !present {
				t.Fatalf("personal pile omits suit %d", suit)
			}
		}
		for _, card := range state.Board {
			if seen[card] {
				t.Fatalf("duplicate board/bounty/personal card %s", card)
			}
			seen[card] = true
		}
		highestBounty := -1
		for index, card := range state.Board {
			if card.Suit == state.Bounty.Suit &&
				(highestBounty < 0 || card.Face > state.Board[highestBounty].Face) {
				highestBounty = index
			}
		}
		for index, card := range state.Board {
			if state.Revealed[index] != (card.Suit == state.Bounty.Suit && index != highestBounty) {
				t.Fatalf("board reveal for %s = %t; highest bounty index %d", card, state.Revealed[index], highestBounty)
			}
		}
		for index, visible := range state.PileRevealed {
			if visible != (index < clampInt(3+hero.Draft, 0, 5)) {
				t.Fatalf("personal card %d reveal = %t", index, visible)
			}
		}
		seenBoards[fmt.Sprint(state.Board)] = true
	}
	if len(seenBoards) < 2 {
		t.Fatal("random setup produced the same board for every seed")
	}
}

func TestAbandonFlipThenPairUsesStandardBattleAndPersonalPenalties(t *testing.T) {
	hero := heroes.Hero{ID: "recruit"}
	state := &abandonSession{
		RulesVersion: abandonRulesVersion, PairCount: 1, Target: 3,
		Board: []game.Card{{Suit: game.Blade, Face: 5}, {Suit: game.Shield, Face: 4}},
		PersonalPile: []game.Card{
			{Suit: game.Fang, Face: 3}, {Suit: game.Shield, Face: 1},
			{Suit: game.Blade, Face: 2}, {Suit: game.Fang, Face: 4},
			{Suit: game.Shield, Face: 6},
		},
		Bounty:   game.Card{Suit: game.Blade, Face: 9},
		Revealed: []bool{false, false}, PileRevealed: make([]bool, 5),
		Used: make([]bool, 2), PileUsed: make([]bool, 5), SelectedIndex: -1,
		Pairings: []abandonPairing{}, Captured: []game.Card{}, Penalties: make([]int, 5),
	}
	if err := state.flipBoardCard(0); err != nil {
		t.Fatal(err)
	}
	if err := state.flipBoardCard(0); err == nil {
		t.Fatal("already revealed board card was flipped again")
	}
	if err := state.selectPairCard(abandonBoardSource, 0, hero); err != nil {
		t.Fatal(err)
	}
	if err := state.selectPairCard(abandonBoardSource, 0, hero); err == nil {
		t.Fatal("same card was selected twice for one battle")
	}
	if err := state.selectPairCard(abandonPileSource, 0, hero); err != nil {
		t.Fatal(err)
	}
	if !state.Finished || len(state.Captured) != 1 ||
		state.Captured[0] != (game.Card{Suit: game.Fang, Face: 3}) {
		t.Fatalf("resolved battle = %+v", state)
	}
	mods := heroModifiers(hero)
	wantScore := abandonCapturedScore(state.Captured, state.Bounty.Suit, hero)
	for index, card := range state.PersonalPile {
		if index == 0 {
			if state.Penalties[index] != 0 {
				t.Fatalf("used personal card penalty = %d", state.Penalties[index])
			}
			continue
		}
		wantPenalty := -modifiers.ScoringFace(card.Face, card.Suit == state.Bounty.Suit, mods)
		if state.Penalties[index] != wantPenalty {
			t.Fatalf("personal card %d penalty = %d, want %d", index, state.Penalties[index], wantPenalty)
		}
		wantScore += wantPenalty
	}
	if state.Score != wantScore {
		t.Fatalf("final score = %d, want %d", state.Score, wantScore)
	}
	view := state.webView()
	if view.PersonalPile[1].Penalty == nil || *view.PersonalPile[1].Penalty >= 0 ||
		view.Board[1].Penalty != nil {
		t.Fatalf("final penalties should apply only to unused personal cards: %+v", view)
	}
}

func TestAbandonTieCapturesNoCard(t *testing.T) {
	hero := heroes.Hero{}
	state := &abandonSession{
		RulesVersion: abandonRulesVersion, PairCount: 1, Target: 1,
		Board: []game.Card{{Suit: game.Blade, Face: 4}, {Suit: game.Fang, Face: 8}},
		PersonalPile: []game.Card{
			{Suit: game.Shield, Face: 1}, {Suit: game.Fang, Face: 2},
			{Suit: game.Blade, Face: 3}, {Suit: game.Shield, Face: 4},
			{Suit: game.Fang, Face: 5},
		},
		Bounty:   game.Card{Suit: game.Blade, Face: 9},
		Revealed: []bool{false, false}, PileRevealed: make([]bool, 5),
		Used: make([]bool, 2), PileUsed: make([]bool, 5), SelectedIndex: -1,
		Pairings: []abandonPairing{}, Captured: []game.Card{}, Penalties: make([]int, 5),
	}
	if err := state.flipBoardCard(0); err != nil {
		t.Fatal(err)
	}
	if err := state.selectPairCard(abandonBoardSource, 0, hero); err != nil {
		t.Fatal(err)
	}
	if err := state.selectPairCard(abandonBoardSource, 1, hero); err != nil {
		t.Fatal(err)
	}
	if !state.Finished || !state.Pairings[0].Tie || len(state.Captured) != 0 {
		t.Fatalf("tied pair state = %+v", state)
	}
}

func TestAbandonCapturedScoreUsesStandardSuitCountAndHighestFace(t *testing.T) {
	hero := heroes.Hero{Bounty: 1, Shift: 2}
	captured := []game.Card{
		{Suit: game.Shield, Face: 2}, {Suit: game.Shield, Face: 6},
		{Suit: game.Blade, Face: 3},
	}
	var capturedByRound [3][3]int
	capturedByRound[0][game.Shield] = 2
	capturedByRound[0][game.Blade] = 1
	want := game.ScoreWithModifiers(captured, capturedByRound, game.Blade, heroModifiers(hero)).Total
	if want != 23 {
		t.Fatalf("standard scoring fixture = %d, want 23", want)
	}
	if got := abandonCapturedScore(captured, game.Blade, hero); got != want {
		t.Fatalf("Abandon prisoners score = %d, want standard score %d", got, want)
	}
}

func TestFogRedactionMasksCardFacesAndBattleValues(t *testing.T) {
	attack := game.Card{Suit: game.Fang, Face: 5}
	defense := game.Card{Suit: game.Shield, Face: 8}
	view := webView{
		DraftPool:    []webCard{{Card: game.Card{Suit: game.Blade, Face: 2}, Index: 0}},
		DraftHand:    []webFaceCard{{Card: game.Card{Suit: game.Shield, Face: 5}}},
		DraftPicked:  []webFaceCard{{Card: game.Card{Suit: game.Fang, Face: 8}}},
		Players:      []webPlayer{{}, {Hand: []webCard{{Card: attack}}, Pool: []webCard{{Card: defense}}}},
		BattleAttack: &webFaceCard{Card: attack}, BattleDefend: &webFaceCard{Card: defense},
		BattleOutcome: &webBattleOutcome{AttackerValue: 17, DefenderValue: 8, Winner: 0},
		Battles: []webBattleRecord{{AttackerIndex: 1, Attack: webFaceCard{Card: attack}, Defense: webFaceCard{Card: defense},
			Outcome: webBattleOutcome{AttackerValue: 17, DefenderValue: 8, Winner: 0}}},
	}
	redactFogView(&view, &webSession{attacker: 1, actor: 1, whitePlayer: 0})
	if view.DraftPool[0].Card.Face != 1 || !view.DraftPool[0].FaceHidden || view.DraftPool[0].FaceRange != "1-3" {
		t.Fatalf("draft pool face was not grouped: %+v", view.DraftPool[0])
	}
	if view.Players[1].Hand[0].Card.Face != 4 || !view.Players[1].Hand[0].FaceHidden ||
		view.Players[1].Hand[0].FaceRange != "4-6" {
		t.Fatalf("opponent hand face was not grouped: %+v", view.Players[1].Hand[0])
	}
	if view.BattleAttack.Face != 4 || !view.BattleAttack.FaceHidden || view.BattleAttack.FaceRange != "4-6" {
		t.Fatalf("opponent battle attack was not grouped: %+v", view.BattleAttack)
	}
	if view.BattleDefend.Face != 8 {
		t.Fatal("player's own battle defense was redacted")
	}
	if view.DraftHand[0].Face != 4 || !view.DraftHand[0].FaceHidden || view.DraftHand[0].FaceRange != "4-6" ||
		view.DraftPicked[0].Face != 7 || !view.DraftPicked[0].FaceHidden || view.DraftPicked[0].FaceRange != "7-9" {
		t.Fatal("opponent draft cards were not redacted")
	}
	if view.BattleOutcome.HideValues != true || view.Battles[0].Outcome.HideValues != true ||
		view.Battles[0].Attack.Face != 4 || !view.Battles[0].Attack.FaceHidden || !view.Battles[0].AttackFaceHidden {
		t.Fatalf("battle history did not redact values: %+v", view.Battles[0])
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(encoded)
	if strings.Contains(jsonText, `"attackerValue"`) || strings.Contains(jsonText, `"defenderValue"`) {
		t.Fatalf("fog response leaked battle value fields: %s", jsonText)
	}
}

func TestFogDraftPoolShowsActualFaces(t *testing.T) {
	view := webView{
		DraftPool: []webCard{{Card: game.Card{Suit: game.Blade, Face: 8}, Index: 0}},
	}
	redactFogView(&view, &webSession{phase: "draft"})
	if view.DraftPool[0].Card.Face != 8 || view.DraftPool[0].FaceHidden || view.DraftPool[0].FaceRange != "" {
		t.Fatalf("draft pool face was redacted: %+v", view.DraftPool[0])
	}
}

func TestFogBattleResultRevealsBothCardsAndValues(t *testing.T) {
	attack := game.Card{Suit: game.Fang, Face: 5}
	defense := game.Card{Suit: game.Shield, Face: 8}
	view := webView{
		BattleAttack:  &webFaceCard{Card: attack},
		BattleDefend:  &webFaceCard{Card: defense},
		BattleOutcome: &webBattleOutcome{AttackerValue: 17, DefenderValue: 8, Winner: 0},
	}
	redactFogView(&view, &webSession{phase: "result", attacker: 1, whitePlayer: 0})
	if view.BattleAttack.Face != 5 || view.BattleAttack.FaceHidden ||
		view.BattleDefend.Face != 8 || view.BattleDefend.FaceHidden ||
		view.BattleOutcome.HideValues {
		t.Fatalf("locked battle result did not reveal both cards and values: %+v", view)
	}
	encoded, err := json.Marshal(view.BattleOutcome)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"attackerValue":17`) ||
		!strings.Contains(string(encoded), `"defenderValue":8`) {
		t.Fatalf("fog battle result did not serialize resolved values: %s", encoded)
	}
}

func TestSessionSnapshotRoundTripsVariantsAndDefaultsOldSnapshots(t *testing.T) {
	puzzle, _ := puzzleByID("gp6_balanced")
	testCases := []struct {
		name    string
		session *webSession
	}{
		{"main", &webSession{variant: "main", mode: "easy"}},
		{"fog", &webSession{variant: "fog", mode: "easy"}},
		{"puzzler", &webSession{variant: "puzzler", puzzle: newPuzzleSession(puzzle, heroes.Hero{ID: "recruit"}), mode: "easy"}},
		{"abandon", &webSession{variant: "abandon", abandon: newAbandonSession(heroes.Hero{ID: "recruit"}, rand.New(rand.NewSource(3))), mode: "easy"}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := json.Marshal(snapshotSession(testCase.session))
			if err != nil {
				t.Fatal(err)
			}
			var snapshot sessionSnapshot
			if err := json.Unmarshal(encoded, &snapshot); err != nil {
				t.Fatal(err)
			}
			restored := snapshot.restore()
			if restored.variant != testCase.session.variant {
				t.Fatalf("restored variant = %q", restored.variant)
			}
			if testCase.session.puzzle != nil && (restored.puzzle == nil || restored.puzzle.Definition.ID != puzzle.ID) {
				t.Fatalf("puzzler state was not restored: %+v", restored.puzzle)
			}
			if testCase.session.abandon != nil && (restored.abandon == nil ||
				len(restored.abandon.Board) != len(testCase.session.abandon.Board) ||
				len(restored.abandon.PersonalPile) != len(testCase.session.abandon.PersonalPile) ||
				restored.abandon.PairCount != testCase.session.abandon.PairCount ||
				restored.abandon.Target != testCase.session.abandon.Target ||
				restored.abandon.RulesVersion != abandonRulesVersion ||
				restored.abandon.Bounty != testCase.session.abandon.Bounty) {
				t.Fatalf("abandon state was not restored: %+v", restored.abandon)
			}
		})
	}
	if got := (&sessionSnapshot{}).restore().variant; got != "main" {
		t.Fatalf("old snapshot variant = %q, want main", got)
	}
	legacy := (&sessionSnapshot{
		Variant: "abandon", Abandon: &abandonSession{Board: []game.Card{{Suit: game.Shield, Face: 2}}},
		Campaign: &webCampaignContext{ID: "abandon", Index: 10},
	}).restore()
	if legacy.abandon == nil || legacy.abandon.RulesVersion != abandonRulesVersion ||
		legacy.abandon.PairCount != 4 || legacy.abandon.Target != 35 ||
		len(legacy.abandon.Board) != 11 || legacy.phase != "abandon-reveal" ||
		legacy.campaign.Index != 1 || legacy.message == "" {
		t.Fatalf("legacy Abandon snapshot was not safely restarted: %+v", legacy)
	}
}

func TestMainVariantRetainsUnmaskedNormalGameView(t *testing.T) {
	state, err := game.NewGameWithModifiers(rand.New(rand.NewSource(5)), game.GameModifiers{Reckoning: modifiers.DefaultReckoning})
	if err != nil {
		t.Fatal(err)
	}
	web := &webServer{rng: rand.New(rand.NewSource(5)), defaultMode: "easy"}
	if err := web.startWebGameSession(state, "easy", "recruit", "recruit"); err != nil {
		t.Fatal(err)
	}
	view := web.view()
	if view.Variant != "main" || len(view.Players) != 2 || len(view.Players[0].Hand) != 5 {
		t.Fatalf("default variant changed normal view: %+v", view)
	}
	for _, card := range view.Players[0].Hand {
		if card.FaceHidden || card.Card.Face < 1 || card.Card.Face > 9 {
			t.Fatalf("main variant unexpectedly masked a normal card: %+v", card)
		}
	}
}
