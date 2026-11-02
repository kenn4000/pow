package app

import (
	"errors"
	"fmt"
	"math/rand"

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
	Board         []game.Card
	BoardID       string
	Target        int
	CategorySuits [3]game.Suit
	Bounty        game.Card
	Shift         int
	Revealed      []bool
	Used          []bool
	DrawPile      []game.Card
	DrawIndex     int
	DrawTokens    int
	Selected      int
	Pairings      []abandonPairing
	Score         int
	Finished      bool
	Evaluated     bool
	Won           bool
	Penalties     []int
}

type abandonPairing struct {
	First  game.Card `json:"first"`
	Second game.Card `json:"second"`
	Score  int       `json:"score"`
}

type webAbandonCard struct {
	Index    int        `json:"index"`
	Card     *game.Card `json:"card,omitempty"`
	Revealed bool       `json:"revealed"`
	Used     bool       `json:"used"`
	Penalty  *int       `json:"penalty,omitempty"`
}

type webAbandonState struct {
	BoardID        string                 `json:"boardId"`
	Target         int                    `json:"target"`
	Evaluation     string                 `json:"evaluation"`
	Won            bool                   `json:"won"`
	Bounty         game.Card              `json:"bounty"`
	CategorySuits  [3]game.Suit           `json:"categorySuits"`
	CanChangeSuits bool                   `json:"canChangeSuits"`
	Board          []webAbandonCard       `json:"board"`
	PairResults    []webAbandonPairResult `json:"pairResults"`
	DrawTokens     int                    `json:"drawTokens"`
	DrawPileCount  int                    `json:"drawPileCount"`
	SelectedIndex  int                    `json:"selectedIndex"`
	Score          int                    `json:"score"`
	Complete       bool                   `json:"complete"`
}

type webAbandonPairResult struct {
	Cards       []game.Card `json:"cards"`
	DisplayCard game.Card   `json:"displayCard"`
	IsPair      bool        `json:"isPair"`
	Score       int         `json:"score"`
}

type abandonBoardOption struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Target int    `json:"target"`
}

type abandonBoardCard struct {
	Face     int
	Category int
}

type abandonBoardDefinition struct {
	abandonBoardOption
	Cards []abandonBoardCard
}

var abandonBoardCatalog = []abandonBoardDefinition{
	{abandonBoardOption: abandonBoardOption{ID: "board-01", Name: "Abandon Board 1", Target: 54}, Cards: []abandonBoardCard{{1, 1}, {1, 2}, {9, 0}, {9, 1}, {8, 0}, {4, 0}, {3, 0}, {6, 1}, {2, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-02", Name: "Abandon Board 2", Target: 58}, Cards: []abandonBoardCard{{4, 1}, {4, 2}, {7, 0}, {7, 1}, {2, 0}, {3, 0}, {5, 0}, {8, 1}, {1, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-03", Name: "Abandon Board 3", Target: 60}, Cards: []abandonBoardCard{{8, 1}, {8, 2}, {9, 0}, {9, 1}, {7, 0}, {5, 0}, {2, 0}, {3, 1}, {4, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-04", Name: "Abandon Board 4", Target: 61}, Cards: []abandonBoardCard{{1, 1}, {1, 2}, {3, 0}, {3, 1}, {4, 0}, {7, 0}, {2, 0}, {6, 1}, {8, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-05", Name: "Abandon Board 5", Target: 62}, Cards: []abandonBoardCard{{8, 1}, {8, 2}, {1, 0}, {1, 1}, {7, 0}, {5, 0}, {6, 0}, {2, 1}, {9, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-06", Name: "Abandon Board 6", Target: 63}, Cards: []abandonBoardCard{{4, 1}, {4, 2}, {9, 0}, {9, 1}, {2, 0}, {6, 0}, {5, 0}, {8, 1}, {1, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-07", Name: "Abandon Board 7", Target: 64}, Cards: []abandonBoardCard{{7, 1}, {7, 2}, {2, 0}, {2, 1}, {5, 0}, {6, 0}, {8, 0}, {4, 1}, {3, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-08", Name: "Abandon Board 8", Target: 65}, Cards: []abandonBoardCard{{8, 1}, {8, 2}, {2, 0}, {2, 1}, {6, 0}, {7, 0}, {9, 0}, {5, 1}, {4, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-09", Name: "Abandon Board 9", Target: 66}, Cards: []abandonBoardCard{{3, 1}, {3, 2}, {1, 0}, {1, 1}, {2, 0}, {8, 0}, {9, 0}, {5, 1}, {7, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-10", Name: "Abandon Board 10", Target: 67}, Cards: []abandonBoardCard{{6, 1}, {6, 2}, {9, 0}, {9, 1}, {7, 0}, {3, 0}, {1, 0}, {2, 1}, {4, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-11", Name: "Abandon Board 11", Target: 68}, Cards: []abandonBoardCard{{7, 1}, {7, 2}, {2, 0}, {2, 1}, {1, 0}, {5, 0}, {8, 0}, {9, 1}, {3, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-12", Name: "Abandon Board 12", Target: 70}, Cards: []abandonBoardCard{{7, 1}, {7, 2}, {9, 0}, {9, 1}, {2, 0}, {1, 0}, {6, 0}, {8, 1}, {3, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-13", Name: "Abandon Board 13", Target: 72}, Cards: []abandonBoardCard{{7, 1}, {7, 2}, {9, 0}, {9, 1}, {1, 0}, {2, 0}, {4, 0}, {3, 1}, {8, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-14", Name: "Abandon Board 14", Target: 75}, Cards: []abandonBoardCard{{4, 1}, {4, 2}, {5, 0}, {5, 1}, {1, 0}, {8, 0}, {7, 0}, {2, 1}, {9, 1}}},
	{abandonBoardOption: abandonBoardOption{ID: "board-15", Name: "Abandon Board 15", Target: 76}, Cards: []abandonBoardCard{{7, 1}, {7, 2}, {4, 0}, {4, 1}, {9, 0}, {1, 0}, {8, 0}, {2, 1}, {5, 1}}},
}

func abandonBoardByID(id string) (abandonBoardDefinition, bool) {
	for _, board := range abandonBoardCatalog {
		if board.ID == id {
			return board, true
		}
	}
	return abandonBoardDefinition{}, false
}

func abandonBoardOptions() []abandonBoardOption {
	options := make([]abandonBoardOption, len(abandonBoardCatalog))
	for index, board := range abandonBoardCatalog {
		options[index] = board.abandonBoardOption
	}
	return options
}

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
		Definition: definition, Modifiers: heroModifiers(hero), Selected: -1,
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
	state, _ := newAbandonSessionForBoard(hero, rng, "board-01")
	return state
}

func newAbandonSessionForBoard(hero heroes.Hero, rng *rand.Rand, boardID string) (*abandonSession, error) {
	definition, ok := abandonBoardByID(boardID)
	if !ok {
		return nil, fmt.Errorf("unknown abandon board %q", boardID)
	}
	bountySuit := game.Blade
	if rng != nil {
		bountySuit = game.Suit(rng.Intn(3))
	}
	categorySuits := [3]game.Suit{
		bountySuit,
		game.Suit((int(bountySuit) + 1) % 3),
		game.Suit((int(bountySuit) + 2) % 3),
	}
	board := make([]game.Card, len(definition.Cards))
	for index, item := range definition.Cards {
		board[index] = game.Card{Suit: categorySuits[item.Category], Face: item.Face}
	}
	boardSet := make(map[game.Card]bool, len(board))
	for _, card := range board {
		boardSet[card] = true
	}
	revealed := make([]bool, len(board))
	state := &abandonSession{
		Board: board, BoardID: definition.ID, Target: definition.Target,
		CategorySuits: categorySuits, Shift: hero.Shift,
		Revealed: revealed, Used: make([]bool, len(board)),
		Penalties: make([]int, len(board)), Selected: -1, Pairings: []abandonPairing{},
		DrawTokens: clampInt(2+hero.Draft, 0, 4),
	}
	state.rebuildBeforePlay(rng)
	return state, nil
}

func (a *abandonSession) rebuildBeforePlay(rng *rand.Rand) {
	boardSet := make(map[game.Card]bool, len(a.Board))
	for _, card := range a.Board {
		boardSet[card] = true
	}
	bountyFace := 1
	for boardSet[game.Card{Suit: a.CategorySuits[0], Face: bountyFace}] {
		bountyFace++
	}
	a.Bounty = game.Card{Suit: a.CategorySuits[0], Face: bountyFace}
	a.DrawPile = make([]game.Card, 0, 17)
	for _, card := range game.NewDeck() {
		if card != a.Bounty && !boardSet[card] {
			a.DrawPile = append(a.DrawPile, card)
		}
	}
	a.DrawIndex = 0
	if rng != nil {
		game.Shuffle(a.DrawPile, rng)
	}
	clear(a.Revealed)
	clear(a.Penalties)
	for count := 0; count < 3; count++ {
		lowest := -1
		for index, card := range a.Board {
			if card.Suit != a.Bounty.Suit || a.Revealed[index] {
				continue
			}
			if lowest < 0 || card.Face < a.Board[lowest].Face {
				lowest = index
			}
		}
		if lowest < 0 {
			break
		}
		a.Revealed[lowest] = true
	}
}

func (a *abandonSession) assignCategorySuit(category int, suit game.Suit, rng *rand.Rand) error {
	if a == nil || a.Finished || a.Selected >= 0 || len(a.Pairings) > 0 {
		return errors.New("suits can only be changed before the first pairing")
	}
	if category < 0 || category >= len(a.CategorySuits) || !suit.Valid() {
		return errors.New("choose a valid category and suit")
	}
	next := a.CategorySuits
	if next[category] == suit {
		return nil
	}
	other := -1
	for index, assigned := range next {
		if assigned == suit {
			other = index
			break
		}
	}
	if other < 0 {
		return errors.New("suit mapping is invalid")
	}
	next[category], next[other] = next[other], next[category]
	return a.setCategorySuits(next, rng)
}

func (a *abandonSession) rotateCategorySuits(rng *rand.Rand) error {
	if a == nil || a.Finished || a.Selected >= 0 || len(a.Pairings) > 0 {
		return errors.New("suits can only be changed before the first pairing")
	}
	return a.setCategorySuits([3]game.Suit{
		a.CategorySuits[1], a.CategorySuits[2], a.CategorySuits[0],
	}, rng)
}

func (a *abandonSession) setCategorySuits(next [3]game.Suit, rng *rand.Rand) error {
	seen := [3]bool{}
	for _, suit := range next {
		if !suit.Valid() || seen[suit] {
			return errors.New("suit mapping must use each suit once")
		}
		seen[suit] = true
	}
	remap := [3]game.Suit{}
	for category, oldSuit := range a.CategorySuits {
		remap[oldSuit] = next[category]
	}
	for index := range a.Board {
		a.Board[index].Suit = remap[a.Board[index].Suit]
	}
	a.CategorySuits = next
	a.rebuildBeforePlay(rng)
	return nil
}

func (a *abandonSession) selectCard(index int) error {
	if a == nil || a.Finished || len(a.Pairings) >= 4 {
		return errors.New("abandon game is complete")
	}
	if index < 0 || index >= len(a.Board) || a.Used[index] {
		return errors.New("selected board card is unavailable")
	}
	if a.Selected >= 0 {
		return errors.New("finish the current pairing first")
	}
	a.Selected = index
	a.Revealed[index] = true
	return nil
}

func (a *abandonSession) pairBoard(index int, hero heroes.Hero) error {
	if a == nil || a.Selected < 0 {
		return errors.New("select a first board card")
	}
	if index < 0 || index >= len(a.Board) || index == a.Selected || a.Used[index] {
		return errors.New("selected board card is unavailable")
	}
	a.Revealed[index] = true
	a.completePair(a.Board[index], hero)
	a.Used[a.Selected], a.Used[index] = true, true
	a.Selected = -1
	a.finishIfReady(hero)
	return nil
}

func (a *abandonSession) pairDraw(hero heroes.Hero) error {
	if a == nil || a.Selected < 0 {
		return errors.New("select a first board card")
	}
	if a.DrawTokens <= 0 {
		return errors.New("no draw tokens remain")
	}
	if a.DrawIndex >= len(a.DrawPile) {
		return errors.New("draw pile is empty")
	}
	card := a.DrawPile[a.DrawIndex]
	a.DrawIndex++
	a.DrawTokens--
	a.completePair(card, hero)
	a.Used[a.Selected] = true
	a.Selected = -1
	a.finishIfReady(hero)
	return nil
}

func (a *abandonSession) completePair(card game.Card, hero heroes.Hero) {
	first := a.Board[a.Selected]
	score := scoreAbandonPair(first, card, hero)
	a.Score += score
	a.Pairings = append(a.Pairings, abandonPairing{First: first, Second: card, Score: score})
}

func (a *abandonSession) finishIfReady(hero heroes.Hero) {
	if len(a.Pairings) != 4 {
		return
	}
	for index, card := range a.Board {
		if !a.Used[index] {
			if len(a.Penalties) != len(a.Board) {
				a.Penalties = make([]int, len(a.Board))
			}
			a.Penalties[index] = -abandonValue(card, hero)
			a.Score += a.Penalties[index]
			a.Revealed[index] = true
		}
	}
	a.Finished = true
	a.Evaluated = true
	a.Won = a.Score >= a.Target
}

func scoreAbandonPair(first, second game.Card, hero heroes.Hero) int {
	firstFace, secondFace := abandonEffectiveFace(first, hero), abandonEffectiveFace(second, hero)
	if firstFace == secondFace && first.Suit != second.Suit {
		score := 30 + 3*firstFace
		if first.Suit == game.Blade || second.Suit == game.Blade {
			score += 2 * heroModifiers(hero).Bounty
		}
		return score
	}
	higher := first
	if secondFace > firstFace {
		higher = second
	}
	score := firstFace
	if secondFace > firstFace {
		score = secondFace
	}
	if higher.Suit == game.Blade {
		score += heroModifiers(hero).Bounty
	}
	return score
}

func abandonValue(card game.Card, hero heroes.Hero) int {
	value := abandonEffectiveFace(card, hero)
	if card.Suit == game.Blade {
		value += heroModifiers(hero).Bounty
	}
	return value
}

func abandonEffectiveFace(card game.Card, hero heroes.Hero) int {
	return maxInt(1, card.Face+hero.Shift)
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
	pairResults := make([]webAbandonPairResult, len(a.Pairings))
	for index, pairing := range a.Pairings {
		displayCard, isPair := abandonPairDisplayCard(pairing.First, pairing.Second, a.Bounty.Suit, a.Shift)
		pairResults[index] = webAbandonPairResult{
			Cards: []game.Card{pairing.First, pairing.Second}, DisplayCard: displayCard,
			IsPair: isPair, Score: pairing.Score,
		}
	}
	canChangeSuits := !a.Finished && a.Selected < 0 && len(a.Pairings) == 0
	for index := range board {
		if a.Finished && !a.Used[index] && index < len(a.Penalties) {
			penalty := a.Penalties[index]
			board[index].Penalty = &penalty
		}
	}
	return &webAbandonState{
		BoardID: a.BoardID, Target: a.Target, Bounty: a.Bounty, Board: board, PairResults: pairResults,
		CategorySuits: a.CategorySuits, CanChangeSuits: canChangeSuits,
		DrawTokens: a.DrawTokens, DrawPileCount: len(a.DrawPile) - a.DrawIndex,
		SelectedIndex: a.Selected, Score: a.Score, Complete: a.Finished,
		Evaluation: abandonEvaluation(a), Won: a.Won,
	}
}

func abandonPairDisplayCard(first, second game.Card, bountySuit game.Suit, shift int) (game.Card, bool) {
	firstFace, secondFace := maxInt(1, first.Face+shift), maxInt(1, second.Face+shift)
	isPair := firstFace == secondFace && first.Suit != second.Suit
	if isPair {
		if first.Suit == bountySuit {
			return first, true
		}
		if second.Suit == bountySuit {
			return second, true
		}
		return first, true
	}
	if secondFace > firstFace {
		return second, false
	}
	return first, false
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
	return &webPuzzleState{
		ID: p.Definition.ID, Name: p.Definition.Title, Cards: cards,
		BountySuit: p.Definition.Bounty, Par: p.Definition.Par, Target: p.Definition.Par,
		Direction: p.Definition.Direction, Hint: p.Definition.Hint, SelectedIndex: p.Selected,
		PairResults: pairResults, Captured: append([]game.Card(nil), p.Captured...),
		Modifiers: p.Modifiers, Score: p.Score, Complete: p.complete(), Won: p.Won,
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
	case "main", "fog", "puzzler", "abandon":
		return variant, nil
	default:
		return "", fmt.Errorf("unknown variant %q", variant)
	}
}
