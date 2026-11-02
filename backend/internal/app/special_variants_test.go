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

func TestAbandonInitializationRevealTokensAndDrawDeck(t *testing.T) {
	hero, _ := heroes.ByID("recruit")
	state := newAbandonSession(hero, rand.New(rand.NewSource(12)))
	if state.DrawTokens != clampInt(2+hero.Draft, 0, 4) {
		t.Fatalf("draw tokens = %d", state.DrawTokens)
	}
	revealed := 0
	for _, visible := range state.Revealed {
		if visible {
			revealed++
		}
	}
	lowestBountyBoardIndex := -1
	for index, card := range state.Board {
		if card.Suit == state.Bounty.Suit &&
			(lowestBountyBoardIndex < 0 || card.Face < state.Board[lowestBountyBoardIndex].Face) {
			lowestBountyBoardIndex = index
		}
	}
	if revealed != 3 || lowestBountyBoardIndex < 0 || !state.Revealed[lowestBountyBoardIndex] {
		t.Fatalf("revealed cards=%d board=%v", revealed, state.Revealed)
	}
	highestRevealedBountyFace := 0
	for index, visible := range state.Revealed {
		if !visible {
			continue
		}
		if state.Board[index].Suit != state.Bounty.Suit {
			t.Fatalf("initially revealed non-bounty card %s", state.Board[index])
		}
		if state.Board[index].Face > highestRevealedBountyFace {
			highestRevealedBountyFace = state.Board[index].Face
		}
	}
	for index, card := range state.Board {
		if card.Suit == state.Bounty.Suit && !state.Revealed[index] && card.Face < highestRevealedBountyFace {
			t.Fatalf("revealed cards were not the three lowest bounty cards: %v", state.Revealed)
		}
	}
	if len(state.Board) != 9 || len(state.DrawPile) != 17 {
		t.Fatalf("board/draw pile sizes = %d/%d, want 9/17", len(state.Board), len(state.DrawPile))
	}
	seen := map[game.Card]bool{state.Bounty: true}
	for _, card := range state.Board {
		seen[card] = true
	}
	for _, card := range state.DrawPile {
		if seen[card] {
			t.Fatalf("draw pile contains excluded or duplicate card %s", card)
		}
		seen[card] = true
	}
	if len(seen) != 27 {
		t.Fatalf("board, bounty, and draw pile account for %d cards", len(seen))
	}

	if err := state.selectCard(8); err != nil {
		t.Fatal(err)
	}
	if !state.Revealed[8] {
		t.Fatal("selected hidden board card did not become visible")
	}
}

func TestAbandonScoringTokensAndRemainingBoardPenalty(t *testing.T) {
	hero := heroes.Hero{Bounty: 3, Shift: 1, Draft: 1}
	if got := scoreAbandonPair(game.Card{Suit: game.Blade, Face: 1}, game.Card{Suit: game.Shield, Face: 1}, hero); got != 42 {
		t.Fatalf("equal-face bounty pair = %d, want 42", got)
	}
	if got := scoreAbandonPair(game.Card{Suit: game.Blade, Face: 1}, game.Card{Suit: game.Shield, Face: 2}, hero); got != 3 {
		t.Fatalf("non-matching pair = %d, want 3", got)
	}
	if got := abandonValue(game.Card{Suit: game.Blade, Face: 1}, heroes.Hero{Bounty: -3, Shift: 0}); got != -1 {
		t.Fatalf("signed abandoned-card value = %d, want -1", got)
	}

	state := newAbandonSession(hero, rand.New(rand.NewSource(7)))
	if state.DrawTokens != 3 {
		t.Fatalf("draw tokens = %d, want 3", state.DrawTokens)
	}
	if err := state.selectCard(0); err != nil {
		t.Fatal(err)
	}
	if err := state.pairDraw(hero); err != nil {
		t.Fatal(err)
	}
	if state.DrawTokens != 2 || len(state.Pairings) != 1 {
		t.Fatalf("after draw pairing tokens=%d pairings=%d", state.DrawTokens, len(state.Pairings))
	}
	if err := state.pairDraw(hero); err == nil {
		t.Fatal("draw pairing succeeded without selecting a board card")
	}

	state = newAbandonSession(heroes.Hero{}, rand.New(rand.NewSource(7)))
	expected := 0
	for pairing := 0; pairing < 4; pairing++ {
		first, second := pairing*2, pairing*2+1
		expected += scoreAbandonPair(state.Board[first], state.Board[second], heroes.Hero{})
		if err := state.selectCard(first); err != nil {
			t.Fatal(err)
		}
		if err := state.pairBoard(second, heroes.Hero{}); err != nil {
			t.Fatal(err)
		}
	}
	expected -= abandonValue(state.Board[8], heroes.Hero{})
	if !state.Finished || state.Score != expected {
		t.Fatalf("finished=%t score=%d, want %d", state.Finished, state.Score, expected)
	}
	view := state.webView()
	if !view.Board[8].Revealed || view.Board[8].Penalty == nil ||
		*view.Board[8].Penalty != -abandonValue(state.Board[8], heroes.Hero{}) {
		t.Fatalf("final abandoned card details = %+v, want revealed card and its score penalty", view.Board[8])
	}
}

func TestAbandonSuitMappingCanSwapAndRotateBeforePlay(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	state := newAbandonSession(heroes.Hero{}, rng)
	initial := state.CategorySuits
	if err := state.assignCategorySuit(0, initial[2], rng); err != nil {
		t.Fatal(err)
	}
	if state.CategorySuits != [3]game.Suit{initial[2], initial[1], initial[0]} ||
		state.Bounty.Suit != state.CategorySuits[0] {
		t.Fatalf("swapped categories=%v bounty=%s", state.CategorySuits, state.Bounty)
	}
	if err := state.rotateCategorySuits(rng); err != nil {
		t.Fatal(err)
	}
	if state.CategorySuits != [3]game.Suit{initial[1], initial[0], initial[2]} ||
		state.Bounty.Suit != state.CategorySuits[0] {
		t.Fatalf("rotated categories=%v bounty=%s", state.CategorySuits, state.Bounty)
	}
	seen := map[game.Card]bool{state.Bounty: true}
	for _, card := range append(append([]game.Card(nil), state.Board...), state.DrawPile...) {
		if seen[card] {
			t.Fatalf("suit remap produced duplicate card %s", card)
		}
		seen[card] = true
	}
	if len(seen) != 27 {
		t.Fatalf("remapped cards account for %d cards, want 27", len(seen))
	}
	if state.webView().CanChangeSuits != true {
		t.Fatal("suit mapping is unavailable before play")
	}
	if err := state.selectCard(0); err != nil {
		t.Fatal(err)
	}
	if err := state.rotateCategorySuits(rng); err == nil {
		t.Fatal("suit rotation succeeded after selecting a card")
	}
}

func TestAbandonPairDisplayChoosesBountySuitForPairs(t *testing.T) {
	first := game.Card{Suit: game.Fang, Face: 5}
	bounty := game.Card{Suit: game.Blade, Face: 5}
	if got, pair := abandonPairDisplayCard(first, bounty, game.Blade, 0); got != bounty || !pair {
		t.Fatalf("bounty pair display = %s, pair=%t; want bounty card %s", got, pair, bounty)
	}
	lower := game.Card{Suit: game.Fang, Face: 3}
	higher := game.Card{Suit: game.Shield, Face: 7}
	if got, pair := abandonPairDisplayCard(lower, higher, game.Blade, 0); got != higher || pair {
		t.Fatalf("non-pair display = %s, pair=%t; want high card %s", got, pair, higher)
	}
	if got, pair := abandonPairDisplayCard(
		game.Card{Suit: game.Fang, Face: 1}, game.Card{Suit: game.Blade, Face: 2}, game.Blade, -8,
	); got.Suit != game.Blade || !pair {
		t.Fatalf("shifted equal-value pair display = %s, pair=%t; want bounty suit pair", got, pair)
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

func TestAbandonFirstSelectionPreservesBountySuit(t *testing.T) {
	hero, ok := heroes.ByID("recruit")
	if !ok {
		t.Fatal("recruit hero is unavailable")
	}
	sawMatchingSuit := false
	sawDifferentSuit := false
	for seed := int64(1); seed <= 8; seed++ {
		initial, err := newAbandonSessionForBoard(hero, rand.New(rand.NewSource(seed)), "board-01")
		if err != nil {
			t.Fatal(err)
		}
		for index, revealed := range initial.Revealed {
			if revealed {
				continue
			}
			abandon, err := newAbandonSessionForBoard(hero, rand.New(rand.NewSource(seed)), "board-01")
			if err != nil {
				t.Fatal(err)
			}
			bounty, categorySuits := abandon.Bounty, abandon.CategorySuits
			matchesBounty := abandon.Board[index].Suit == bounty.Suit
			if err := abandon.selectCard(index); err != nil {
				t.Fatalf("select face-down card %d: %v", index, err)
			}
			if abandon.Bounty != bounty || abandon.CategorySuits != categorySuits {
				t.Fatalf("selecting card %d changed bounty or suit mapping: bounty=%+v categories=%v",
					index, abandon.Bounty, abandon.CategorySuits)
			}
			sawMatchingSuit = sawMatchingSuit || matchesBounty
			sawDifferentSuit = sawDifferentSuit || !matchesBounty
		}
	}
	if !sawMatchingSuit || !sawDifferentSuit {
		t.Fatalf("test did not cover both bounty-suit cases: match=%t different=%t",
			sawMatchingSuit, sawDifferentSuit)
	}
}

func TestAbandonBoardCatalogLayoutsAndTargets(t *testing.T) {
	expected := [][]abandonBoardCard{
		{{1, 1}, {1, 2}, {9, 0}, {9, 1}, {8, 0}, {4, 0}, {3, 0}, {6, 1}, {2, 1}},
		{{4, 1}, {4, 2}, {7, 0}, {7, 1}, {2, 0}, {3, 0}, {5, 0}, {8, 1}, {1, 1}},
		{{8, 1}, {8, 2}, {9, 0}, {9, 1}, {7, 0}, {5, 0}, {2, 0}, {3, 1}, {4, 1}},
		{{1, 1}, {1, 2}, {3, 0}, {3, 1}, {4, 0}, {7, 0}, {2, 0}, {6, 1}, {8, 1}},
		{{8, 1}, {8, 2}, {1, 0}, {1, 1}, {7, 0}, {5, 0}, {6, 0}, {2, 1}, {9, 1}},
		{{4, 1}, {4, 2}, {9, 0}, {9, 1}, {2, 0}, {6, 0}, {5, 0}, {8, 1}, {1, 1}},
		{{7, 1}, {7, 2}, {2, 0}, {2, 1}, {5, 0}, {6, 0}, {8, 0}, {4, 1}, {3, 1}},
		{{8, 1}, {8, 2}, {2, 0}, {2, 1}, {6, 0}, {7, 0}, {9, 0}, {5, 1}, {4, 1}},
		{{3, 1}, {3, 2}, {1, 0}, {1, 1}, {2, 0}, {8, 0}, {9, 0}, {5, 1}, {7, 1}},
		{{6, 1}, {6, 2}, {9, 0}, {9, 1}, {7, 0}, {3, 0}, {1, 0}, {2, 1}, {4, 1}},
		{{7, 1}, {7, 2}, {2, 0}, {2, 1}, {1, 0}, {5, 0}, {8, 0}, {9, 1}, {3, 1}},
		{{7, 1}, {7, 2}, {9, 0}, {9, 1}, {2, 0}, {1, 0}, {6, 0}, {8, 1}, {3, 1}},
		{{7, 1}, {7, 2}, {9, 0}, {9, 1}, {1, 0}, {2, 0}, {4, 0}, {3, 1}, {8, 1}},
		{{4, 1}, {4, 2}, {5, 0}, {5, 1}, {1, 0}, {8, 0}, {7, 0}, {2, 1}, {9, 1}},
		{{7, 1}, {7, 2}, {4, 0}, {4, 1}, {9, 0}, {1, 0}, {8, 0}, {2, 1}, {5, 1}},
	}
	targets := []int{54, 58, 60, 61, 62, 63, 64, 65, 66, 67, 68, 70, 72, 75, 76}
	if len(abandonBoardCatalog) != 15 || len(abandonBoardOptions()) != 15 {
		t.Fatalf("catalog/options sizes = %d/%d, want 15", len(abandonBoardCatalog), len(abandonBoardOptions()))
	}
	for index, board := range abandonBoardCatalog {
		if board.ID != fmt.Sprintf("board-%02d", index+1) ||
			board.Name != fmt.Sprintf("Abandon Board %d", index+1) ||
			board.Target != targets[index] || len(board.Cards) != 9 {
			t.Fatalf("board %d metadata = %+v", index+1, board.abandonBoardOption)
		}
		for cardIndex, card := range board.Cards {
			if card != expected[index][cardIndex] {
				t.Fatalf("board %d card %d = %+v, want %+v", index+1, cardIndex, card, expected[index][cardIndex])
			}
		}
	}
}

func TestAbandonRandomBountySuitMappingAndBoardSelection(t *testing.T) {
	seenBountySuits := map[game.Suit]bool{}
	for seed := int64(0); seed < 100; seed++ {
		state, err := newAbandonSessionForBoard(heroes.Hero{}, rand.New(rand.NewSource(seed)), "board-08")
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Board) != 9 || len(state.DrawPile) != 17 || state.BoardID != "board-08" ||
			state.Target != 65 || state.Bounty.Suit != state.CategorySuits[0] {
			t.Fatalf("selected board state = %+v", state)
		}
		seenBountySuits[state.Bounty.Suit] = true
		for category := 0; category < 3; category++ {
			want := game.Suit((int(state.CategorySuits[0]) + category) % 3)
			if state.CategorySuits[category] != want {
				t.Fatalf("category %d maps to %s, want %s", category, state.CategorySuits[category], want)
			}
		}
		used := map[game.Card]bool{}
		lowestBountyBoardFace := 10
		for _, card := range state.Board {
			if used[card] {
				t.Fatalf("duplicate board card %s", card)
			}
			used[card] = true
			if card.Suit == state.Bounty.Suit && card.Face < lowestBountyBoardFace {
				lowestBountyBoardFace = card.Face
			}
		}
		if used[state.Bounty] || state.Bounty.Face < 1 || state.Bounty.Face > 9 {
			t.Fatalf("bounty card %s is invalid or present on board", state.Bounty)
		}
		for face := 1; face < state.Bounty.Face; face++ {
			if !used[game.Card{Suit: state.Bounty.Suit, Face: face}] {
				t.Fatalf("bounty %s is not the lowest available face", state.Bounty)
			}
		}
		if lowestBountyBoardFace == 10 {
			t.Fatal("board has no bounty-suited card")
		}
	}
	if len(seenBountySuits) != 3 {
		t.Fatalf("deterministic RNG seeds yielded bounty suits %v, want all three", seenBountySuits)
	}
	if _, err := newAbandonSessionForBoard(heroes.Hero{}, rand.New(rand.NewSource(1)), "missing"); err == nil {
		t.Fatal("unknown board ID was accepted")
	}
}

func TestAbandonTargetEvaluationIsAtLeast(t *testing.T) {
	for _, testCase := range []struct {
		score int
		won   bool
	}{{54, true}, {53, false}} {
		state := newAbandonSession(heroes.Hero{}, rand.New(rand.NewSource(2)))
		state.Score = testCase.score
		state.Used = make([]bool, len(state.Board))
		for index := range state.Used {
			state.Used[index] = true
		}
		state.Pairings = make([]abandonPairing, 4)
		state.finishIfReady(heroes.Hero{})
		if !state.Evaluated || state.Won != testCase.won {
			t.Fatalf("score %d evaluated=%t won=%t, want won=%t", testCase.score, state.Evaluated, state.Won, testCase.won)
		}
		wantEvaluation := "loss"
		if testCase.won {
			wantEvaluation = "win"
		}
		if got := state.webView().Evaluation; got != wantEvaluation {
			t.Fatalf("score %d evaluation=%q, want %q", testCase.score, got, wantEvaluation)
		}
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
				len(restored.abandon.DrawPile) != len(testCase.session.abandon.DrawPile) ||
				restored.abandon.DrawTokens != testCase.session.abandon.DrawTokens ||
				restored.abandon.BoardID != testCase.session.abandon.BoardID ||
				restored.abandon.Target != testCase.session.abandon.Target ||
				restored.abandon.CategorySuits != testCase.session.abandon.CategorySuits ||
				restored.abandon.Bounty != testCase.session.abandon.Bounty) {
				t.Fatalf("abandon state was not restored: %+v", restored.abandon)
			}
		})
	}
	if got := (&sessionSnapshot{}).restore().variant; got != "main" {
		t.Fatalf("old snapshot variant = %q, want main", got)
	}
	legacyBoard := []game.Card{{Suit: game.Shield, Face: 2}, {Suit: game.Fang, Face: 4}}
	legacy := (&sessionSnapshot{Abandon: &abandonSession{Board: legacyBoard}}).restore().abandon
	if legacy.BoardID != "board-01" || legacy.Target != 54 ||
		legacy.Bounty != (game.Card{Suit: game.Blade, Face: 9}) ||
		legacy.CategorySuits != [3]game.Suit{game.Blade, game.Fang, game.Shield} ||
		len(legacy.Board) != len(legacyBoard) || legacy.Board[0] != legacyBoard[0] {
		t.Fatalf("legacy abandon snapshot did not default/preserve state: %+v", legacy)
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
