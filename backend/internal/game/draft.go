package game

import (
	"errors"
	"fmt"
)

type DraftState struct {
	Mode        int    `json:"mode"`
	Pool        []Card `json:"pool"`
	PickedX     []Card `json:"picked_x"`
	PickedY     []Card `json:"picked_y"`
	DealtX      []Card `json:"dealt_x"`
	DealtY      []Card `json:"dealt_y"`
	PickOrder   []Seat `json:"pick_order"`
	CurrentPick int    `json:"current_pick"`
	Finished    bool   `json:"finished"`
	DrawPile    []Card `json:"draw_pile"`
	Removed     []Card `json:"removed"`
}

type DraftResult struct {
	XHand    []Card
	YHand    []Card
	DrawPile []Card
	Removed  []Card
}

type DraftUI interface {
	ShowDraftStart(dealtX, dealtY, pool []Card, viewingSeat Seat)
	ChooseDraftPick(viewingSeat Seat, pool, myHand, myDrafted []Card, opponentDraftedCount, pickNumber, totalPicks int) int
	ShowDraftPickResult(picker Seat, card Card, remainingPool []Card)
	ShowDraftComplete(xHand, yHand []Card)
}

func NewDraft(deck []Card) (*DraftState, error) {
	return NewDraftWithMode(deck, 3)
}

func NewDraftWithMode(deck []Card, mode int) (*DraftState, error) {
	if len(deck) != 26 {
		return nil, fmt.Errorf("opening draft requires 26 cards after the bounty, got %d", len(deck))
	}
	if mode < 1 || mode > 5 {
		return nil, fmt.Errorf("draft mode must be between 1 and 5, got %d", mode)
	}
	dealtEach := 5 - mode
	poolSize := 2 + 2*mode
	pickOrder := make([]Seat, 2*mode)
	for pick := range pickOrder {
		if pick%2 == 0 {
			pickOrder[pick] = SeatY
		} else {
			pickOrder[pick] = SeatX
		}
	}
	draft := &DraftState{
		Mode:      mode,
		DealtX:    append([]Card(nil), deck[:dealtEach]...),
		DealtY:    append([]Card(nil), deck[dealtEach:2*dealtEach]...),
		Pool:      append([]Card(nil), deck[2*dealtEach:2*dealtEach+poolSize]...),
		DrawPile:  append([]Card(nil), deck[12:]...),
		PickOrder: pickOrder,
	}
	if err := draft.validate(); err != nil {
		return nil, err
	}
	return draft, nil
}

func (d *DraftState) CurrentSeat() (Seat, bool) {
	if d == nil || d.Finished || d.CurrentPick < 0 || d.CurrentPick >= len(d.PickOrder) {
		return 0, false
	}
	return d.PickOrder[d.CurrentPick], true
}

func (d *DraftState) Pick(index int) (Card, Seat, error) {
	if d == nil || d.Finished {
		return Card{}, 0, errors.New("draft is not active")
	}
	if err := d.validate(); err != nil {
		return Card{}, 0, fmt.Errorf("validate draft before picking: %w", err)
	}
	seat, ok := d.CurrentSeat()
	if !ok {
		return Card{}, 0, errors.New("draft has no pending pick")
	}
	if index < 0 || index >= len(d.Pool) {
		return Card{}, 0, fmt.Errorf("draft pick index %d is outside the remaining pool", index)
	}
	card := take(&d.Pool, index)
	if seat == SeatX {
		d.PickedX = append(d.PickedX, card)
	} else {
		d.PickedY = append(d.PickedY, card)
	}
	d.CurrentPick++
	return card, seat, nil
}

func (d *DraftState) Finish() (DraftResult, error) {
	if d == nil {
		return DraftResult{}, errors.New("draft is required")
	}
	if d.Finished || d.CurrentPick != len(d.PickOrder) || len(d.Pool) != 2 {
		return DraftResult{}, errors.New("draft cannot finish before all picks are made")
	}
	if err := d.validate(); err != nil {
		return DraftResult{}, fmt.Errorf("validate draft before finishing: %w", err)
	}
	d.Removed = append([]Card(nil), d.Pool...)
	d.Pool = nil
	xHand := append(append([]Card(nil), d.DealtX...), d.PickedX...)
	yHand := append(append([]Card(nil), d.DealtY...), d.PickedY...)
	d.Finished = true
	if err := d.validate(); err != nil {
		return DraftResult{}, err
	}
	return DraftResult{
		XHand: xHand, YHand: yHand,
		DrawPile: append([]Card(nil), d.DrawPile...),
		Removed:  append([]Card(nil), d.Removed...),
	}, nil
}

func RunDraft(deck []Card, ui DraftUI) (DraftResult, error) {
	if ui == nil {
		return DraftResult{}, errors.New("draft UI is required")
	}
	draft, err := NewDraft(deck)
	if err != nil {
		return DraftResult{}, err
	}
	return draft.Run(ui)
}

func (d *DraftState) Run(ui DraftUI) (DraftResult, error) {
	if d == nil || ui == nil {
		return DraftResult{}, errors.New("draft state and UI are required")
	}
	if d.Finished {
		return DraftResult{}, errors.New("draft is already finished")
	}
	if err := d.validate(); err != nil {
		return DraftResult{}, fmt.Errorf("validate draft before running: %w", err)
	}
	firstSeat, _ := d.CurrentSeat()
	ui.ShowDraftStart(append([]Card(nil), d.DealtX...), append([]Card(nil), d.DealtY...),
		append([]Card(nil), d.Pool...), firstSeat)
	for !d.Finished && d.CurrentPick < len(d.PickOrder) {
		seat, _ := d.CurrentSeat()
		hand, drafted, opponentDrafted := d.privateDraftView(seat)
		index := ui.ChooseDraftPick(seat, append([]Card(nil), d.Pool...),
			hand, drafted, opponentDrafted, d.CurrentPick+1, len(d.PickOrder))
		card, picker, err := d.Pick(index)
		if err != nil {
			return DraftResult{}, err
		}
		ui.ShowDraftPickResult(picker, card, append([]Card(nil), d.Pool...))
	}
	result, err := d.Finish()
	if err != nil {
		return DraftResult{}, err
	}
	ui.ShowDraftComplete(result.XHand, result.YHand)
	return result, nil
}

func (d *DraftState) privateDraftView(seat Seat) (hand, drafted []Card, opponentDrafted int) {
	if seat == SeatX {
		hand = append([]Card(nil), d.DealtX...)
		drafted = append([]Card(nil), d.PickedX...)
		opponentDrafted = len(d.PickedY)
	} else {
		hand = append([]Card(nil), d.DealtY...)
		drafted = append([]Card(nil), d.PickedY...)
		opponentDrafted = len(d.PickedX)
	}
	return hand, drafted, opponentDrafted
}

func (d *DraftState) validate() error {
	if d == nil || d.Mode < 1 || d.Mode > 5 || len(d.PickOrder) != 2*d.Mode ||
		d.CurrentPick < 0 || d.CurrentPick > len(d.PickOrder) {
		return errors.New("invalid draft state")
	}
	if len(d.DealtX) != 5-d.Mode || len(d.DealtY) != 5-d.Mode {
		return errors.New("draft deal does not match the selected mode")
	}
	pickedX, pickedY := 0, 0
	for _, seat := range d.PickOrder[:d.CurrentPick] {
		if seat == SeatX {
			pickedX++
		} else {
			pickedY++
		}
	}
	if len(d.PickedX) != pickedX || len(d.PickedY) != pickedY ||
		(!d.Finished && len(d.Pool) != 2+2*d.Mode-d.CurrentPick) {
		return errors.New("draft picks do not match the pick counter")
	}
	if d.Finished && (d.CurrentPick != len(d.PickOrder) || len(d.Pool) != 0 ||
		len(d.PickedX) != d.Mode || len(d.PickedY) != d.Mode || len(d.DrawPile) != 14 ||
		len(d.Removed) != 2) {
		return errors.New("finished draft has invalid card counts")
	}
	if !d.Finished && len(d.DrawPile) != 14 {
		return errors.New("unfinished draft draw pile must contain 14 cards")
	}
	if !d.Finished && len(d.Removed) != 0 {
		return errors.New("unfinished draft cannot have removed cards")
	}
	seen := make(map[Card]bool, 26)
	for _, cards := range [][]Card{d.DealtX, d.DealtY, d.Pool, d.PickedX, d.PickedY,
		d.DrawPile, d.Removed} {
		for _, card := range cards {
			if !card.Valid() || seen[card] {
				return fmt.Errorf("draft contains invalid or duplicate card %v", card)
			}
			seen[card] = true
		}
	}
	if len(seen) != 26 {
		return fmt.Errorf("draft accounts for %d of 26 cards", len(seen))
	}
	firstSeat := d.PickOrder[0]
	for i, seat := range d.PickOrder {
		want := firstSeat
		if i%2 == 1 {
			if firstSeat == SeatX {
				want = SeatY
			} else {
				want = SeatX
			}
		}
		if seat != want {
			return errors.New("invalid draft pick order")
		}
	}
	return nil
}
