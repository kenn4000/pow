package app

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

const (
	abandonDuelPairsPerPlayer = 6
	abandonDuelBoardSize      = 16
	abandonDuelTurnCount      = 2 * abandonDuelPairsPerPlayer
)

type abandonDuelModifiers struct {
	B int `json:"bounty"`
	S int `json:"shift"`
	F int `json:"favor"`
	D int `json:"draft"`
}

type abandonDuelHeroParams struct {
	B int
	S int
	F int
	D int
}

type abandonDuelPlayer struct {
	PersonalPile []game.Card
	PileRevealed []bool
	PileUsed     []bool
	Prisoners    []game.Card
	PairCount    int
	TurnsTaken   int
}

type abandonDuelPairing struct {
	Player   int        `json:"player"`
	First    game.Card  `json:"first"`
	Second   game.Card  `json:"second"`
	Captured *game.Card `json:"captured,omitempty"`
	Tie      bool       `json:"tie"`
}

type abandonDuelGame struct {
	Heroes          [2]heroes.Hero
	Setup           abandonDuelSetup
	Modifiers       abandonDuelModifiers
	Bounty          game.Card
	Board           []game.Card
	DrawPile        []game.Card
	Discard         []game.Card
	BoardRevealed   []bool
	BoardUsed       []bool
	Players         [2]abandonDuelPlayer
	Round           int
	RoundTurn       int
	StartingPlayer  int
	ActivePlayer    int
	SelectedSource  string
	SelectedIndex   int
	FlippedThisTurn bool
	Pairings        []abandonDuelPairing
	RoundScores     [2][2]int
	RoundPairCounts [2][2]int
	TotalScores     [2]int
	GameOver        bool
	Winner          int
	Tiebreak        string
}

type abandonDuelSetup struct {
	Board        []game.Card
	PlayerHand   []game.Card
	CPUHand      []game.Card
	BoardSize    int
	HandSize     int
	PairCount    int
	Rounds       int
	BountySuits  [2]int
	MirrorHands  bool
	Snake        bool
	OpponentAlgo string
}

func newAbandonDuelGame(heroA, heroB heroes.Hero, rng *rand.Rand) (*abandonDuelGame, error) {
	return newAbandonDuelGameWithSetup(heroA, heroB, abandonDuelSetup{
		BoardSize: abandonDuelBoardSize, HandSize: 5, PairCount: abandonDuelPairsPerPlayer,
		Rounds: 2, BountySuits: [2]int{-1, -1}, MirrorHands: true, Snake: true,
	}, rng)
}

func newAbandonDuelGameWithSetup(
	heroA, heroB heroes.Hero,
	setup abandonDuelSetup,
	rng *rand.Rand,
) (*abandonDuelGame, error) {
	if rng == nil {
		return nil, errors.New("Abandon Duel requires a random source")
	}
	if setup.BoardSize < 2 || setup.HandSize < 2 || setup.PairCount < 1 ||
		setup.PairCount*2 > setup.BoardSize+setup.HandSize || setup.Rounds < 1 || setup.Rounds > 2 {
		return nil, errors.New("Abandon Duel campaign setup is invalid")
	}
	if len(setup.Board) > 0 && len(setup.Board) != setup.BoardSize ||
		len(setup.PlayerHand) > 0 && len(setup.PlayerHand) != setup.HandSize ||
		len(setup.CPUHand) > 0 && len(setup.CPUHand) != setup.HandSize {
		return nil, errors.New("Abandon Duel campaign cards do not match their configured sizes")
	}
	setup.Board = append([]game.Card(nil), setup.Board...)
	setup.PlayerHand = append([]game.Card(nil), setup.PlayerHand...)
	setup.CPUHand = append([]game.Card(nil), setup.CPUHand...)
	modifiers := abandonDuelModifiers{
		B: clampInt(heroA.Bounty+heroB.Bounty, -3, 3),
		S: clampInt(heroA.Shift+heroB.Shift, 0, 4),
		F: heroA.Favored + heroB.Favored,
		D: clampInt(heroA.Draft+heroB.Draft, -1, 1),
	}
	duel := &abandonDuelGame{
		Heroes: [2]heroes.Hero{heroA, heroB}, Setup: setup, Modifiers: modifiers,
		Round: 1, StartingPlayer: 0, ActivePlayer: 0, SelectedIndex: -1,
		Winner: -1,
	}
	if err := duel.dealRound(rng); err != nil {
		return nil, err
	}
	if err := duel.validateState(); err != nil {
		return nil, fmt.Errorf("initialize Abandon Duel: %w", err)
	}
	return duel, nil
}

func (g *abandonDuelGame) dealRound(rng *rand.Rand) error {
	if g == nil || rng == nil {
		return errors.New("Abandon Duel cannot deal without a game and random source")
	}
	deck := game.NewDeck()
	game.Shuffle(deck, rng)
	selected := make(map[game.Card]bool, len(deck))
	for _, card := range g.Setup.Board {
		selected[card] = true
	}
	for _, card := range g.Setup.PlayerHand {
		selected[card] = true
	}
	if !g.Setup.MirrorHands {
		for _, card := range g.Setup.CPUHand {
			selected[card] = true
		}
	}
	roundIndex := g.Round - 1
	bountySuit := g.Setup.BountySuits[roundIndex]
	if bountySuit < 0 || bountySuit > int(game.Shield) {
		bountySuit = -1
	}
	bountyIndex := -1
	for index, card := range deck {
		suitMatches := bountySuit < 0 || int(card.Suit) == bountySuit
		if bountySuit < 0 && roundIndex > 0 {
			suitMatches = card.Suit != g.Bounty.Suit
		}
		if suitMatches {
			if !selected[card] {
				bountyIndex = index
				break
			}
		}
	}
	if bountyIndex < 0 {
		return errors.New("Abandon Duel could not deal the configured bounty")
	}
	bounty := deck[bountyIndex]
	deck = append(deck[:bountyIndex], deck[bountyIndex+1:]...)
	selected = map[game.Card]bool{bounty: true}
	for _, card := range g.Setup.Board {
		if !card.Valid() || selected[card] {
			return fmt.Errorf("invalid or duplicate Abandon Duel board card %v", card)
		}
		selected[card] = true
	}

	playerHand := append([]game.Card(nil), g.Setup.PlayerHand...)
	if len(playerHand) == 0 {
		playerHand = make([]game.Card, 0, g.Setup.HandSize)
		if bountySuit >= 0 {
			for _, card := range deck {
				if int(card.Suit) == bountySuit {
					playerHand = append(playerHand, card)
					break
				}
			}
		} else {
			for _, card := range deck {
				if card.Suit == bounty.Suit {
					playerHand = append(playerHand, card)
					break
				}
			}
		}
	}
	for _, card := range playerHand {
		if !card.Valid() || selected[card] {
			return fmt.Errorf("invalid or duplicate Abandon Duel player card %v", card)
		}
		selected[card] = true
	}
	for _, card := range deck {
		if len(playerHand) == g.Setup.HandSize {
			break
		}
		if !selected[card] {
			playerHand = append(playerHand, card)
			selected[card] = true
		}
	}
	if len(playerHand) != g.Setup.HandSize {
		return errors.New("not enough cards for the Abandon Duel player hand")
	}

	cpuHand := append([]game.Card(nil), g.Setup.CPUHand...)
	if g.Setup.MirrorHands {
		cpuHand = append([]game.Card(nil), playerHand...)
	} else {
		for _, card := range cpuHand {
			if !card.Valid() || selected[card] {
				return fmt.Errorf("invalid or duplicate Abandon Duel opponent card %v", card)
			}
			selected[card] = true
		}
		for _, card := range deck {
			if len(cpuHand) == g.Setup.HandSize {
				break
			}
			if !selected[card] {
				cpuHand = append(cpuHand, card)
				selected[card] = true
			}
		}
		if len(cpuHand) != g.Setup.HandSize {
			return errors.New("not enough cards for the Abandon Duel opponent hand")
		}
	}

	board := append([]game.Card(nil), g.Setup.Board...)
	if len(board) == 0 {
		board = make([]game.Card, 0, g.Setup.BoardSize)
		for _, card := range deck {
			if len(board) == g.Setup.BoardSize {
				break
			}
			if !selected[card] {
				board = append(board, card)
				selected[card] = true
			}
		}
	}
	if len(board) != g.Setup.BoardSize {
		return errors.New("not enough cards for the Abandon Duel board")
	}
	hasBountyCard := false
	for _, card := range board {
		hasBountyCard = hasBountyCard || card.Suit == bounty.Suit
	}
	if !hasBountyCard {
		replaced := false
		for _, card := range deck {
			if card.Suit == bounty.Suit && !selected[card] {
				selected[board[len(board)-1]] = false
				board[len(board)-1] = card
				selected[card] = true
				replaced = true
				break
			}
		}
		if !replaced {
			return errors.New("Abandon Duel board has no available bounty-suit card")
		}
	}
	revealed := make([]bool, len(board))
	highestBountyIndex := -1
	for index, card := range board {
		if card.Suit == bounty.Suit &&
			(highestBountyIndex < 0 || card.Face > board[highestBountyIndex].Face) {
			highestBountyIndex = index
		}
	}
	for index, card := range board {
		revealed[index] = card.Suit == bounty.Suit && index != highestBountyIndex
	}
	visiblePrivateCount := clampInt(3+g.Modifiers.D, 0, len(playerHand))
	players := [2]abandonDuelPlayer{}
	for index := range players {
		hand := playerHand
		if index == 1 {
			hand = cpuHand
		}
		players[index] = abandonDuelPlayer{
			PersonalPile: append([]game.Card(nil), hand...),
			PileRevealed: make([]bool, len(hand)),
			PileUsed:     make([]bool, len(hand)),
			Prisoners:    []game.Card{},
		}
		for cardIndex := 0; cardIndex < visiblePrivateCount; cardIndex++ {
			players[index].PileRevealed[cardIndex] = true
		}
	}
	g.Bounty = bounty
	g.Board, g.BoardRevealed, g.BoardUsed = board, revealed, make([]bool, len(board))
	g.Players = players
	removed := append(append([]game.Card{bounty}, board...), playerHand...)
	if !g.Setup.MirrorHands {
		removed = append(removed, cpuHand...)
	}
	g.DrawPile = replacementDeck(removed, rng)
	if len(g.DrawPile) > 0 {
		rng.Shuffle(len(g.DrawPile), func(i, j int) {
			g.DrawPile[i], g.DrawPile[j] = g.DrawPile[j], g.DrawPile[i]
		})
	}
	g.Discard = nil
	g.RoundTurn, g.ActivePlayer = 0, g.StartingPlayer
	g.SelectedSource, g.SelectedIndex, g.FlippedThisTurn = "", -1, false
	g.Pairings = []abandonDuelPairing{}
	if err := g.beginTurn(rng); err != nil {
		return err
	}
	return nil
}

func (g *abandonDuelGame) turnLimit() int {
	return 2 * g.Setup.PairCount
}

func (g *abandonDuelGame) turnOwner(turn int) int {
	if !g.Setup.Snake {
		return (g.StartingPlayer + turn) % 2
	}
	return abandonDuelSnakeOwner(turn, g.StartingPlayer)
}

type webAbandonDuelCard struct {
	Card       *game.Card `json:"card,omitempty"`
	Index      int        `json:"index"`
	Revealed   bool       `json:"revealed"`
	Used       bool       `json:"used"`
	Selectable bool       `json:"selectable"`
}

type webAbandonDuelPlayer struct {
	HeroID       string               `json:"heroId"`
	HeroName     string               `json:"heroName"`
	PairCount    int                  `json:"pairCount"`
	PersonalPile []webAbandonDuelCard `json:"personalPile"`
	Prisoners    []game.Card          `json:"prisoners"`
	PreviewScore int                  `json:"previewScore"`
}

type webAbandonDuelState struct {
	Round           int                     `json:"round"`
	TurnNumber      int                     `json:"turnNumber"`
	TurnLimit       int                     `json:"turnLimit"`
	StartingPlayer  int                     `json:"startingPlayer"`
	ActivePlayer    int                     `json:"activePlayer"`
	Bounty          game.Card               `json:"bounty"`
	Modifiers       abandonDuelModifiers    `json:"modifiers"`
	Board           []webAbandonDuelCard    `json:"board"`
	Players         [2]webAbandonDuelPlayer `json:"players"`
	Pairings        []abandonDuelPairing    `json:"pairings"`
	RoundScores     [2][2]int               `json:"roundScores"`
	RoundPairCounts [2][2]int               `json:"roundPairCounts"`
	TotalScores     [2]int                  `json:"totalScores"`
	SelectedSource  string                  `json:"selectedSource,omitempty"`
	SelectedIndex   int                     `json:"selectedIndex"`
	GameOver        bool                    `json:"gameOver"`
	Winner          int                     `json:"winner"`
	Tiebreak        string                  `json:"tiebreak,omitempty"`
}

func (g *abandonDuelGame) webView(phase string, activePlayer int, canAct bool) *webAbandonDuelState {
	if g == nil {
		return nil
	}
	result := &webAbandonDuelState{
		Round: g.Round, TurnNumber: g.RoundTurn + 1, TurnLimit: g.turnLimit(),
		StartingPlayer: g.StartingPlayer, ActivePlayer: g.ActivePlayer,
		Bounty: g.Bounty, Modifiers: g.Modifiers,
		Board:       make([]webAbandonDuelCard, len(g.Board)),
		Pairings:    append([]abandonDuelPairing(nil), g.Pairings...),
		RoundScores: g.RoundScores, RoundPairCounts: g.RoundPairCounts, TotalScores: g.TotalScores,
		SelectedSource: g.SelectedSource, SelectedIndex: g.SelectedIndex,
		GameOver: g.GameOver, Winner: g.Winner, Tiebreak: g.Tiebreak,
	}
	pairPhase := phase == "abandon-duel-pair-first" || phase == "abandon-duel-pair-second"
	for index, card := range g.Board {
		item := webAbandonDuelCard{
			Index: index, Revealed: g.BoardRevealed[index], Used: g.BoardUsed[index],
			Selectable: canAct && pairPhase && !g.BoardUsed[index] &&
				!(g.SelectedSource == abandonBoardSource && g.SelectedIndex == index),
		}
		if item.Revealed && !item.Used {
			revealedCard := card
			item.Card = &revealedCard
		}
		if g.SelectedSource == abandonBoardSource && g.SelectedIndex == index {
			item.Selectable = false
		}
		result.Board[index] = item
	}
	for playerIndex, player := range g.Players {
		view := webAbandonDuelPlayer{
			HeroID: g.Heroes[playerIndex].ID, HeroName: g.Heroes[playerIndex].Name,
			PairCount:    player.PairCount,
			PersonalPile: make([]webAbandonDuelCard, len(player.PersonalPile)),
			Prisoners:    append([]game.Card(nil), player.Prisoners...),
			PreviewScore: abandonDuelScore(
				player.Prisoners, player.PersonalPile, player.PileUsed, g.Bounty.Suit, g.Modifiers,
			),
		}
		for index, card := range player.PersonalPile {
			item := webAbandonDuelCard{
				Index: index, Revealed: player.PileRevealed[index], Used: player.PileUsed[index],
				Selectable: canAct && pairPhase && playerIndex == activePlayer && !player.PileUsed[index] &&
					!(g.SelectedSource == abandonPileSource && g.SelectedIndex == index),
			}
			if item.Revealed && !item.Used {
				revealedCard := card
				item.Card = &revealedCard
			}
			if g.SelectedSource == abandonPileSource && playerIndex == activePlayer && g.SelectedIndex == index {
				item.Selectable = false
			}
			view.PersonalPile[index] = item
		}
		result.Players[playerIndex] = view
	}
	return result
}

func (g *abandonDuelGame) beginTurn(rng *rand.Rand) error {
	for g.RoundTurn < g.turnLimit() {
		owner := g.turnOwner(g.RoundTurn)
		g.ActivePlayer = owner
		g.Players[owner].TurnsTaken++
		g.FlippedThisTurn = !g.hasHiddenBoardCard()
		if g.availableCardCount(owner) < 2 {
			g.RoundTurn++
			continue
		}
		return nil
	}
	return g.finishRound(rng)
}

func abandonDuelSnakeOwner(turn, startingPlayer int) int {
	if turn == 0 {
		return startingPlayer
	}
	block := (turn - 1) / 2
	if block%2 == 0 {
		return 1 - startingPlayer
	}
	return startingPlayer
}

func (g *abandonDuelGame) availableCardCount(player int) int {
	count := 0
	for index := range g.Board {
		if !g.BoardUsed[index] {
			count++
		}
	}
	for index := range g.Players[player].PersonalPile {
		if !g.Players[player].PileUsed[index] {
			count++
		}
	}
	return count
}

func (g *abandonDuelGame) hasHiddenBoardCard() bool {
	for index := range g.Board {
		if !g.BoardUsed[index] && !g.BoardRevealed[index] {
			return true
		}
	}
	return false
}

func (g *abandonDuelGame) phase() string {
	if g.GameOver {
		return "gameover"
	}
	if g.SelectedSource != "" {
		return "abandon-duel-pair-second"
	}
	if !g.FlippedThisTurn && g.hasHiddenBoardCard() {
		return "abandon-duel-reveal"
	}
	return "abandon-duel-pair-first"
}

func (g *abandonDuelGame) hiddenBoardIndices() []int {
	indices := make([]int, 0, len(g.Board))
	for index := range g.Board {
		if !g.BoardUsed[index] && !g.BoardRevealed[index] {
			indices = append(indices, index)
		}
	}
	return indices
}

func (g *abandonDuelGame) flipBoardCard(index int) error {
	if g == nil || g.GameOver {
		return errors.New("Abandon Duel is complete")
	}
	if g.SelectedSource != "" {
		return errors.New("finish the current pairing first")
	}
	if index < 0 || index >= len(g.Board) || g.BoardUsed[index] || g.BoardRevealed[index] {
		return errors.New("selected board card is unavailable or already revealed")
	}
	g.BoardRevealed[index] = true
	g.FlippedThisTurn = true
	return nil
}

func (g *abandonDuelGame) revealPairCard(source string, index int) error {
	if g == nil || g.GameOver {
		return errors.New("Abandon Duel is complete")
	}
	if g.SelectedSource == "" {
		return errors.New("choose the first card before revealing a second")
	}
	if g.SelectedSource == source && g.SelectedIndex == index {
		return errors.New("choose a different second card")
	}
	if _, err := g.availableCard(g.SelectedSource, g.SelectedIndex); err != nil {
		return err
	}
	if _, err := g.availableCard(source, index); err != nil {
		return err
	}
	g.revealCard(source, g.ActivePlayer, index)
	return nil
}

func (g *abandonDuelGame) selectPairCard(source string, index int, rng *rand.Rand) (bool, error) {
	if g == nil || g.GameOver {
		return false, errors.New("Abandon Duel is complete")
	}
	if rng == nil {
		return false, errors.New("Abandon Duel requires a random source")
	}
	card, err := g.availableCard(source, index)
	if err != nil {
		return false, err
	}
	if g.SelectedSource == "" {
		g.SelectedSource, g.SelectedIndex = source, index
		g.revealCard(source, g.ActivePlayer, index)
		return false, nil
	}
	if g.SelectedSource == source && g.SelectedIndex == index {
		return false, errors.New("choose a different second card")
	}
	first, err := g.availableCard(g.SelectedSource, g.SelectedIndex)
	if err != nil {
		return false, err
	}
	g.revealCard(source, g.ActivePlayer, index)
	captured, _ := abandonDuelResolve(first, card, g.Modifiers.S, g.Modifiers.F)
	pairing := abandonDuelPairing{Player: g.ActivePlayer, First: first, Second: card, Tie: captured == nil}
	if captured != nil {
		pairing.Captured = captured
		g.Players[g.ActivePlayer].Prisoners = append(g.Players[g.ActivePlayer].Prisoners, *captured)
	}
	g.Pairings = append(g.Pairings, pairing)
	g.markUsed(g.SelectedSource, g.SelectedIndex)
	g.markUsed(source, index)
	g.Players[g.ActivePlayer].PairCount++
	g.SelectedSource, g.SelectedIndex = "", -1
	g.FlippedThisTurn = false
	g.RoundTurn++
	if err := g.beginTurn(rng); err != nil {
		return false, err
	}
	if err := g.validateState(); err != nil {
		return false, fmt.Errorf("invalid Abandon Duel state after pair: %w", err)
	}
	return true, nil
}

func (g *abandonDuelGame) availableCard(source string, index int) (game.Card, error) {
	switch source {
	case abandonBoardSource:
		if index >= 0 && index < len(g.Board) && !g.BoardUsed[index] {
			return g.Board[index], nil
		}
	case abandonPileSource:
		player := &g.Players[g.ActivePlayer]
		if index >= 0 && index < len(player.PersonalPile) && !player.PileUsed[index] {
			return player.PersonalPile[index], nil
		}
	default:
		return game.Card{}, errors.New("choose a card from the board or your personal pile")
	}
	return game.Card{}, errors.New("selected card is unavailable")
}

func (g *abandonDuelGame) revealCard(source string, playerIndex, index int) {
	if source == abandonBoardSource {
		g.BoardRevealed[index] = true
	} else {
		g.Players[playerIndex].PileRevealed[index] = true
	}
}

func (g *abandonDuelGame) markUsed(source string, index int) {
	if source == abandonBoardSource {
		g.BoardUsed[index] = true
	} else {
		g.Players[g.ActivePlayer].PileUsed[index] = true
	}
}

func (g *abandonDuelGame) finishRound(rng *rand.Rand) error {
	if g.GameOver {
		return nil
	}
	for playerIndex := range g.Players {
		g.RoundPairCounts[g.Round-1][playerIndex] = g.Players[playerIndex].PairCount
		score := abandonDuelScore(
			g.Players[playerIndex].Prisoners, g.Players[playerIndex].PersonalPile,
			g.Players[playerIndex].PileUsed, g.Bounty.Suit, g.Modifiers,
		)
		g.RoundScores[g.Round-1][playerIndex] = score
		g.TotalScores[playerIndex] += score
	}
	if g.Round < g.Setup.Rounds {
		g.Round = 2
		g.StartingPlayer = 1
		return g.dealRound(rng)
	}
	g.GameOver = true
	switch {
	case g.TotalScores[0] > g.TotalScores[1]:
		g.Winner, g.Tiebreak = 0, "total_score"
	case g.TotalScores[1] > g.TotalScores[0]:
		g.Winner, g.Tiebreak = 1, "total_score"
	default:
		g.Winner, g.Tiebreak = -1, "score_tied"
	}
	return nil
}

func abandonDuelScore(prisoners, personalPile []game.Card, pileUsed []bool, bounty game.Suit, stats abandonDuelModifiers) int {
	var highest [3]int
	var counts [3]int
	for _, card := range prisoners {
		suitIndex := int(card.Suit)
		counts[suitIndex]++
		if card.Face > highest[suitIndex] {
			highest[suitIndex] = card.Face
		}
	}
	score := 0
	for suitIndex, count := range counts {
		if count == 0 {
			continue
		}
		perCard := highest[suitIndex] + stats.S
		if game.Suit(suitIndex) == bounty {
			perCard += 1 + stats.B
		}
		score += perCard * count
	}
	for index, card := range personalPile {
		if index < len(pileUsed) && !pileUsed[index] {
			score -= abandonDuelPenalty(card, bounty, stats.B, stats.S)
		}
	}
	return score
}

func abandonDuelPenalty(card game.Card, bounty game.Suit, bountyBonus, shift int) int {
	value := card.Face + shift
	if card.Suit == bounty {
		value += 1 + bountyBonus
	}
	return value
}

func (g *abandonDuelGame) validateState() error {
	if g == nil {
		return errors.New("Abandon Duel state is unavailable")
	}
	if g.Round < 1 || g.Round > g.Setup.Rounds || g.ActivePlayer < 0 || g.ActivePlayer > 1 {
		return errors.New("Abandon Duel has invalid round or turn state")
	}
	if !g.Bounty.Valid() || len(g.Board) != g.Setup.BoardSize ||
		len(g.BoardRevealed) != len(g.Board) || len(g.BoardUsed) != len(g.Board) {
		return errors.New("Abandon Duel has invalid bounty or board state")
	}
	seen := make(map[game.Card]bool, len(game.NewDeck()))
	seen[g.Bounty] = true
	for _, card := range g.Board {
		if !card.Valid() || seen[card] {
			return fmt.Errorf("Abandon Duel has an invalid or duplicate board card %v", card)
		}
		seen[card] = true
	}
	for playerIndex, player := range g.Players {
		for _, card := range player.PersonalPile {
			if !card.Valid() || !g.Setup.MirrorHands && seen[card] {
				return fmt.Errorf("Abandon Duel has an invalid or duplicate personal card %v", card)
			}
			if g.Setup.MirrorHands && playerIndex == 1 {
				continue
			}
			seen[card] = true
		}
	}
	for _, card := range g.DrawPile {
		if !card.Valid() || seen[card] {
			return fmt.Errorf("Abandon Duel has an invalid or duplicate draw-pile card %v", card)
		}
		seen[card] = true
	}
	for _, card := range g.Discard {
		if !card.Valid() || seen[card] {
			return fmt.Errorf("Abandon Duel has an invalid or duplicate discarded card %v", card)
		}
		seen[card] = true
	}
	if len(g.Players[0].PersonalPile) != g.Setup.HandSize ||
		len(g.Players[1].PersonalPile) != g.Setup.HandSize ||
		len(g.Players[0].PileRevealed) != g.Setup.HandSize ||
		len(g.Players[1].PileRevealed) != g.Setup.HandSize ||
		len(g.Players[0].PileUsed) != g.Setup.HandSize ||
		len(g.Players[1].PileUsed) != g.Setup.HandSize {
		return errors.New("Abandon Duel players have invalid personal pile sizes")
	}
	if g.Setup.MirrorHands {
		for index, card := range g.Players[0].PersonalPile {
			if g.Players[1].PersonalPile[index] != card {
				return errors.New("Abandon Duel players do not have identical personal piles")
			}
		}
	}
	if !g.GameOver && g.RoundTurn >= g.turnLimit() {
		return errors.New("Abandon Duel has an invalid round turn")
	}
	if g.SelectedSource == "" && g.SelectedIndex != -1 ||
		g.SelectedSource != "" && g.SelectedIndex < 0 {
		return errors.New("Abandon Duel has invalid selected-card state")
	}
	if g.SelectedSource == abandonBoardSource &&
		(g.SelectedIndex >= len(g.Board) || g.BoardUsed[g.SelectedIndex]) {
		return errors.New("Abandon Duel selected board card is unavailable")
	}
	if g.SelectedSource == abandonPileSource &&
		(g.SelectedIndex >= len(g.Players[g.ActivePlayer].PersonalPile) ||
			g.Players[g.ActivePlayer].PileUsed[g.SelectedIndex]) {
		return errors.New("Abandon Duel selected personal card is unavailable")
	}
	return nil
}
