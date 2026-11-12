package app

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

const pokerHandSize = 6
const pokerSlotCount = 3
const pokerSlotCapacity = game.PokerHandCapacity
const pokerTurnLimit = 30

type pokerSession struct {
	Rules       game.PokerMissionRules
	Hero        game.PokerHeroParameters
	HeroID      string
	BountySuit  game.Suit
	DeckFloor   int
	HandSize    int
	DrawPile    []game.Card
	DiscardPile []game.Card
	Hand        []game.Card
	Slots       [pokerSlotCount][]game.Card
	Turns       int
	GamesPlayed int
	Streak      int
	LastScore   *game.PokerTableScore
	Ghost       *pokerGhostSession
}

type pokerGhostSession struct {
	Strategy  game.PokerGhostStrategy
	HandSize  int
	Hand      []game.Card
	DrawPile  []game.Card
	Slots     [pokerSlotCount][]game.Card
	Turns     int
	LastScore *game.PokerTableScore
}

type webPokerHeroStats struct {
	Bounty  int `json:"bounty"`
	Favored int `json:"favored"`
	Shift   int `json:"shift"`
	Draft   int `json:"draft"`
}

type webPokerState struct {
	HeroID            string                      `json:"heroId"`
	HeroName          string                      `json:"heroName"`
	Hero              webPokerHeroStats           `json:"hero"`
	BountySuit        game.Suit                   `json:"bountySuit"`
	DeckFloor         int                         `json:"deckFloor"`
	HandSize          int                         `json:"handSize"`
	DrawPileCount     int                         `json:"drawPileCount"`
	DiscardPileCount  int                         `json:"discardPileCount"`
	DiscardTop        *game.Card                  `json:"discardTop,omitempty"`
	Hand              []webCard                   `json:"hand"`
	Slots             [pokerSlotCount][]webCard   `json:"slots"`
	Turns             int                         `json:"turns"`
	TurnLimit         int                         `json:"turnLimit"`
	GameNumber        int                         `json:"gameNumber"`
	Streak            int                         `json:"streak"`
	StreakCount       int                         `json:"streakCount"`
	StreakScore       int                         `json:"streakScore"`
	Par               int                         `json:"par"`
	NineBase          int                         `json:"nineBase"`
	BountyBase        int                         `json:"bountyBase"`
	Constraint        game.PokerMissionConstraint `json:"constraint,omitempty"`
	Lesson            string                      `json:"lesson,omitempty"`
	GhostTeachingNote string                      `json:"ghostTeachingNote,omitempty"`
	RecommendedHeroes []string                    `json:"recommendedHeroes,omitempty"`
	Ghost             *webPokerGhostState         `json:"ghost,omitempty"`
	CanRedraw         bool                        `json:"canRedraw"`
	CanPlace          [pokerSlotCount]bool        `json:"canPlace"`
	Score             *game.PokerTableScore       `json:"score,omitempty"`
}

type webPokerGhostState struct {
	Strategy string                      `json:"strategy"`
	Turns    int                         `json:"turns"`
	Slots    [pokerSlotCount][]game.Card `json:"slots"`
	Score    *game.PokerTableScore       `json:"score,omitempty"`
}

func newPokerSession(heroID string, hero heroes.Hero, rules game.PokerMissionRules, rng *rand.Rand) (*pokerSession, error) {
	session := &pokerSession{Rules: rules}
	if err := session.startGame(heroID, hero, rng); err != nil {
		return nil, err
	}
	return session, nil
}

func (p *pokerSession) startGame(heroID string, hero heroes.Hero, rng *rand.Rand) error {
	if rng == nil {
		return errors.New("poker game requires a random source")
	}
	deckFloor := 2 - hero.Draft
	handSize := configuredPokerHandSize(p.Rules.HandSize, pokerHandSize)
	if handSize > len(game.NewDeck()) {
		return errors.New("Poker starting hand is larger than the deck")
	}
	deck, err := pokerDrawPile(hero.Draft, p.Rules.StartingHand)
	if err != nil {
		return err
	}
	if len(p.Rules.StartingHand) > handSize {
		return errors.New("Poker scripted starting hand exceeds its configured hand size")
	}
	if len(deck)+len(p.Rules.StartingHand) < handSize {
		return errors.New("hero Draft leaves too few cards for a poker hand")
	}
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	suits := [...]game.Suit{game.Blade, game.Fang, game.Shield}
	p.HeroID = heroID
	p.Hero = game.PokerHeroParameters{
		Bounty: hero.Bounty, Favored: hero.Favored, Shift: hero.Shift, Draft: hero.Draft,
	}
	p.DeckFloor = deckFloor
	p.HandSize = handSize
	if p.Rules.ForcedBountySuit != nil {
		if !p.Rules.ForcedBountySuit.Valid() {
			return errors.New("Poker mission has an invalid forced bounty suit")
		}
		p.BountySuit = *p.Rules.ForcedBountySuit
	} else {
		p.BountySuit = suits[rng.Intn(len(suits))]
	}
	p.DrawPile = deck
	p.Hand = make([]game.Card, 0, pokerHandSize)
	p.Hand = append(p.Hand, p.Rules.StartingHand...)
	p.DiscardPile = nil
	p.Slots = [pokerSlotCount][]game.Card{}
	p.Turns = 0
	p.GamesPlayed++
	p.LastScore = nil
	p.Ghost = nil
	for len(p.Hand) < p.HandSize {
		p.Hand = append(p.Hand, p.drawCard())
	}
	if p.Rules.Ghost != nil {
		ghost, err := newPokerGhostSession(hero.Draft, p.Rules.Ghost, p.HandSize)
		if err != nil {
			return err
		}
		p.Ghost = ghost
	}
	return nil
}

func configuredPokerHandSize(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func pokerDrawPile(draft int, startingHand []game.Card) ([]game.Card, error) {
	floor := 2 - draft
	excluded := make(map[game.Card]bool, len(startingHand))
	for _, card := range startingHand {
		if !card.Valid() {
			return nil, fmt.Errorf("Poker starting hand contains invalid card %v", card)
		}
		if excluded[card] {
			return nil, fmt.Errorf("Poker starting hand contains duplicate card %s", card)
		}
		excluded[card] = true
	}
	deck := make([]game.Card, 0, len(game.NewDeck()))
	for _, card := range game.NewDeck() {
		if !excluded[card] && card.Face >= floor {
			deck = append(deck, card)
		}
	}
	return deck, nil
}

func newPokerGhostSession(draft int, rules *game.PokerGhostRules, fallbackHandSize int) (*pokerGhostSession, error) {
	if rules == nil {
		return nil, errors.New("Poker ghost rules are unavailable")
	}
	if rules.Strategy != game.PokerGhostBounty && rules.Strategy != game.PokerGhostNine &&
		rules.Strategy != game.PokerGhostPairs && rules.Strategy != game.PokerGhostMedium {
		return nil, fmt.Errorf("unsupported Poker ghost strategy %q", rules.Strategy)
	}
	handSize := configuredPokerHandSize(rules.HandSize, fallbackHandSize)
	if handSize > len(game.NewDeck()) || len(rules.StartingHand) > handSize {
		return nil, errors.New("Poker ghost starting hand exceeds its configured hand size")
	}
	deck, err := pokerDrawPile(draft, rules.StartingHand)
	if err != nil {
		return nil, err
	}
	if len(deck)+len(rules.StartingHand) < handSize {
		return nil, errors.New("hero Draft leaves too few cards for the Poker ghost")
	}
	ghost := &pokerGhostSession{
		Strategy: rules.Strategy, HandSize: handSize,
		Hand: append([]game.Card(nil), rules.StartingHand...), DrawPile: deck,
	}
	for len(ghost.Hand) < ghost.HandSize {
		ghost.Hand = append(ghost.Hand, ghost.drawCard())
	}
	return ghost, nil
}

func (p *pokerSession) drawCard() game.Card {
	top := len(p.DrawPile) - 1
	card := p.DrawPile[top]
	p.DrawPile = p.DrawPile[:top]
	return card
}

func (g *pokerGhostSession) drawCard() game.Card {
	top := len(g.DrawPile) - 1
	card := g.DrawPile[top]
	g.DrawPile = g.DrawPile[:top]
	return card
}

func (p *pokerSession) webView(phase string) *webPokerState {
	result := &webPokerState{
		HeroID: p.HeroID, HeroName: heroName(p.HeroID),
		Hero: webPokerHeroStats{
			Bounty: p.Hero.Bounty, Favored: p.Hero.Favored,
			Shift: p.Hero.Shift, Draft: p.Hero.Draft,
		},
		BountySuit: p.BountySuit, DeckFloor: p.DeckFloor, HandSize: p.HandSize,
		DrawPileCount: len(p.DrawPile), DiscardPileCount: len(p.DiscardPile),
		Turns: p.Turns, TurnLimit: pokerTurnLimit, GameNumber: p.GamesPlayed,
		Streak: p.Streak, StreakCount: p.Rules.StreakCount,
		StreakScore: p.Rules.StreakScore, Par: p.Rules.Par,
		NineBase: p.Rules.NineBase, BountyBase: p.Rules.BountyBase,
		Constraint: p.Rules.Constraint,
	}
	if p.GamesPlayed == 1 {
		result.Lesson = p.Rules.Lesson
		if p.Rules.Ghost != nil {
			result.GhostTeachingNote = p.Rules.Ghost.TeachingNote
		}
	}
	for _, heroID := range p.Rules.RecommendedHeroes {
		result.RecommendedHeroes = append(result.RecommendedHeroes, heroName(heroID))
	}
	if len(p.DiscardPile) > 0 {
		top := p.DiscardPile[len(p.DiscardPile)-1]
		result.DiscardTop = &top
	}
	result.Hand = make([]webCard, len(p.Hand))
	for index, card := range p.Hand {
		result.Hand[index] = webCard{
			Card: card, Index: index, Selectable: phase == "poker-play",
			Placeable: phase == "poker-play" && pokerCardPlaceable(p.Rules.Constraint, card),
		}
	}
	for slotIndex, slot := range p.Slots {
		result.Slots[slotIndex] = make([]webCard, len(slot))
		for cardIndex, card := range slot {
			result.Slots[slotIndex][cardIndex] = webCard{Card: card, Index: cardIndex}
		}
		result.CanPlace[slotIndex] = phase == "poker-play" &&
			len(p.Hand) > 0 && game.CanPlacePokerCard(p.Slots, slotIndex)
	}
	result.CanRedraw = phase == "poker-play" && len(p.DrawPile) > 0 && len(p.Hand) > 0
	if phase != "poker-play" {
		result.Score = p.LastScore
	}
	if p.Ghost != nil {
		ghost := &webPokerGhostState{Strategy: string(p.Ghost.Strategy), Turns: p.Ghost.Turns}
		ghost.Slots = p.Ghost.Slots
		if phase != "poker-play" {
			ghost.Score = p.Ghost.LastScore
		}
		result.Ghost = ghost
	}
	return result
}

func (p *pokerSession) place(cardIndex, slotIndex int) error {
	if cardIndex < 0 || cardIndex >= len(p.Hand) {
		return errors.New("choose a card from your hand")
	}
	if slotIndex < 0 || slotIndex >= len(p.Slots) {
		return errors.New("choose a valid poker hand slot")
	}
	if len(p.Slots[slotIndex]) >= pokerSlotCapacity {
		return errors.New("that poker hand is full")
	}
	card := p.Hand[cardIndex]
	if !pokerCardPlaceable(p.Rules.Constraint, card) {
		return errors.New("9s cannot be placed in this mission; redraw or keep the card in your hand")
	}
	if !game.CanPlacePokerCard(p.Slots, slotIndex) {
		return errors.New("Poker hands must stay within one card of each other")
	}
	p.Hand = append(p.Hand[:cardIndex], p.Hand[cardIndex+1:]...)
	p.Slots[slotIndex] = append(p.Slots[slotIndex], card)
	p.refillHand()
	return nil
}

func (p *pokerSession) redraw(cardIndex int) error {
	if cardIndex < 0 || cardIndex >= len(p.Hand) {
		return errors.New("choose a card from your hand")
	}
	if len(p.DrawPile) == 0 {
		return errors.New("the draw pile is empty")
	}
	discard := p.Hand[cardIndex]
	p.Hand = append(p.Hand[:cardIndex], p.Hand[cardIndex+1:]...)
	p.DiscardPile = append(p.DiscardPile, discard)
	p.Hand = append(p.Hand, p.drawCard())
	p.refillHand()
	return nil
}

func (p *pokerSession) refillHand() {
	handSize := configuredPokerHandSize(p.HandSize, pokerHandSize)
	for len(p.Hand) < handSize && len(p.DrawPile) > 0 {
		p.Hand = append(p.Hand, p.drawCard())
	}
}

func pokerCardPlaceable(constraint game.PokerMissionConstraint, card game.Card) bool {
	return constraint != game.PokerNoNines || card.Face != 9
}

func (p *pokerSession) canPlace() bool {
	if len(p.Hand) == 0 {
		return false
	}
	for _, card := range p.Hand {
		if !pokerCardPlaceable(p.Rules.Constraint, card) {
			continue
		}
		for slotIndex := range p.Slots {
			if game.CanPlacePokerCard(p.Slots, slotIndex) {
				return true
			}
		}
	}
	return false
}

func (g *pokerGhostSession) canPlace(constraint game.PokerMissionConstraint) bool {
	for _, card := range g.Hand {
		if !pokerCardPlaceable(constraint, card) {
			continue
		}
		for slotIndex := range g.Slots {
			if game.CanPlacePokerCard(g.Slots, slotIndex) {
				return true
			}
		}
	}
	return false
}

func (g *pokerGhostSession) gameComplete(constraint game.PokerMissionConstraint) bool {
	allSlotsFull := true
	for _, slot := range g.Slots {
		if len(slot) < pokerSlotCapacity {
			allSlotsFull = false
			break
		}
	}
	return allSlotsFull || g.Turns >= pokerTurnLimit || !g.canPlace(constraint)
}

func (g *pokerGhostSession) takeTurn(p *pokerSession) (game.Card, int, bool, error) {
	cardIndex, slotIndex, ok := g.chooseMove(p.BountySuit, p.Rules.Constraint)
	if !ok {
		return game.Card{}, 0, false, nil
	}
	card := g.Hand[cardIndex]
	g.Hand = append(g.Hand[:cardIndex], g.Hand[cardIndex+1:]...)
	g.Slots[slotIndex] = append(g.Slots[slotIndex], card)
	g.Turns++
	for len(g.Hand) < g.HandSize && len(g.DrawPile) > 0 {
		g.Hand = append(g.Hand, g.drawCard())
	}
	if g.gameComplete(p.Rules.Constraint) {
		score, err := game.ScorePokerTable(g.Slots, p.Hero, p.BountySuit, p.Rules)
		if err != nil {
			return game.Card{}, 0, false, err
		}
		g.LastScore = &score
	}
	return card, slotIndex, true, nil
}

func (g *pokerGhostSession) chooseMove(bountySuit game.Suit, constraint game.PokerMissionConstraint) (int, int, bool) {
	type move struct {
		cardIndex int
		slotIndex int
		priority  int
		targetA   int
		face      int
		slotCount int
	}
	moves := []move{}
	for cardIndex, card := range g.Hand {
		if !pokerCardPlaceable(constraint, card) {
			continue
		}
		for slotIndex := range g.Slots {
			if !game.CanPlacePokerCard(g.Slots, slotIndex) {
				continue
			}
			priority := 1
			switch g.Strategy {
			case game.PokerGhostBounty:
				if card.Suit == bountySuit {
					priority = 0
				}
			case game.PokerGhostNine:
				if card.Face == 9 {
					priority = 0
				}
			case game.PokerGhostPairs, game.PokerGhostMedium:
				for _, placed := range g.Slots[slotIndex] {
					if placed.Face == card.Face {
						priority = 0
						break
					}
				}
			}
			targetA := 1
			if priority == 0 &&
				(g.Strategy == game.PokerGhostBounty || g.Strategy == game.PokerGhostNine) &&
				slotIndex == 0 {
				targetA = 0
			}
			moves = append(moves, move{
				cardIndex: cardIndex, slotIndex: slotIndex, priority: priority,
				targetA: targetA, face: card.Face, slotCount: len(g.Slots[slotIndex]),
			})
		}
	}
	if len(moves) == 0 {
		return 0, 0, false
	}
	best := moves[0]
	for _, candidate := range moves[1:] {
		if candidate.priority < best.priority ||
			candidate.priority == best.priority && candidate.targetA < best.targetA ||
			candidate.priority == best.priority && candidate.targetA == best.targetA && candidate.face > best.face ||
			candidate.priority == best.priority && candidate.targetA == best.targetA && candidate.face == best.face && candidate.slotCount < best.slotCount ||
			candidate.priority == best.priority && candidate.targetA == best.targetA && candidate.face == best.face && candidate.slotCount == best.slotCount && candidate.slotIndex < best.slotIndex {
			best = candidate
		}
	}
	return best.cardIndex, best.slotIndex, true
}

func (p *pokerSession) gameComplete() bool {
	allSlotsFull := true
	for _, slot := range p.Slots {
		if len(slot) < pokerSlotCapacity {
			allSlotsFull = false
			break
		}
	}
	return allSlotsFull || p.Turns >= pokerTurnLimit ||
		len(p.DrawPile) == 0 && !p.canPlace()
}

func (s *webServer) applyPokerAction(request actionRequest) error {
	session := s.session
	poker := session.poker
	if poker == nil {
		return errors.New("Poker state is unavailable")
	}
	switch request.Action {
	case "poker-place":
		if session.phase != "poker-play" {
			return errors.New("Poker is not accepting card placements")
		}
		if err := poker.place(request.Index, request.Slot); err != nil {
			return err
		}
		return s.completePokerTurn("Card placed.")
	case "poker-redraw":
		if session.phase != "poker-play" {
			return errors.New("Poker is not accepting redraws")
		}
		if err := poker.redraw(request.Index); err != nil {
			return err
		}
		return s.completePokerTurn("Card discarded and redrawn.")
	case "poker-next-game":
		if session.phase != "poker-next-game" || poker.Rules.StreakCount == 0 {
			return errors.New("there is no next Poker game to start")
		}
		return s.startNextPokerGame(request.PlayerHeroID)
	default:
		return errors.New("unknown Poker action")
	}
}

func (s *webServer) completePokerTurn(message string) error {
	session := s.session
	poker := session.poker
	poker.Turns++
	ghostMessage, err := poker.advanceGhost()
	if err != nil {
		return err
	}
	if !poker.gameComplete() {
		session.message = message
		if ghostMessage != "" {
			session.message += " " + ghostMessage
		}
		session.message += " Choose your next action."
		return nil
	}
	if err := poker.finishGhost(); err != nil {
		return err
	}
	score, err := game.ScorePokerTable(poker.Slots, poker.Hero, poker.BountySuit, poker.Rules)
	if err != nil {
		return err
	}
	poker.LastScore = &score
	if score.ConstraintFailure != "" {
		poker.Streak = 0
		session.phase, session.winner = "gameover", 1
		s.finishSpecialCampaignMission(false)
		session.message = "Mission failed: " + score.ConstraintFailure
		return nil
	}
	if poker.Rules.StreakCount > 0 {
		if score.Total >= poker.Rules.StreakScore {
			poker.Streak++
		} else {
			poker.Streak = 0
		}
		if poker.Streak >= poker.Rules.StreakCount {
			session.phase, session.winner = "gameover", 0
			session.message = "Poker mission complete: three consecutive games reached the target."
			s.finishSpecialCampaignMission(true)
			return nil
		}
		session.phase, session.winner = "poker-next-game", -1
		session.message = fmt.Sprintf("Game %d/%d — need %d+.", poker.Streak+1, poker.Rules.StreakCount, poker.Rules.StreakScore)
		if score.Total < poker.Rules.StreakScore {
			session.message = fmt.Sprintf("Score %d was below %d; the streak resets. %s",
				score.Total, poker.Rules.StreakScore, session.message)
		} else {
			session.message += fmt.Sprintf(" Streak: %d/%d.", poker.Streak, poker.Rules.StreakCount)
		}
		session.message += " Continue with the same hero or switch heroes to reset the streak."
		return nil
	}
	won := score.Total >= poker.Rules.Par
	ghostResult := ""
	if poker.Ghost != nil {
		if poker.Ghost.LastScore == nil {
			return errors.New("Poker ghost score is unavailable")
		}
		ghostScore := poker.Ghost.LastScore.Total
		won = won && score.Total > ghostScore
		ghostResult = fmt.Sprintf(" Ghost score %d; you needed to beat it.", ghostScore)
	}
	session.phase, session.winner = "gameover", 1
	if won {
		session.winner = 0
		session.message = fmt.Sprintf("Poker mission complete: %d points (par %d).%s", score.Total, poker.Rules.Par, ghostResult)
	} else {
		session.message = fmt.Sprintf("Poker mission not completed: %d points (par %d).%s", score.Total, poker.Rules.Par, ghostResult)
	}
	s.finishSpecialCampaignMission(won)
	if won {
		session.message = fmt.Sprintf("Mission complete! Score %d (par %d).%s %s",
			score.Total, poker.Rules.Par, ghostResult, session.campaign.Result)
	} else {
		session.message = fmt.Sprintf("Mission not completed: %d points (par %d).%s",
			score.Total, poker.Rules.Par, ghostResult)
	}
	return nil
}

func (p *pokerSession) advanceGhost() (string, error) {
	if p.Ghost == nil || p.Ghost.LastScore != nil {
		return "", nil
	}
	card, slot, moved, err := p.Ghost.takeTurn(p)
	if err != nil {
		return "", err
	}
	if p.Ghost.gameComplete(p.Rules.Constraint) {
		if err := p.scoreGhost(); err != nil {
			return "", err
		}
	}
	if !moved {
		return "The teaching ghost has no legal placements remaining.", nil
	}
	return fmt.Sprintf("The teaching ghost placed %s in Hand %c.", card, 'A'+slot), nil
}

func (p *pokerSession) finishGhost() error {
	if p.Ghost == nil {
		return nil
	}
	for p.Ghost.LastScore == nil && !p.Ghost.gameComplete(p.Rules.Constraint) {
		_, _, moved, err := p.Ghost.takeTurn(p)
		if err != nil {
			return err
		}
		if !moved {
			break
		}
	}
	if p.Ghost.LastScore == nil {
		return p.scoreGhost()
	}
	return nil
}

func (p *pokerSession) scoreGhost() error {
	score, err := game.ScorePokerTable(p.Ghost.Slots, p.Hero, p.BountySuit, p.Rules)
	if err != nil {
		return err
	}
	p.Ghost.LastScore = &score
	return nil
}

func (s *webServer) startNextPokerGame(heroID string) error {
	session := s.session
	poker := session.poker
	if heroID == "" {
		heroID = poker.HeroID
	}
	hero, ok := heroes.ByID(heroID)
	if !ok || !s.heroUnlocked(heroID) {
		return fmt.Errorf("player hero %q is not unlocked", heroID)
	}
	changedHero := heroID != poker.HeroID
	if changedHero {
		poker.Streak = 0
	} else {
		hero.Bounty, hero.Favored, hero.Shift, hero.Draft = poker.Hero.Bounty, poker.Hero.Favored, poker.Hero.Shift, poker.Hero.Draft
	}
	if err := poker.startGame(heroID, hero, s.rng); err != nil {
		return err
	}
	session.playerHeroID = heroID
	session.phase, session.winner = "poker-play", -1
	if poker.Rules.StreakCount > 0 {
		session.message = fmt.Sprintf("Game %d/%d — need %d+.", poker.Streak+1, poker.Rules.StreakCount, poker.Rules.StreakScore)
		if changedHero {
			session.message = fmt.Sprintf("Hero changed; the streak resets. %s", session.message)
		}
	} else {
		session.message = fmt.Sprintf("Game %d. Place cards into three hands or redraw one card.", poker.GamesPlayed)
	}
	if s.playerProfile == nil {
		return errors.New("player profile is unavailable")
	}
	if err := s.playerProfile.Save(); err != nil {
		return err
	}
	return nil
}
