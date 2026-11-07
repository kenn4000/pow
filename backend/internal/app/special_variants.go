package app

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/modifiers"
)

type puzzleDefinition struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Name      string      `json:"name"`
	Category  string      `json:"category"`
	Cards     []game.Card `json:"cards"`
	CardCount int         `json:"cardCount"`
	Bounty    game.Suit   `json:"bounty"`
	Par       int         `json:"par"`
	Target    int         `json:"target"`
	Direction string      `json:"direction"`
	Solver    string      `json:"solver"`
	Ways      int         `json:"ways"`
	Hint      string      `json:"hint"`
}

var puzzleCatalog = []puzzleDefinition{
	{ID: "gp6_balanced", Title: "The Even Hand", Category: "bounty_up", Cards: puzzleCards([]int{2, 3}, []int{4, 9}, []int{3, 8}), Bounty: game.Blade, Par: 14, Direction: "high", Solver: "Turtle", Ways: 2, Hint: "Every free hero has room here — find the pairing that stacks Scissors captures."},
	{ID: "gp6_hunters_edge", Title: "Hunter's Edge", Category: "cycle_exploit", Cards: puzzleCards([]int{3, 4}, []int{2, 6}, []int{2, 7}), Bounty: game.Fang, Par: 16, Direction: "high", Solver: "Recruit", Ways: 3, Hint: "Pair mid Scissors against low Paper to double your way past the target."},
	{ID: "gp6_bounty_trap", Title: "The Bounty Trap", Category: "bounty_avoidance", Cards: puzzleCards([]int{5, 8}, []int{1, 9}, []int{5, 9}), Bounty: game.Shield, Par: 27, Direction: "high", Solver: "Recruit", Ways: 2, Hint: "The bounty hero has it hardest here — exploit that BodyBuilder can't keep up."},
	{ID: "gp6_narrow_gate", Title: "The Narrow Gate", Category: "precise_pars", Cards: puzzleCards([]int{2, 5}, []int{4, 6}, []int{2, 7}), Bounty: game.Fang, Par: 17, Direction: "high", Solver: "Recruit", Ways: 1, Hint: "Only one or two pairings reach the target — watch the tie rules carefully."},
	{ID: "gp6_bodybuilders_wall", Title: "The Bodybuilder's Wall", Category: "shift_inflation", Cards: puzzleCards([]int{3, 8}, []int{3, 5}, []int{4, 5}), Bounty: game.Blade, Par: 20, Direction: "high", Solver: "Recruit", Ways: 1, Hint: "The bounty multiplier makes Bodybuilder nearly unbeatable — find the one path that slips past him."},
	{ID: "gp8_balanced", Title: "The Balanced Blade", Category: "multi_suit", Cards: puzzleCards([]int{2, 5}, []int{1, 3, 9}, []int{3, 5, 7}), Bounty: game.Blade, Par: 27, Direction: "high", Solver: "Turtle", Ways: 6, Hint: "Gorilla is the threat, not Bodybuilder — shift tricks spike when you least expect them."},
	{ID: "gp8_bounty_climb", Title: "The Bounty Climb", Category: "bounty_up", Cards: puzzleCards([]int{6, 8, 9}, []int{5, 9}, []int{1, 4, 6}), Bounty: game.Fang, Par: 29, Direction: "high", Solver: "Turtle", Ways: 2, Hint: "Stack Paper captures — Bodybuilder will still outscore you if you slip."},
	{ID: "gp8_forgiving", Title: "The Open Field", Category: "multi_suit", Cards: puzzleCards([]int{3, 4, 5}, []int{1, 9}, []int{1, 3, 5}), Bounty: game.Fang, Par: 20, Direction: "high", Solver: "Recruit", Ways: 15, Hint: "For a warm-up: every free hero has room, and the target is forgiving."},
	{ID: "gp8_gamblers_puzzle", Title: "The Gambler's Puzzle", Category: "cycle_exploit", Cards: puzzleCards([]int{1, 3, 8}, []int{2, 4, 5}, []int{5, 8}), Bounty: game.Shield, Par: 34, Direction: "high", Solver: "Warden", Ways: 10, Hint: "Gambler's shift+bounty combo is the real threat — stack Rocks with care."},
	{ID: "gp8_tight", Title: "The Tight Squeeze", Category: "precise_pars", Cards: puzzleCards([]int{6, 8}, []int{4, 5, 8}, []int{1, 6, 8}), Bounty: game.Fang, Par: 34, Direction: "high", Solver: "Warden", Ways: 4, Hint: "Bodybuilder is one point from the cap — every capture counts."},
	{ID: "gp10_balanced", Title: "The Grand Balance", Category: "multi_suit", Cards: puzzleCards([]int{2, 3, 5, 7}, []int{5, 7, 8}, []int{1, 2, 8}), Bounty: game.Fang, Par: 32, Direction: "high", Solver: "Turtle", Ways: 118, Hint: "All four free heroes have wide room; Bodybuilder is near his limit at 167."},
	{ID: "gp10_gorilla_threat", Title: "The Silent Shift", Category: "shift_inflation", Cards: puzzleCards([]int{2, 3, 5}, []int{1, 2, 3, 9}, []int{3, 4, 5}), Bounty: game.Fang, Par: 30, Direction: "high", Solver: "Recruit", Ways: 83, Hint: "Gorilla's +2 shift is the real boss here — Bodybuilder is far behind."},
	{ID: "gp10_turtle_lead", Title: "Turtle's Long Game", Category: "bounty_avoidance", Cards: puzzleCards([]int{2, 4, 7}, []int{1, 7, 8}, []int{1, 2, 5, 8}), Bounty: game.Fang, Par: 36, Direction: "high", Solver: "Turtle", Ways: 22, Hint: "Two threats (Gambler and Bodybuilder) — Turtle's low ways make this a sniper puzzle."},
	{ID: "gp10_low_score", Title: "The Low Road", Category: "bounty_down", Cards: puzzleCards([]int{3, 4, 5, 7}, []int{2, 4, 5}, []int{2, 3, 4}), Bounty: game.Fang, Par: 29, Direction: "high", Solver: "Recruit", Ways: 12, Hint: "Low faces everywhere — the target is reachable only through careful stacking."},
	{ID: "gp10_everyone_solves", Title: "The Open Court", Category: "multi_suit", Cards: puzzleCards([]int{2, 5, 7, 9}, []int{3, 4, 6}, []int{1, 3, 9}), Bounty: game.Fang, Par: 36, Direction: "high", Solver: "Turtle", Ways: 34, Hint: "Everyone is close to the cap — no single hero dominates, so play to the threat you can beat."},
	{ID: "gp12_balanced", Title: "The Grand Trial", Category: "multi_suit", Cards: puzzleCards([]int{3, 5, 6, 9}, []int{4, 5, 6, 8}, []int{2, 4, 6, 9}), Bounty: game.Blade, Par: 52, Direction: "high", Solver: "Turtle", Ways: 33, Hint: "Bodybuilder is near the cap — every free hero has wide room, but S-stacking is dangerous."},
	{ID: "gp12_dual_threat", Title: "The Twin Shadows", Category: "bounty_up", Cards: puzzleCards([]int{4, 6, 8, 9}, []int{4, 5, 6, 8}, []int{1, 5, 7, 8}), Bounty: game.Blade, Par: 54, Direction: "high", Solver: "Recruit", Ways: 30, Hint: "Two non-free threats (Bodybuilder and Gambler) both near cap — narrow window."},
	{ID: "gp12_shift_dominant", Title: "The Shift Dominance", Category: "shift_inflation", Cards: puzzleCards([]int{1, 3, 4, 7}, []int{1, 2, 3, 5}, []int{3, 5, 7, 8}), Bounty: game.Blade, Par: 34, Direction: "high", Solver: "Turtle", Ways: 45, Hint: "Gorilla and Gambler are both near cap — the shift heroes rule this board."},
	{ID: "gp12_forgiving", Title: "The Long Game", Category: "multi_suit", Cards: puzzleCards([]int{1, 4, 5, 8}, []int{2, 6, 7, 9}, []int{5, 7, 8, 9}), Bounty: game.Blade, Par: 51, Direction: "high", Solver: "Turtle", Ways: 88, Hint: "Forgiving puzzle — every free hero has wide room. Good for introducing 12-card play."},
	{ID: "gp12_inverted", Title: "The Inverted Crown", Category: "bounty_down", Cards: puzzleCards([]int{1, 2, 5, 9}, []int{1, 3, 4, 5}, []int{3, 5, 6, 7}), Bounty: game.Fang, Par: 38, Direction: "high", Solver: "Recruit", Ways: 796, Hint: "Bodybuilder is NOT the top threat — Gorilla and Gambler edge him out. Unusual board."},
}

type puzzleSession struct {
	Definition puzzleDefinition
	Modifiers  game.GameModifiers
	StartedAt  time.Time
	DrawPile   []game.Card
	Discard    []game.Card
	Selected   int
	Used       []bool
	Pairings   []puzzlePairing
	Captured   []game.Card
	Score      int
	Won        bool
}

type puzzlePairing struct {
	Attacker game.Card
	Defender game.Card
	Outcome  game.Outcome
}

type webPuzzleState struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Cards         []webPuzzleCard    `json:"cards"`
	BountySuit    game.Suit          `json:"bountySuit"`
	Par           int                `json:"par"`
	Target        int                `json:"target"`
	Direction     string             `json:"direction"`
	Hint          string             `json:"hint"`
	SelectedIndex int                `json:"selectedIndex"`
	PairResults   []webPuzzlePair    `json:"pairResults"`
	Captured      []game.Card        `json:"captured"`
	Modifiers     game.GameModifiers `json:"modifiers"`
	Score         int                `json:"score"`
	Complete      bool               `json:"complete"`
	Won           bool               `json:"won"`
	StartedAt     time.Time          `json:"startedAt"`
}

type webPuzzleCard struct {
	game.Card
	Index      int  `json:"index"`
	Used       bool `json:"used"`
	Selectable bool `json:"selectable"`
}

type webPuzzlePair struct {
	Cards    []game.Card `json:"cards"`
	Captured *game.Card  `json:"captured,omitempty"`
	Score    int         `json:"score"`
	Result   string      `json:"result"`
}

type abandonSession struct {
	RulesVersion   int
	Board          []game.Card
	PersonalPile   []game.Card
	DrawPile       []game.Card
	Discard        []game.Card
	Target         int
	PairCount      int
	Bounty         game.Card
	Revealed       []bool
	PileRevealed   []bool
	Used           []bool
	PileUsed       []bool
	SelectedSource string
	SelectedIndex  int
	Pairings       []abandonPairing
	Captured       []game.Card
	Score          int
	Finished       bool
	Evaluated      bool
	Won            bool
	Penalties      []int
}

type abandonPairing struct {
	First    game.Card  `json:"first"`
	Second   game.Card  `json:"second"`
	Captured *game.Card `json:"captured,omitempty"`
	Tie      bool       `json:"tie"`
}

type webAbandonCard struct {
	Index    int        `json:"index"`
	Card     *game.Card `json:"card,omitempty"`
	Revealed bool       `json:"revealed"`
	Used     bool       `json:"used"`
	Penalty  *int       `json:"penalty,omitempty"`
}

type webAbandonState struct {
	Target         int                    `json:"target"`
	PairCount      int                    `json:"pairCount"`
	Evaluation     string                 `json:"evaluation"`
	Won            bool                   `json:"won"`
	Bounty         game.Card              `json:"bounty"`
	Board          []webAbandonCard       `json:"board"`
	PersonalPile   []webAbandonCard       `json:"personalPile"`
	PairResults    []webAbandonPairResult `json:"pairResults"`
	SelectedSource string                 `json:"selectedSource"`
	SelectedIndex  int                    `json:"selectedIndex"`
	Score          int                    `json:"score"`
	Complete       bool                   `json:"complete"`
}

type webAbandonPairResult struct {
	Captured *game.Card `json:"captured,omitempty"`
	Tie      bool       `json:"tie"`
}

const abandonRulesVersion = 2
const abandonBoardSource = "board"
const abandonPileSource = "personal-pile"

func puzzleCards(rocks, papers, scissors []int) []game.Card {
	cards := make([]game.Card, 0, len(rocks)+len(papers)+len(scissors))
	for _, face := range rocks {
		cards = append(cards, game.Card{Suit: game.Shield, Face: face})
	}
	for _, face := range papers {
		cards = append(cards, game.Card{Suit: game.Fang, Face: face})
	}
	for _, face := range scissors {
		cards = append(cards, game.Card{Suit: game.Blade, Face: face})
	}
	return cards
}

func puzzleByID(id string) (puzzleDefinition, bool) {
	for _, definition := range puzzleCatalog {
		if definition.ID == id {
			return copyPuzzleDefinition(definition), true
		}
	}
	return puzzleDefinition{}, false
}

func puzzleOptions() []puzzleDefinition {
	options := make([]puzzleDefinition, len(puzzleCatalog))
	for i, definition := range puzzleCatalog {
		options[i] = copyPuzzleDefinition(definition)
	}
	return options
}

func copyPuzzleDefinition(definition puzzleDefinition) puzzleDefinition {
	definition.Cards = append([]game.Card(nil), definition.Cards...)
	definition.Name = definition.Title
	definition.CardCount = len(definition.Cards)
	definition.Target = definition.Par
	return definition
}

func heroModifiers(hero heroes.Hero) game.GameModifiers {
	return game.GameModifiers{
		Bounty: clampInt(1+hero.Bounty, -3, 3), Shift: hero.Shift,
		Favored: hero.Favored, Reckoning: modifiers.DefaultReckoning,
	}
}

func newPuzzleSession(definition puzzleDefinition, hero heroes.Hero) *puzzleSession {
	return &puzzleSession{
		Definition: definition, Modifiers: heroModifiers(hero), StartedAt: time.Now(), Selected: -1,
		Used: make([]bool, len(definition.Cards)), Pairings: []puzzlePairing{},
		Captured: []game.Card{},
	}
}

func (p *puzzleSession) pick(index int) error {
	if p == nil || p.Won || len(p.Pairings) == len(p.Definition.Cards)/2 {
		return errors.New("puzzle is complete")
	}
	if index < 0 || index >= len(p.Used) || p.Used[index] {
		return errors.New("selected puzzle card is unavailable")
	}
	if p.Selected < 0 {
		p.Selected = index
		return nil
	}
	if p.Selected == index {
		return errors.New("choose a different card for the second pick")
	}
	attacker, defender := p.Definition.Cards[p.Selected], p.Definition.Cards[index]
	outcome := game.ResolveWithModifiers(attacker, defender, p.Modifiers)
	if outcome.Winner == 0 {
		p.Captured = append(p.Captured, defender)
	} else if outcome.Winner == 1 {
		p.Captured = append(p.Captured, attacker)
	}
	p.Used[p.Selected], p.Used[index] = true, true
	p.Pairings = append(p.Pairings, puzzlePairing{Attacker: attacker, Defender: defender, Outcome: outcome})
	p.Selected = -1
	if len(p.Pairings) == len(p.Definition.Cards)/2 {
		var capturedByRound [3][3]int
		for _, card := range p.Captured {
			capturedByRound[0][int(card.Suit)]++
		}
		p.Score = game.ScoreWithModifiers(p.Captured, capturedByRound, p.Definition.Bounty, p.Modifiers).Total
		if p.Definition.Direction == "low" {
			p.Won = p.Score <= p.Definition.Par
		} else {
			p.Won = p.Score >= p.Definition.Par
		}
	}
	return nil
}

func newAbandonSession(hero heroes.Hero, rng *rand.Rand) *abandonSession {
	state, _ := newAbandonSessionForMission(hero, rng, 4, 11, 35)
	return state
}

func newAbandonSessionForMission(hero heroes.Hero, rng *rand.Rand, pairCount, boardCardCount, target int) (*abandonSession, error) {
	if pairCount < 1 || boardCardCount < 1 || target < 1 || pairCount*2 > boardCardCount+5 {
		return nil, errors.New("invalid Abandon mission configuration")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	deck := game.NewDeck()
	game.Shuffle(deck, rng)
	bounty := deck[0]
	remaining := deck[1:]
	personalPile := make([]game.Card, 0, 5)
	selected := make(map[game.Card]bool, 5)
	suitsFound := [3]bool{}
	for _, card := range remaining {
		suit := int(card.Suit)
		if !suitsFound[suit] {
			personalPile = append(personalPile, card)
			selected[card] = true
			suitsFound[suit] = true
		}
	}
	for _, card := range remaining {
		if len(personalPile) == 5 {
			break
		}
		if !selected[card] {
			personalPile = append(personalPile, card)
			selected[card] = true
		}
	}
	available := make([]game.Card, 0, len(remaining)-len(personalPile))
	for _, card := range remaining {
		if !selected[card] {
			available = append(available, card)
		}
	}
	if len(available) < boardCardCount {
		return nil, errors.New("not enough cards for Abandon board")
	}
	board := append([]game.Card(nil), available[:boardCardCount]...)
	hasBountyCard := false
	for _, card := range board {
		hasBountyCard = hasBountyCard || card.Suit == bounty.Suit
	}
	if !hasBountyCard {
		for _, card := range available[boardCardCount:] {
			if card.Suit == bounty.Suit {
				board[len(board)-1] = card
				break
			}
		}
	}
	revealed := make([]bool, len(board))
	highestBountyIndex := -1
	for index, card := range board {
		if card.Suit == bounty.Suit && (highestBountyIndex < 0 || card.Face > board[highestBountyIndex].Face) {
			highestBountyIndex = index
		}
	}
	for index, card := range board {
		revealed[index] = card.Suit == bounty.Suit && index != highestBountyIndex
	}
	pileRevealed := make([]bool, len(personalPile))
	for index := 0; index < clampInt(3+hero.Draft, 0, len(pileRevealed)); index++ {
		pileRevealed[index] = true
	}
	return &abandonSession{
		RulesVersion: abandonRulesVersion,
		Board:        board, PersonalPile: personalPile, Target: target, PairCount: pairCount, Bounty: bounty,
		DrawPile: replacementDeck(append(append([]game.Card{bounty}, board...), personalPile...), rng),
		Revealed: revealed, PileRevealed: pileRevealed,
		Used: make([]bool, len(board)), PileUsed: make([]bool, len(personalPile)),
		SelectedIndex: -1, Pairings: make([]abandonPairing, 0, pairCount),
		Captured: []game.Card{}, Penalties: make([]int, len(personalPile)),
	}, nil
}

func (a *abandonSession) flipBoardCard(index int) error {
	if a == nil || a.Finished || len(a.Pairings) >= a.PairCount {
		return errors.New("Abandon game is complete")
	}
	if a.SelectedSource != "" {
		return errors.New("finish the current pairing first")
	}
	if index < 0 || index >= len(a.Board) || a.Used[index] || a.Revealed[index] {
		return errors.New("selected board card is unavailable or already revealed")
	}
	a.Revealed[index] = true
	return nil
}

func (a *abandonSession) selectPairCard(source string, index int, hero heroes.Hero) error {
	if a == nil || a.Finished || len(a.Pairings) >= a.PairCount {
		return errors.New("Abandon game is complete")
	}
	card, err := a.availableCard(source, index)
	if err != nil {
		return err
	}
	if a.SelectedSource == "" {
		a.SelectedSource, a.SelectedIndex = source, index
		a.revealCard(source, index)
		return nil
	}
	if a.SelectedSource == source && a.SelectedIndex == index {
		return errors.New("choose a different second card")
	}
	first, err := a.availableCard(a.SelectedSource, a.SelectedIndex)
	if err != nil {
		return err
	}
	a.revealCard(source, index)
	outcome := game.ResolveWithModifiers(first, card, heroModifiers(hero))
	pairing := abandonPairing{First: first, Second: card, Tie: outcome.Winner < 0}
	if outcome.Winner >= 0 {
		captured := outcome.Captured
		pairing.Captured = &captured
		a.Captured = append(a.Captured, captured)
	}
	a.Pairings = append(a.Pairings, pairing)
	a.markUsed(a.SelectedSource, a.SelectedIndex)
	a.markUsed(source, index)
	a.SelectedSource, a.SelectedIndex = "", -1
	a.Score = abandonCapturedScore(a.Captured, a.Bounty.Suit, hero)
	a.finishIfReady(hero)
	return nil
}

func (a *abandonSession) revealPairCard(source string, index int) error {
	if a == nil || a.Finished || len(a.Pairings) >= a.PairCount {
		return errors.New("Abandon game is complete")
	}
	if a.SelectedSource == "" {
		return errors.New("choose the first card before revealing a second")
	}
	if a.SelectedSource == source && a.SelectedIndex == index {
		return errors.New("choose a different second card")
	}
	if _, err := a.availableCard(a.SelectedSource, a.SelectedIndex); err != nil {
		return err
	}
	if _, err := a.availableCard(source, index); err != nil {
		return err
	}
	a.revealCard(source, index)
	return nil
}

func (a *abandonSession) availableCard(source string, index int) (game.Card, error) {
	switch source {
	case abandonBoardSource:
		if index >= 0 && index < len(a.Board) && !a.Used[index] {
			return a.Board[index], nil
		}
	case abandonPileSource:
		if index >= 0 && index < len(a.PersonalPile) && !a.PileUsed[index] {
			return a.PersonalPile[index], nil
		}
	default:
		return game.Card{}, errors.New("choose a card from the board or personal pile")
	}
	return game.Card{}, errors.New("selected card is unavailable")
}

func (a *abandonSession) revealCard(source string, index int) {
	switch source {
	case abandonBoardSource:
		a.Revealed[index] = true
	case abandonPileSource:
		a.PileRevealed[index] = true
	}
}

func (a *abandonSession) markUsed(source string, index int) {
	if source == abandonBoardSource {
		a.Used[index] = true
	} else {
		a.PileUsed[index] = true
	}
}

func (a *abandonSession) finishIfReady(hero heroes.Hero) {
	if len(a.Pairings) != a.PairCount {
		return
	}
	a.Score = abandonCapturedScore(a.Captured, a.Bounty.Suit, hero)
	mods := heroModifiers(hero)
	for index, card := range a.PersonalPile {
		a.PileRevealed[index] = true
		if !a.PileUsed[index] {
			a.Penalties[index] = -modifiers.ScoringFace(card.Face, card.Suit == a.Bounty.Suit, mods)
			a.Score += a.Penalties[index]
		}
	}
	a.Finished = true
	a.Evaluated = true
	a.Won = a.Score >= a.Target
}

func abandonCapturedScore(captured []game.Card, bounty game.Suit, hero heroes.Hero) int {
	var capturedByRound [3][3]int
	for _, card := range captured {
		if card.Valid() {
			capturedByRound[0][int(card.Suit)]++
		}
	}
	return game.ScoreWithModifiers(captured, capturedByRound, bounty, heroModifiers(hero)).Total
}

func (a *abandonSession) webView() *webAbandonState {
	if a == nil {
		return nil
	}
	board := make([]webAbandonCard, len(a.Board))
	for index, card := range a.Board {
		item := webAbandonCard{Index: index, Revealed: a.Revealed[index], Used: a.Used[index]}
		if item.Revealed {
			revealedCard := card
			item.Card = &revealedCard
		}
		board[index] = item
	}
	personalPile := make([]webAbandonCard, len(a.PersonalPile))
	for index, card := range a.PersonalPile {
		item := webAbandonCard{Index: index, Revealed: a.PileRevealed[index], Used: a.PileUsed[index]}
		if item.Revealed {
			revealedCard := card
			item.Card = &revealedCard
		}
		if a.Finished && !item.Used {
			penalty := a.Penalties[index]
			item.Penalty = &penalty
		}
		personalPile[index] = item
	}
	pairResults := make([]webAbandonPairResult, len(a.Pairings))
	for index, pairing := range a.Pairings {
		pairResults[index] = webAbandonPairResult{Captured: pairing.Captured, Tie: pairing.Tie}
	}
	return &webAbandonState{
		Target: a.Target, PairCount: a.PairCount, Bounty: a.Bounty,
		Board: board, PersonalPile: personalPile, PairResults: pairResults,
		SelectedSource: a.SelectedSource, SelectedIndex: a.SelectedIndex,
		Score: a.Score, Complete: a.Finished, Evaluation: abandonEvaluation(a), Won: a.Won,
	}
}

func abandonEvaluation(a *abandonSession) string {
	if a == nil || !a.Evaluated {
		return "pending"
	}
	if a.Won {
		return "win"
	}
	return "loss"
}

func (p *puzzleSession) webView() *webPuzzleState {
	if p == nil {
		return nil
	}
	cards := make([]webPuzzleCard, len(p.Definition.Cards))
	for index, card := range p.Definition.Cards {
		cards[index] = webPuzzleCard{
			Card: card, Index: index, Used: p.Used[index],
			Selectable: !p.complete() && !p.Used[index],
		}
	}
	pairResults := make([]webPuzzlePair, len(p.Pairings))
	captured := make([]game.Card, 0, len(p.Captured))
	var capturedByRound [3][3]int
	runningScore := 0
	for index, pairing := range p.Pairings {
		result := "Tie"
		var capturedCard *game.Card
		if pairing.Outcome.Winner == 0 {
			result = "Attacker wins"
			card := pairing.Defender
			capturedCard = &card
		} else if pairing.Outcome.Winner == 1 {
			result = "Defender wins"
			card := pairing.Attacker
			capturedCard = &card
		}
		if capturedCard != nil {
			captured = append(captured, *capturedCard)
			capturedByRound[0][int(capturedCard.Suit)]++
		}
		score := game.ScoreWithModifiers(captured, capturedByRound, p.Definition.Bounty, p.Modifiers).Total
		pairResults[index] = webPuzzlePair{
			Cards: []game.Card{pairing.Attacker, pairing.Defender}, Captured: capturedCard,
			Score: score - runningScore, Result: result,
		}
		runningScore = score
	}
	startedAt := p.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
		p.StartedAt = startedAt
	}
	return &webPuzzleState{
		ID: p.Definition.ID, Name: p.Definition.Title, Cards: cards,
		BountySuit: p.Definition.Bounty, Par: p.Definition.Par, Target: p.Definition.Par,
		Direction: p.Definition.Direction, Hint: p.Definition.Hint, SelectedIndex: p.Selected,
		PairResults: pairResults, Captured: append([]game.Card(nil), p.Captured...),
		Modifiers: p.Modifiers, Score: p.Score, Complete: p.complete(), Won: p.Won,
		StartedAt: startedAt,
	}
}

func (p *puzzleSession) complete() bool {
	return p != nil && len(p.Pairings) == len(p.Definition.Cards)/2
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func maxInt(first, second int) int {
	if first > second {
		return first
	}
	return second
}

func specialVariant(variant string) (string, error) {
	if variant == "" {
		return "main", nil
	}
	switch variant {
	case "main", "fog", "puzzler", "abandon", "abandon-duel", "rummy", "ladder", "poker", "poker-duel", "tug-war":
		return variant, nil
	default:
		return "", fmt.Errorf("unknown variant %q", variant)
	}
}
