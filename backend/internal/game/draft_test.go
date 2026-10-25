package game

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"prisonersofwar/internal/modifiers"
)

func TestNewDraftGameCountsAndPickOrder(t *testing.T) {
	state, err := NewDraftGameWithModifiers(rand.New(rand.NewSource(12)),
		GameModifiers{Reckoning: modifiers.DefaultReckoning})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	if state.Draft.Mode != 3 || len(state.Draft.DealtX) != 2 ||
		len(state.Draft.DealtY) != 2 || len(state.Draft.Pool) != 8 {
		t.Fatalf("draft deal sizes X=%d Y=%d pool=%d", len(state.Draft.DealtX), len(state.Draft.DealtY), len(state.Draft.Pool))
	}
	want := []Seat{SeatY, SeatX, SeatY, SeatX, SeatY, SeatX}
	if !reflect.DeepEqual(state.Draft.PickOrder, want) {
		t.Fatalf("pick order = %v, want %v", state.Draft.PickOrder, want)
	}
}

func TestDraftModesProduceExpectedDealsAndPicks(t *testing.T) {
	tests := []struct {
		mode, dealtX, dealtY, pool, picks int
	}{
		{mode: 1, dealtX: 4, dealtY: 4, pool: 4, picks: 2},
		{mode: 2, dealtX: 3, dealtY: 3, pool: 6, picks: 4},
		{mode: 3, dealtX: 2, dealtY: 2, pool: 8, picks: 6},
		{mode: 4, dealtX: 1, dealtY: 1, pool: 10, picks: 8},
		{mode: 5, dealtX: 0, dealtY: 0, pool: 12, picks: 10},
	}
	if _, err := NewDraftGameWithDraftMode(
		rand.New(rand.NewSource(99)),
		GameModifiers{Reckoning: modifiers.DefaultReckoning}, 0); err == nil {
		t.Fatal("draft mode 0 should be rejected")
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("mode-%d", test.mode), func(t *testing.T) {
			state, err := NewDraftGameWithDraftMode(
				rand.New(rand.NewSource(int64(100+test.mode))),
				GameModifiers{Reckoning: modifiers.DefaultReckoning}, test.mode)
			if err != nil {
				t.Fatal(err)
			}
			if err := state.Validate(); err != nil {
				t.Fatal(err)
			}
			if state.DraftMode != test.mode {
				t.Fatalf("draft mode = %d, want %d", state.DraftMode, test.mode)
			}
			if len(state.Draft.DealtX) != test.dealtX || len(state.Draft.DealtY) != test.dealtY ||
				len(state.Draft.Pool) != test.pool || len(state.Draft.PickOrder) != test.picks {
				t.Fatalf("mode %d deal/pool/pick sizes = %d/%d/%d/%d; want %d/%d/%d/%d",
					test.mode, len(state.Draft.DealtX), len(state.Draft.DealtY),
					len(state.Draft.Pool), len(state.Draft.PickOrder),
					test.dealtX, test.dealtY, test.pool, test.picks)
			}
			for !state.Draft.Finished {
				if _, _, err := state.Draft.Pick(0); err != nil {
					t.Fatal(err)
				}
				if state.Draft.CurrentPick == len(state.Draft.PickOrder) {
					break
				}
			}
			if err := state.CompleteDraft(); err != nil {
				t.Fatal(err)
			}
			if len(state.P1.Hand) != 5 || len(state.P2.Hand) != 5 ||
				len(state.Deck) != 14 || len(state.Discard) != 2 {
				t.Fatalf("mode %d final hands/draw/discard = %d/%d/%d/%d, want 5/5/14/2",
					test.mode, len(state.P1.Hand), len(state.P2.Hand),
					len(state.Deck), len(state.Discard))
			}
		})
	}
}

func TestDraftCompletionAccountsForAllCardsAndIsDeterministic(t *testing.T) {
	run := func(seed int64) (GameState, error) {
		state, err := NewDraftGameWithModifiers(rand.New(rand.NewSource(seed)),
			GameModifiers{Reckoning: modifiers.DefaultReckoning})
		if err != nil {
			return GameState{}, err
		}
		for state.Draft.CurrentPick < len(state.Draft.PickOrder) {
			if _, _, err := state.Draft.Pick(0); err != nil {
				return GameState{}, err
			}
		}
		if err := state.CompleteDraft(); err != nil {
			return GameState{}, err
		}
		return state, nil
	}
	first, err := run(44)
	if err != nil {
		t.Fatal(err)
	}
	second, err := run(44)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("fixed random seed produced different completed draft states")
	}
	x, _ := first.PlayerAtSeat(SeatX)
	y, _ := first.PlayerAtSeat(SeatY)
	xPlayer, _ := first.Player(x)
	yPlayer, _ := first.Player(y)
	if len(xPlayer.Hand) != 5 || len(yPlayer.Hand) != 5 || len(first.Deck) != 14 ||
		len(first.Discard) != 2 {
		t.Fatalf("final cards X=%d Y=%d draw pile=%d removed=%d; want 5/5/14/2",
			len(xPlayer.Hand), len(yPlayer.Hand), len(first.Deck), len(first.Discard))
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("completed draft state is invalid: %v", err)
	}
	removed := append([]Card(nil), first.Discard...)
	for round := 0; round < 2; round++ {
		if err := EndRound(&first, round == 0, true); err != nil {
			t.Fatalf("end round %d: %v", round+1, err)
		}
	}
	for _, card := range removed {
		for _, cards := range [][]Card{first.Deck, first.P1.Hand, first.P2.Hand, first.P1.Pool,
			first.P2.Pool, first.P1.Jail, first.P2.Jail} {
			for _, dealt := range cards {
				if dealt == card {
					t.Fatalf("removed draft card %s returned to play", card)
				}
			}
		}
	}
}

type firstCardDraftUI struct{}

func (firstCardDraftUI) ShowDraftStart(_, _, _ []Card, _ Seat) {}
func (firstCardDraftUI) ChooseDraftPick(_ Seat, pool, _, _ []Card, _, _, _ int) int {
	return 0
}
func (firstCardDraftUI) ShowDraftPickResult(_ Seat, _ Card, _ []Card) {}
func (firstCardDraftUI) ShowDraftComplete(_, _ []Card)                {}

func TestRunDraftValidatesPicksAndReturnsExpectedCounts(t *testing.T) {
	deck := NewDeck()[1:]
	result, err := RunDraft(deck, firstCardDraftUI{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.XHand) != 5 || len(result.YHand) != 5 || len(result.DrawPile) != 14 ||
		len(result.Removed) != 2 {
		t.Fatalf("draft result sizes X=%d Y=%d draw pile=%d removed=%d; want 5/5/14/2",
			len(result.XHand), len(result.YHand), len(result.DrawPile), len(result.Removed))
	}
	if _, err := RunDraft(deck, nil); err == nil {
		t.Fatal("draft without UI should fail")
	}
}
