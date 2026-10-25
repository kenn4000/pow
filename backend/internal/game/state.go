package game

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/internal/modifiers"
)

type Seat int

const (
	SeatX Seat = iota
	SeatY
)

func (s Seat) String() string {
	if s == SeatX {
		return "X"
	}
	if s == SeatY {
		return "Y"
	}
	return "?"
}

type Player struct {
	Hand            []Card    `json:"hand"`
	Jail            []Card    `json:"jail"`
	Pool            []Card    `json:"pool"`
	Seat            Seat      `json:"seat"`
	CapturedByRound [3][3]int `json:"captured_by_round"`
}

type GameModifiers = modifiers.GameModifiers

type GameState struct {
	Deck      []Card        `json:"deck"`
	Discard   []Card        `json:"discard"`
	Draft     *DraftState   `json:"draft,omitempty"`
	Bounty    Card          `json:"bounty"`
	Modifiers GameModifiers `json:"modifiers"`
	DraftMode int           `json:"draft_mode"`
	P1        Player        `json:"p1"`
	P2        Player        `json:"p2"`
	Round     int           `json:"round"`
	Log       []string      `json:"log,omitempty"`
	Clashes   int           `json:"clashes"`
}

var (
	ErrInvalidPlayer   = errors.New("player index must be 0 or 1")
	ErrInvalidCommit   = errors.New("commit must contain exactly three distinct hand cards")
	ErrInvalidPoolCard = errors.New("selected card is not available in the pool")
)

func NewGame(rng *rand.Rand) (GameState, error) {
	return NewGameWithModifiers(rng, GameModifiers{Reckoning: modifiers.DefaultReckoning})
}

func NewGameWithModifiers(rng *rand.Rand, gameModifiers GameModifiers) (GameState, error) {
	if rng == nil {
		return GameState{}, errors.New("random source is required")
	}
	if err := gameModifiers.Validate(); err != nil {
		return GameState{}, fmt.Errorf("invalid game modifiers: %w", err)
	}
	deck := NewDeck()
	Shuffle(deck, rng)
	state := GameState{Bounty: deck[0], Deck: deck[1:], Round: 1, Modifiers: gameModifiers}
	if rng.Intn(2) == 0 {
		state.P1.Seat, state.P2.Seat = SeatX, SeatY
	} else {
		state.P1.Seat, state.P2.Seat = SeatY, SeatX
	}
	draw(&state, &state.P1.Hand, 5)
	draw(&state, &state.P2.Hand, 5)
	if err := state.Validate(); err != nil {
		return GameState{}, err
	}
	return state, nil
}

func NewDraftGameWithModifiers(rng *rand.Rand, gameModifiers GameModifiers) (GameState, error) {
	return NewDraftGameWithDraftMode(rng, gameModifiers, 3)
}

func NewDraftGameWithDraftMode(rng *rand.Rand, gameModifiers GameModifiers, draftMode int) (GameState, error) {
	if rng == nil {
		return GameState{}, errors.New("random source is required")
	}
	if err := gameModifiers.Validate(); err != nil {
		return GameState{}, fmt.Errorf("invalid game modifiers: %w", err)
	}
	if draftMode < 1 || draftMode > 5 {
		return GameState{}, fmt.Errorf("draft mode must be between 1 and 5, got %d", draftMode)
	}
	deck := NewDeck()
	Shuffle(deck, rng)
	state := GameState{
		Bounty: deck[0], Deck: append([]Card(nil), deck[1:]...),
		Round: 1, Modifiers: gameModifiers, DraftMode: draftMode,
	}
	if rng.Intn(2) == 0 {
		state.P1.Seat, state.P2.Seat = SeatX, SeatY
	} else {
		state.P1.Seat, state.P2.Seat = SeatY, SeatX
	}
	draft, err := NewDraftWithMode(deck[1:], draftMode)
	if err != nil {
		return GameState{}, err
	}
	state.Draft = draft
	state.Deck = nil
	if err := state.Validate(); err != nil {
		return GameState{}, err
	}
	return state, nil
}

func (s *GameState) CompleteDraft() error {
	if s == nil || s.Draft == nil {
		return errors.New("game has no active draft")
	}
	var result DraftResult
	if s.Draft.Finished {
		result = DraftResult{
			XHand:    append(append([]Card(nil), s.Draft.DealtX...), s.Draft.PickedX...),
			YHand:    append(append([]Card(nil), s.Draft.DealtY...), s.Draft.PickedY...),
			DrawPile: append([]Card(nil), s.Draft.DrawPile...),
			Removed:  append([]Card(nil), s.Draft.Removed...),
		}
	} else {
		var err error
		result, err = s.Draft.Finish()
		if err != nil {
			return err
		}
	}
	x, err := s.PlayerAtSeat(SeatX)
	if err != nil {
		return err
	}
	y, err := s.PlayerAtSeat(SeatY)
	if err != nil {
		return err
	}
	xPlayer, _ := s.Player(x)
	yPlayer, _ := s.Player(y)
	xPlayer.Hand = result.XHand
	yPlayer.Hand = result.YHand
	s.Deck = result.DrawPile
	s.Discard = append(s.Discard, result.Removed...)
	s.Draft = nil
	return s.Validate()
}

func (s *GameState) Player(index int) (*Player, error) {
	switch index {
	case 0:
		return &s.P1, nil
	case 1:
		return &s.P2, nil
	default:
		return nil, ErrInvalidPlayer
	}
}

func (s GameState) Players() [2]Player { return [2]Player{s.P1, s.P2} }

func (s GameState) PlayerAtSeat(seat Seat) (int, error) {
	if s.P1.Seat == seat {
		return 0, nil
	}
	if s.P2.Seat == seat {
		return 1, nil
	}
	return 0, errors.New("seat is not assigned")
}

func (s GameState) Validate() error {
	if !s.Bounty.Valid() {
		return fmt.Errorf("invalid bounty card %v", s.Bounty)
	}
	if s.DraftMode < 0 || s.DraftMode > 5 {
		return fmt.Errorf("invalid draft mode %d", s.DraftMode)
	}
	if s.Draft != nil && (s.DraftMode == 0 || s.Draft.Mode != s.DraftMode) {
		return errors.New("draft state mode does not match game draft mode")
	}
	if err := s.Modifiers.Validate(); err != nil {
		return fmt.Errorf("invalid game modifiers: %w", err)
	}
	if s.Round < 1 || s.Round > 3 {
		return fmt.Errorf("invalid round %d", s.Round)
	}
	if s.P1.Seat == s.P2.Seat || (s.P1.Seat != SeatX && s.P1.Seat != SeatY) ||
		(s.P2.Seat != SeatX && s.P2.Seat != SeatY) {
		return errors.New("players must occupy opposite seats")
	}
	if err := s.validateCardAccounting(); err != nil {
		return err
	}
	if err := validateCaptureCounts(s.P1); err != nil {
		return fmt.Errorf("player 1 capture history: %w", err)
	}
	if err := validateCaptureCounts(s.P2); err != nil {
		return fmt.Errorf("player 2 capture history: %w", err)
	}
	return nil
}

func (s GameState) validateCardAccounting() error {
	seen := make(map[Card]bool, 27)
	add := func(card Card) error {
		if !card.Valid() {
			return fmt.Errorf("invalid card %v", card)
		}
		if seen[card] {
			return fmt.Errorf("duplicate card %s", card)
		}
		seen[card] = true
		return nil
	}
	for _, card := range []Card{s.Bounty} {
		if err := add(card); err != nil {
			return err
		}
	}
	for _, cards := range [][]Card{s.Deck, s.Discard, s.P1.Hand, s.P1.Jail, s.P1.Pool, s.P2.Hand, s.P2.Jail, s.P2.Pool} {
		for _, card := range cards {
			if err := add(card); err != nil {
				return err
			}
		}
	}
	if s.Draft != nil {
		if err := s.Draft.validate(); err != nil {
			return fmt.Errorf("opening draft: %w", err)
		}
		for _, cards := range [][]Card{s.Draft.Pool, s.Draft.PickedX, s.Draft.PickedY,
			s.Draft.DealtX, s.Draft.DealtY, s.Draft.DrawPile, s.Draft.Removed} {
			for _, card := range cards {
				if err := add(card); err != nil {
					return err
				}
			}
		}
	}
	if len(seen) != 27 {
		return fmt.Errorf("card accounting has %d of 27 cards", len(seen))
	}
	return nil
}

func validateCaptureCounts(player Player) error {
	var bySuit [3]int
	for round, counts := range player.CapturedByRound {
		for suit, count := range counts {
			if count < 0 {
				return fmt.Errorf("round %d has a negative capture count", round+1)
			}
			bySuit[suit] += count
		}
	}
	var jailBySuit [3]int
	for _, card := range player.Jail {
		jailBySuit[int(card.Suit)]++
	}
	if bySuit != jailBySuit {
		return fmt.Errorf("capture counts %v do not match jail cards %v", bySuit, jailBySuit)
	}
	return nil
}

func targetForSeat(seat Seat) int {
	if seat == SeatX {
		return 6
	}
	return 5
}
