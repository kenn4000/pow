package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"testing"

	"prisonersofwar/game"
)

func TestAssignInitialSeatsPreservesCardAccounting(t *testing.T) {
	for _, seat := range []game.Seat{game.SeatX, game.SeatY} {
		state, err := game.NewGame(rand.New(rand.NewSource(int64(seat + 80))))
		if err != nil {
			t.Fatal(err)
		}
		if err := assignInitialSeats(&state, seat); err != nil {
			t.Fatal(err)
		}
		wantP1 := 5
		if seat == game.SeatX {
			wantP1 = 6
		}
		if len(state.P1.Hand) != wantP1 || len(state.P2.Hand) != 11-wantP1 {
			t.Fatalf("seat %s hand sizes = %d/%d", seat, len(state.P1.Hand), len(state.P2.Hand))
		}
		if err := state.Validate(); err != nil {
			t.Fatalf("state after assigning seat %s: %v", seat, err)
		}
	}
}

func TestDumpEconomyProducesAuditableJSON(t *testing.T) {
	var output bytes.Buffer
	if err := dumpEconomyConfig(&output); err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(output.Bytes(), &config); err != nil {
		t.Fatalf("dump is not JSON: %v", err)
	}
	for _, section := range []string{"payouts", "energy", "camp_draw_and_catalog", "skills", "chromas", "bundles", "achievements", "achievement_sets"} {
		if _, ok := config[section]; !ok {
			t.Errorf("dump is missing %q", section)
		}
	}
}
