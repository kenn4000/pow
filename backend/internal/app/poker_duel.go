package app

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

const (
	pokerDuelHandSize    = 6
	pokerDuelTurnSeconds = 15
)

type pokerDuelSharedStats struct {
	Draft     int `json:"draft"`
	Shift     int `json:"shift"`
	Favored   int `json:"favor"`
	Bounty    int `json:"bounty"`
	DeckFloor int `json:"deckFloor"`
}

type pokerDuelPlayer struct {
	Hand        []game.Card
	Table       [3][]game.Card
	DrawPile    []game.Card
	DiscardPile []game.Card
	TimeBank    time.Duration
	Done        bool
}

type pokerDuelPlayerScore struct {
	Hands            [3]game.PokerHandScore `json:"hands"`
	NineBonusTotal   int                    `json:"nineBonusTotal"`
	BountyBonusTotal int                    `json:"bountyBonusTotal"`
	BustPenaltyTotal int                    `json:"bustPenaltyTotal"`
	TableBonus       int                    `json:"tableBonus"`
	HandAHighBonus   int                    `json:"handAHighBonus"`
	HandCLowBonus    int                    `json:"handCLowBonus"`
	GameScore        int                    `json:"gameScore"`
	BankBonus        float64                `json:"bankBonus"`
	FinalScore       float64                `json:"finalScore"`
	ConstraintMet    bool                   `json:"constraintMet"`
	ConstraintReason string                 `json:"constraintFailReason,omitempty"`
}

type pokerDuelGame struct {
	Rules        game.PokerMissionRules
	HeroP1       heroes.Hero
	HeroP2       heroes.Hero
	SharedStats  pokerDuelSharedStats
	BountySuit   game.Suit
	Players      [2]pokerDuelPlayer
	ActivePlayer int
	TurnNumber   int
	TurnStarted  time.Time
	GameOver     bool
	Winner       int
	Tiebreak     string
	FinalScores  []pokerDuelPlayerScore
}

type pokerDuelSlotView struct {
	Cards    []webCard `json:"cards,omitempty"`
	Count    int       `json:"count"`
	Hidden   bool      `json:"hidden,omitempty"`
	CanPlace bool      `json:"canPlace,omitempty"`
}

type pokerDuelPlayerView struct {
	HeroID          string               `json:"heroId"`
	HeroName        string               `json:"heroName"`
	Hand            []webCard            `json:"hand,omitempty"`
	HandCount       int                  `json:"handCount"`
	HandHidden      bool                 `json:"handHidden,omitempty"`
	Table           [3]pokerDuelSlotView `json:"table"`
	DrawPileCount   int                  `json:"drawPileCount"`
	DiscardPile     []game.Card          `json:"discardPile"`
	TimeBankSeconds float64              `json:"timeBankSeconds"`
	BankBonus       float64              `json:"bankBonus"`
	Done            bool                 `json:"done"`
}

type webPokerDuelState struct {
	Rules               webPokerDuelRules      `json:"rules"`
	Heroes              [2]webPokerDuelHero    `json:"heroes"`
	SharedStats         pokerDuelSharedStats   `json:"sharedStats"`
	BountySuit          game.Suit              `json:"bountySuit"`
	Players             [2]pokerDuelPlayerView `json:"players"`
	ActivePlayer        int                    `json:"activePlayer"`
	TurnNumber          int                    `json:"turnNumber"`
	TurnRemainingMS     int64                  `json:"turnRemainingMs"`
	CPUThinkRemainingMS int64                  `json:"cpuThinkRemainingMs,omitempty"`
	GameOver            bool                   `json:"gameOver"`
	Winner              int                    `json:"winner"`
	Tiebreak            string                 `json:"tiebreak,omitempty"`
	FinalScores         []pokerDuelPlayerScore `json:"finalScores,omitempty"`
}

type webPokerDuelHero struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Bounty int    `json:"bounty"`
	Shift  int    `json:"shift"`
	Favor  int    `json:"favor"`
	Draft  int    `json:"draft"`
}

type webPokerDuelRules struct {
	NineBase   int `json:"nineBase"`
	BountyBase int `json:"bountyBase"`
}

func pokerDuelRulesView(rules game.PokerMissionRules) webPokerDuelRules {
	return webPokerDuelRules{
		NineBase: rules.NineBase, BountyBase: rules.BountyBase,
	}
}

func pokerDuelDefaultRules() game.PokerMissionRules {
	return game.PokerMissionRules{NineBase: 12, BountyBase: 1}
}

func newPokerDuelGame(heroP1, heroP2 heroes.Hero, rng *rand.Rand, now time.Time) (*pokerDuelGame, error) {
	if rng == nil {
		return nil, errors.New("Poker duel requires a random source")
	}

	shared := pokerDuelSharedStats{
		Draft:   maxInt(heroP1.Draft, heroP2.Draft),
		Shift:   maxInt(heroP1.Shift, heroP2.Shift),
		Favored: maxInt(heroP1.Favored, heroP2.Favored),
		Bounty:  maxInt(heroP1.Bounty, heroP2.Bounty),
	}
	shared.DeckFloor = minInt(2, 2-shared.Draft)

	deck := make([]game.Card, 0, len(game.NewDeck()))
	for _, card := range game.NewDeck() {
		if card.Face >= shared.DeckFloor {
			deck = append(deck, card)
		}
	}
	if len(deck) < pokerDuelHandSize {
		return nil, errors.New("shared Draft leaves too few cards for Poker")
	}
	suits := [...]game.Suit{game.Blade, game.Fang, game.Shield}

	result := &pokerDuelGame{
		Rules: pokerDuelDefaultRules(), HeroP1: heroP1, HeroP2: heroP2,
		SharedStats: shared, BountySuit: suits[rng.Intn(len(suits))],
		ActivePlayer: rng.Intn(2), TurnNumber: 1, TurnStarted: now,
		Winner: -1,
	}
	for index := range result.Players {
		playerDeck := append([]game.Card(nil), deck...)
		rng.Shuffle(len(playerDeck), func(i, j int) { playerDeck[i], playerDeck[j] = playerDeck[j], playerDeck[i] })
		result.Players[index] = pokerDuelPlayer{
			Hand:     append([]game.Card(nil), playerDeck[:pokerDuelHandSize]...),
			DrawPile: append([]game.Card(nil), playerDeck[pokerDuelHandSize:]...),
		}
	}
	if err := result.validateState(); err != nil {
		return nil, err
	}
	return result, nil
}

func minInt(first, second int) int {
	if first < second {
		return first
	}
	return second
}

func (g *pokerDuelGame) takeTurn(index, slot int, redraw bool, now time.Time, rng *rand.Rand) error {
	if g.GameOver {
		return errors.New("Poker duel is already complete")
	}
	if err := g.validateAction(now); err != nil {
		return err
	}
	player := &g.Players[g.ActivePlayer]
	if player.Done {
		return errors.New("your Poker duel table is complete")
	}
	if index < 0 || index >= len(player.Hand) {
		return errors.New("choose a card from your hand")
	}
	if redraw {
		if len(player.DrawPile) == 0 {
			return errors.New("your draw pile is empty")
		}
	} else if slot < 0 || slot >= len(player.Table) {
		return errors.New("choose a valid Poker hand slot")
	} else if len(player.Table[slot]) >= pokerSlotCapacity {
		return errors.New("that Poker hand is full")
	} else if !game.CanPlacePokerCard(player.Table, slot) {
		return errors.New("Poker hands must stay within one card of each other")
	}
	if rng == nil {
		return errors.New("Poker duel requires a random source")
	}
	remaining := time.Duration(pokerDuelTurnSeconds)*time.Second - now.Sub(g.TurnStarted)
	if remaining < 0 {
		remaining = 0
	}
	player.TimeBank += remaining
	if redraw {
		card := player.Hand[index]
		player.Hand = removePokerDuelCard(player.Hand, index)
		player.DiscardPile = append(player.DiscardPile, card)
		player.Hand = append(player.Hand, drawPokerDuelCard(&player.DrawPile))
	} else {
		card := player.Hand[index]
		player.Hand = removePokerDuelCard(player.Hand, index)
		player.Table[slot] = append(player.Table[slot], card)
	}
	refillPokerDuelHand(player)
	return g.completeTurn(now)
}

func (g *pokerDuelGame) handleTimeout(now time.Time, rng *rand.Rand) error {
	if g.GameOver {
		return errors.New("Poker duel is already complete")
	}
	if err := g.validateAction(now); err != nil {
		return err
	}
	if now.Sub(g.TurnStarted) < time.Duration(pokerDuelTurnSeconds)*time.Second {
		return errors.New("the turn clock has not expired")
	}
	if rng == nil {
		return errors.New("Poker duel requires a random source")
	}
	if rng == nil {
		return errors.New("Poker duel requires a random source")
	}
	return g.completeTurn(now)
}

func (g *pokerDuelGame) completeTurn(now time.Time) error {
	if err := g.validateState(); err != nil {
		return fmt.Errorf("invalid Poker duel state after turn: %w", err)
	}
	g.updateDoneFlags()
	g.TurnNumber++
	if g.shouldEndGame() {
		return g.completeGame()
	}
	previous := g.ActivePlayer
	if err := g.advanceActivePlayer(); err != nil {
		return err
	}
	if g.ActivePlayer == previous {
		g.TurnStarted = now
	}
	return nil
}

func (g *pokerDuelGame) updateDoneFlags() {
	for index := range g.Players {
		player := &g.Players[index]
		player.Done = allPokerDuelSlotsFull(player) ||
			len(player.Hand) == 0 && len(player.DrawPile) == 0 && !pokerDuelHasLegalMove(player)
	}
}

func (g *pokerDuelGame) shouldEndGame() bool {
	if g.Players[0].Done && g.Players[1].Done {
		return true
	}
	return len(g.Players[0].DrawPile) == 0 && len(g.Players[1].DrawPile) == 0 &&
		!pokerDuelHasLegalMove(&g.Players[0]) && !pokerDuelHasLegalMove(&g.Players[1])
}

func (g *pokerDuelGame) advanceActivePlayer() error {
	for attempts := 0; attempts < 2; attempts++ {
		g.ActivePlayer = 1 - g.ActivePlayer
		if !g.Players[g.ActivePlayer].Done {
			return nil
		}
	}
	if g.Players[0].Done && g.Players[1].Done {
		return nil
	}
	return errors.New("Poker duel has no active player")
}

func (g *pokerDuelGame) completeGame() error {
	scores := make([]pokerDuelPlayerScore, 2)
	for index := range scores {
		score, err := g.scorePlayer(index)
		if err != nil {
			return err
		}
		scores[index] = score
	}
	g.GameOver = true
	g.Winner, g.Tiebreak = pokerDuelWinner(scores)
	g.FinalScores = append([]pokerDuelPlayerScore(nil), scores...)
	return nil
}

func (g *pokerDuelGame) scorePlayer(index int) (pokerDuelPlayerScore, error) {
	if index < 0 || index >= len(g.Players) {
		return pokerDuelPlayerScore{}, errors.New("invalid Poker player")
	}
	hero := g.sharedHeroParameters()
	bankBonus := g.Players[index].TimeBank.Seconds() / 12
	score, err := game.ScorePokerTable(g.Players[index].Table, hero, g.BountySuit, g.Rules)
	if err != nil {
		return pokerDuelPlayerScore{}, err
	}
	result := pokerDuelPlayerScore{
		Hands: score.Hands, TableBonus: score.TableBonus,
		HandAHighBonus: score.HandAHighBonus, HandCLowBonus: score.HandCLowBonus,
		BankBonus:        bankBonus,
		ConstraintMet:    score.ConstraintPassed,
		ConstraintReason: score.ConstraintFailure,
	}
	if !score.ConstraintPassed {
		result.FinalScore = result.BankBonus
		return result, nil
	}
	for _, hand := range score.Hands {
		result.NineBonusTotal += hand.NineBonus
		result.BountyBonusTotal += hand.BountyBonus
		result.BustPenaltyTotal += hand.BustPenalty
	}
	result.GameScore = score.Total
	result.FinalScore = float64(result.GameScore) + result.BankBonus
	return result, nil
}

func (g *pokerDuelGame) sharedHeroParameters() game.PokerHeroParameters {
	return game.PokerHeroParameters{
		Bounty: g.SharedStats.Bounty, Favored: g.SharedStats.Favored,
		Shift: g.SharedStats.Shift, Draft: g.SharedStats.Draft,
	}
}

func pokerDuelWinner(scores []pokerDuelPlayerScore) (int, string) {
	if len(scores) != 2 {
		return -1, "invalid_scores"
	}
	switch {
	case !scores[0].ConstraintMet && !scores[1].ConstraintMet:
		return -1, "both_constraints_failed"
	case !scores[0].ConstraintMet:
		return 1, "constraint"
	case !scores[1].ConstraintMet:
		return 0, "constraint"
	case scores[0].FinalScore > scores[1].FinalScore:
		return 0, "final_score"
	case scores[1].FinalScore > scores[0].FinalScore:
		return 1, "final_score"
	case scores[0].GameScore > scores[1].GameScore:
		return 0, "game_score"
	case scores[1].GameScore > scores[0].GameScore:
		return 1, "game_score"
	default:
		return -1, "game_score_tied"
	}
}

func firstOpenPokerDuelSlot(player *pokerDuelPlayer) int {
	for slot := range player.Table {
		if game.CanPlacePokerCard(player.Table, slot) {
			return slot
		}
	}
	return -1
}

func allPokerDuelSlotsFull(player *pokerDuelPlayer) bool {
	for _, cards := range player.Table {
		if len(cards) < pokerSlotCapacity {
			return false
		}
	}
	return true
}

func pokerDuelHasLegalMove(player *pokerDuelPlayer) bool {
	if len(player.Hand) == 0 {
		return false
	}
	return firstOpenPokerDuelSlot(player) >= 0 || len(player.DrawPile) > 0
}

func removePokerDuelCard(cards []game.Card, index int) []game.Card {
	copy(cards[index:], cards[index+1:])
	return cards[:len(cards)-1]
}

func drawPokerDuelCard(pile *[]game.Card) game.Card {
	last := len(*pile) - 1
	card := (*pile)[last]
	*pile = (*pile)[:last]
	return card
}

func refillPokerDuelHand(player *pokerDuelPlayer) {
	for len(player.Hand) < pokerDuelHandSize && len(player.DrawPile) > 0 {
		player.Hand = append(player.Hand, drawPokerDuelCard(&player.DrawPile))
	}
}

func (g *pokerDuelGame) webView(phase string, now time.Time, cpuPlayer int) *webPokerDuelState {
	result := &webPokerDuelState{
		Rules: pokerDuelRulesView(g.Rules), SharedStats: g.SharedStats, BountySuit: g.BountySuit,
		ActivePlayer: g.ActivePlayer, TurnNumber: g.TurnNumber,
		GameOver: g.GameOver, Winner: g.Winner, Tiebreak: g.Tiebreak,
	}
	if g.GameOver {
		result.FinalScores = append([]pokerDuelPlayerScore(nil), g.FinalScores...)
	}
	if (phase == "poker-duel-play" || phase == "poker-duel-thinking") && !g.GameOver {
		remaining := time.Duration(pokerDuelTurnSeconds)*time.Second - now.Sub(g.TurnStarted)
		if remaining < 0 {
			remaining = 0
		}
		result.TurnRemainingMS = remaining.Milliseconds()
	}
	heroList := [...]heroes.Hero{g.HeroP1, g.HeroP2}
	for index := range g.Players {
		player := &g.Players[index]
		hero := heroList[index]
		result.Heroes[index] = webPokerDuelHero{
			ID: hero.ID, Name: hero.Name, Bounty: hero.Bounty,
			Shift: hero.Shift, Favor: hero.Favored, Draft: hero.Draft,
		}
		view := pokerDuelPlayerView{
			HeroID: hero.ID, HeroName: hero.Name, HandCount: len(player.Hand),
			DrawPileCount: len(player.DrawPile), DiscardPile: append([]game.Card(nil), player.DiscardPile...),
			TimeBankSeconds: player.TimeBank.Seconds(), BankBonus: player.TimeBank.Seconds() / 12,
			Done: player.Done,
		}
		privateVisible := g.GameOver ||
			cpuPlayer >= 0 && index != cpuPlayer ||
			cpuPlayer < 0 && phase == "poker-duel-play" && index == g.ActivePlayer
		if privateVisible {
			view.Hand = make([]webCard, len(player.Hand))
			for cardIndex, card := range player.Hand {
				view.Hand[cardIndex] = webCard{
					Card: card, Index: cardIndex,
					Selectable: phase == "poker-duel-play" && index == g.ActivePlayer && !player.Done,
				}
			}
		} else {
			view.HandHidden = true
		}
		for slotIndex, cards := range player.Table {
			hidden := slotIndex > 0 && !g.GameOver &&
				(phase == "handoff" || index != g.ActivePlayer || index == cpuPlayer)
			slot := pokerDuelSlotView{
				Count: len(cards), Hidden: hidden,
				CanPlace: phase == "poker-duel-play" && index == g.ActivePlayer && index != cpuPlayer &&
					!player.Done && len(player.Hand) > 0 && game.CanPlacePokerCard(player.Table, slotIndex),
			}
			if !hidden {
				slot.Cards = make([]webCard, len(cards))
				for cardIndex, card := range cards {
					slot.Cards[cardIndex] = webCard{Card: card, Index: cardIndex}
				}
			}
			view.Table[slotIndex] = slot
		}
		result.Players[index] = view
	}
	return result
}

func (g *pokerDuelGame) validateAction(now time.Time) error {
	if g.GameOver {
		return errors.New("Poker duel is already complete")
	}
	if g.ActivePlayer < 0 || g.ActivePlayer >= len(g.Players) {
		return fmt.Errorf("invalid active Poker player %d", g.ActivePlayer)
	}
	if g.TurnNumber < 1 {
		return errors.New("Poker duel turn number must be positive")
	}
	if !g.GameOver && g.Players[g.ActivePlayer].Done {
		return errors.New("Poker duel has a completed active player")
	}
	if now.Before(g.TurnStarted) {
		return errors.New("Poker turn clock cannot begin in the future")
	}
	return nil
}

func (g *pokerDuelGame) validateState() error {
	wantStats := pokerDuelSharedStats{
		Draft:   maxInt(g.HeroP1.Draft, g.HeroP2.Draft),
		Shift:   maxInt(g.HeroP1.Shift, g.HeroP2.Shift),
		Favored: maxInt(g.HeroP1.Favored, g.HeroP2.Favored),
		Bounty:  maxInt(g.HeroP1.Bounty, g.HeroP2.Bounty),
	}
	wantStats.DeckFloor = minInt(2, 2-wantStats.Draft)
	if g.SharedStats != wantStats {
		return fmt.Errorf("Poker duel shared stats %+v do not match selected heroes %+v", g.SharedStats, wantStats)
	}
	if !g.BountySuit.Valid() {
		return errors.New("Poker duel has an invalid bounty suit")
	}
	if g.ActivePlayer < 0 || g.ActivePlayer >= len(g.Players) {
		return fmt.Errorf("invalid active Poker player %d", g.ActivePlayer)
	}
	expectedCards := 0
	if g.SharedStats.DeckFloor <= 9 {
		expectedCards = (10 - g.SharedStats.DeckFloor) * 3
	}
	for playerIndex, player := range g.Players {
		seen := make(map[game.Card]bool, expectedCards)
		if len(player.Hand) > pokerDuelHandSize {
			return fmt.Errorf("Poker player %d has more than %d cards in hand", playerIndex, pokerDuelHandSize)
		}
		if player.TimeBank < 0 {
			return fmt.Errorf("Poker player %d has a negative time bank", playerIndex)
		}
		for slotIndex, cards := range player.Table {
			if len(cards) > pokerSlotCapacity {
				return fmt.Errorf("Poker player %d hand %c exceeds five cards", playerIndex, 'A'+slotIndex)
			}
		}
		if !game.PokerHandsBalanced(player.Table) {
			return fmt.Errorf("Poker player %d has hands more than one card apart", playerIndex)
		}
		cardsInState := [][]game.Card{player.Hand, player.DrawPile, player.DiscardPile}
		cardsInState = append(cardsInState, player.Table[:]...)
		for _, zone := range cardsInState {
			for _, card := range zone {
				if !card.Valid() {
					return fmt.Errorf("Poker duel contains invalid card %v", card)
				}
				if seen[card] {
					return fmt.Errorf("Poker duel contains duplicate card %s", card)
				}
				seen[card] = true
			}
		}
		if len(seen) != expectedCards {
			return fmt.Errorf("Poker player %d has %d cards; deck floor requires %d",
				playerIndex, len(seen), expectedCards)
		}
	}
	return nil
}

func (s *webServer) applyPokerDuelAction(request actionRequest) error {
	if request.Action == "poker-duel-cpu-turn" {
		return s.applyPokerDuelCPUAction()
	}
	session := s.session
	duel := session.pokerDuel
	if duel == nil {
		return errors.New("Poker duel state is unavailable")
	}
	if session.phase != "poker-duel-play" || session.actor != duel.ActivePlayer {
		return errors.New("Poker duel is not accepting an action")
	}
	if session.mode != "hvh" && duel.ActivePlayer == 1 {
		return errors.New("the CPU is taking its turn")
	}
	return s.applyPokerDuelTurn(request, time.Now())
}

func (s *webServer) applyPokerDuelTurn(request actionRequest, now time.Time) error {
	session := s.session
	duel := session.pokerDuel
	if duel == nil {
		return errors.New("Poker duel state is unavailable")
	}
	if session.phase != "poker-duel-play" || session.actor != duel.ActivePlayer {
		return errors.New("Poker duel is not accepting an action")
	}
	if err := duel.validateAction(now); err != nil {
		return err
	}
	previousPlayer := duel.ActivePlayer
	var err error
	switch request.Action {
	case "poker-duel-place":
		err = duel.takeTurn(request.Index, request.Slot, false, now, s.rng)
	case "poker-duel-redraw":
		err = duel.takeTurn(request.Index, 0, true, now, s.rng)
	case "poker-duel-timeout":
		err = duel.handleTimeout(now, s.rng)
	default:
		return errors.New("unknown Poker duel action")
	}
	if err != nil {
		return err
	}
	s.finishPokerDuelTurn(previousPlayer, now, request.Action)
	return nil
}

func (s *webServer) finishPokerDuelTurn(previousPlayer int, now time.Time, action string) {
	session := s.session
	duel := session.pokerDuel
	session.actor = duel.ActivePlayer
	if duel.GameOver {
		session.phase, session.winner, session.tiebreak = "gameover", duel.Winner, duel.Tiebreak
		switch duel.Winner {
		case -1:
			session.message = "Poker Duel ends in a tie."
		case 0, 1:
			session.message = fmt.Sprintf("%s wins the Poker Duel.", s.playerName(duel.Winner))
		}
		score := 0
		if len(duel.FinalScores) > 0 {
			score = int(duel.FinalScores[0].FinalScore)
		}
		s.awardLocalMultiplayerPayout(score)
		return
	}
	if session.mode == "hvh" && duel.ActivePlayer != previousPlayer {
		session.phase = "handoff"
		session.handoffTarget = duel.ActivePlayer
		session.handoffPhase = "poker-duel-play"
		session.message = fmt.Sprintf("Pass the screen to Player %d. Their cards stay hidden until they continue.", duel.ActivePlayer+1)
		return
	}
	duel.TurnStarted = now
	session.phase = "poker-duel-play"
	if session.mode != "hvh" && duel.ActivePlayer == 1 {
		session.phase = "poker-duel-thinking"
		session.message = s.playerName(1) + " is thinking."
	} else if action == "poker-duel-timeout" {
		session.message = fmt.Sprintf("%s's turn timed out. They receive no time-bank bonus.", s.playerName(duel.ActivePlayer))
	} else if session.mode != "hvh" {
		session.message = "Your turn. Place or redraw a card before the 15-second clock expires."
	} else {
		session.message = fmt.Sprintf("Player %d, make your next move.", duel.ActivePlayer+1)
	}
}
