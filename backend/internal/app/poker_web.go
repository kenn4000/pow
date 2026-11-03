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
	DrawPile    []game.Card
	DiscardPile []game.Card
	Hand        []game.Card
	Slots       [pokerSlotCount][]game.Card
	Turns       int
	GamesPlayed int
	Streak      int
	LastScore   *game.PokerTableScore
}

type webPokerHeroStats struct {
	Bounty  int `json:"bounty"`
	Favored int `json:"favored"`
	Shift   int `json:"shift"`
	Draft   int `json:"draft"`
}

type webPokerState struct {
	HeroID           string                      `json:"heroId"`
	HeroName         string                      `json:"heroName"`
	Hero             webPokerHeroStats           `json:"hero"`
	BountySuit       game.Suit                   `json:"bountySuit"`
	DeckFloor        int                         `json:"deckFloor"`
	DrawPileCount    int                         `json:"drawPileCount"`
	DiscardPileCount int                         `json:"discardPileCount"`
	DiscardTop       *game.Card                  `json:"discardTop,omitempty"`
	Hand             []webCard                   `json:"hand"`
	Slots            [pokerSlotCount][]webCard   `json:"slots"`
	Turns            int                         `json:"turns"`
	TurnLimit        int                         `json:"turnLimit"`
	GameNumber       int                         `json:"gameNumber"`
	Streak           int                         `json:"streak"`
	StreakCount      int                         `json:"streakCount"`
	StreakScore      int                         `json:"streakScore"`
	Par              int                         `json:"par"`
	NineBase         int                         `json:"nineBase"`
	BountyBase       int                         `json:"bountyBase"`
	Constraint       game.PokerMissionConstraint `json:"constraint,omitempty"`
	CanRedraw        bool                        `json:"canRedraw"`
	CanPlace         [pokerSlotCount]bool        `json:"canPlace"`
	Score            *game.PokerTableScore       `json:"score,omitempty"`
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
	deck := make([]game.Card, 0, len(game.NewDeck()))
	for _, card := range game.NewDeck() {
		if card.Face >= deckFloor {
			deck = append(deck, card)
		}
	}
	if len(deck) < pokerHandSize {
		return errors.New("hero Draft leaves too few cards for a poker hand")
	}
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	suits := [...]game.Suit{game.Blade, game.Fang, game.Shield}
	p.HeroID = heroID
	p.Hero = game.PokerHeroParameters{
		Bounty: hero.Bounty, Favored: hero.Favored, Shift: hero.Shift, Draft: hero.Draft,
	}
	p.DeckFloor = deckFloor
	p.BountySuit = suits[rng.Intn(len(suits))]
	p.DrawPile = deck
	p.Hand = make([]game.Card, 0, pokerHandSize)
	p.DiscardPile = nil
	p.Slots = [pokerSlotCount][]game.Card{}
	p.Turns = 0
	p.GamesPlayed++
	p.LastScore = nil
	for len(p.Hand) < pokerHandSize {
		p.Hand = append(p.Hand, p.drawCard())
	}
	return nil
}

func (p *pokerSession) drawCard() game.Card {
	top := len(p.DrawPile) - 1
	card := p.DrawPile[top]
	p.DrawPile = p.DrawPile[:top]
	return card
}

func (p *pokerSession) webView(phase string) *webPokerState {
	result := &webPokerState{
		HeroID: p.HeroID, HeroName: heroName(p.HeroID),
		Hero: webPokerHeroStats{
			Bounty: p.Hero.Bounty, Favored: p.Hero.Favored,
			Shift: p.Hero.Shift, Draft: p.Hero.Draft,
		},
		BountySuit: p.BountySuit, DeckFloor: p.DeckFloor,
		DrawPileCount: len(p.DrawPile), DiscardPileCount: len(p.DiscardPile),
		Turns: p.Turns, TurnLimit: pokerTurnLimit, GameNumber: p.GamesPlayed,
		Streak: p.Streak, StreakCount: p.Rules.StreakCount,
		StreakScore: p.Rules.StreakScore, Par: p.Rules.Par,
		NineBase: p.Rules.NineBase, BountyBase: p.Rules.BountyBase,
		Constraint: p.Rules.Constraint,
	}
	if len(p.DiscardPile) > 0 {
		top := p.DiscardPile[len(p.DiscardPile)-1]
		result.DiscardTop = &top
	}
	result.Hand = make([]webCard, len(p.Hand))
	for index, card := range p.Hand {
		result.Hand[index] = webCard{
			Card: card, Index: index, Selectable: phase == "poker-play",
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
	if !game.CanPlacePokerCard(p.Slots, slotIndex) {
		return errors.New("Poker hands must stay within one card of each other")
	}
	card := p.Hand[cardIndex]
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
	for len(p.Hand) < pokerHandSize && len(p.DrawPile) > 0 {
		p.Hand = append(p.Hand, p.drawCard())
	}
}

func (p *pokerSession) canPlace() bool {
	if len(p.Hand) == 0 {
		return false
	}
	for slotIndex := range p.Slots {
		if game.CanPlacePokerCard(p.Slots, slotIndex) {
			return true
		}
	}
	return false
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
	if !poker.gameComplete() {
		session.message = message + " Choose your next action."
		return nil
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
		if score.Total >= poker.Rules.StreakScore {
			session.message = fmt.Sprintf("Score %d reached the target. Streak: %d of %d. Continue with the same hero or switch heroes to reset the streak.",
				score.Total, poker.Streak, poker.Rules.StreakCount)
		} else {
			session.message = fmt.Sprintf("Score %d was below %d. The streak resets; play another game to continue.",
				score.Total, poker.Rules.StreakScore)
		}
		return nil
	}
	won := score.Total >= poker.Rules.Par
	session.phase, session.winner = "gameover", 1
	if won {
		session.winner = 0
		session.message = fmt.Sprintf("Poker mission complete: %d points (par %d).", score.Total, poker.Rules.Par)
	} else {
		session.message = fmt.Sprintf("Poker mission not completed: %d points (par %d).", score.Total, poker.Rules.Par)
	}
	s.finishSpecialCampaignMission(won)
	if won {
		session.message = fmt.Sprintf("Mission complete! Score %d (par %d). %s",
			score.Total, poker.Rules.Par, session.campaign.Result)
	} else {
		session.message = fmt.Sprintf("Mission not completed: %d points (par %d).",
			score.Total, poker.Rules.Par)
	}
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
	if heroID != poker.HeroID {
		poker.Streak = 0
	}
	if err := poker.startGame(heroID, hero, s.rng); err != nil {
		return err
	}
	session.playerHeroID = heroID
	session.phase, session.winner = "poker-play", -1
	session.message = fmt.Sprintf("Game %d. Place cards into three hands or redraw one card.", poker.GamesPlayed)
	if s.playerProfile == nil {
		return errors.New("player profile is unavailable")
	}
	if err := s.playerProfile.Save(); err != nil {
		return err
	}
	return nil
}
