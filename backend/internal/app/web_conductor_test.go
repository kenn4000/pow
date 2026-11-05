package app

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func TestWebConductorTuningRequiresUnlockedSkillAndHonorsTier(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if tuning, err := webConductorTuning(p, "", 0); err != nil || len(tuning) != 0 {
		t.Fatalf("no-tuning selection = %v, %v", tuning, err)
	}
	if _, err := webConductorTuning(p, "bounty", 1); err == nil {
		t.Fatal("tuning was accepted without The Conductor")
	}
	p.SkillNodes["conductor-1"] = true
	if _, err := webConductorTuning(p, "bounty", 2); err == nil {
		t.Fatal("T1 Conductor accepted a ±2 tuning")
	}
	p.SkillNodes["conductor-2"] = true
	tuning, err := webConductorTuning(p, "shift", -2)
	if err != nil || tuning["shift"] != -2 || len(tuning) != 1 {
		t.Fatalf("Master Conductor tuning = %v, %v", tuning, err)
	}
	if _, err := webConductorTuning(p, "draft", 1); err == nil {
		t.Fatal("unsupported modifier was accepted")
	}
}

func TestApplyWebHeroTuningChangesOnlySelectedModifier(t *testing.T) {
	hero := heroes.Hero{Bounty: 1, Shift: 2, Favored: -1}
	got := applyWebHeroTuning(hero, heroes.Tuning{"bounty": -1})
	if got.Bounty != 0 || got.Shift != 2 || got.Favored != -1 {
		t.Fatalf("tuned hero = %+v", got)
	}
}

func TestNewWebGameAppliesConductorTuning(t *testing.T) {
	web := campaignTestServer(t)
	web.playerProfile.SkillNodes["conductor-1"] = true
	response := callDashboardHandler(t, web.handleNewGame, http.MethodPost, "/api/new",
		map[string]any{
			"mode": "easy", "variant": "main", "playerHeroId": "recruit",
			"conductorModifier": "shift", "conductorAmount": 1,
		})
	if response.Code != http.StatusOK {
		t.Fatalf("new game status = %d: %s", response.Code, response.Body.String())
	}
	if web.session.state.Modifiers.Shift != 1 {
		t.Fatalf("new game Shift modifier = %d, want 1", web.session.state.Modifiers.Shift)
	}

	response = callDashboardHandler(t, web.handleNewGame, http.MethodPost, "/api/new",
		map[string]any{
			"mode": "easy", "variant": "main", "playerHeroId": "recruit",
			"conductorModifier": "shift", "conductorAmount": 2,
		})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid Conductor tuning status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestOnlineBattleSnapshotPreservesConductorTunings(t *testing.T) {
	session := &webSession{
		mode: "hvh", variant: "main",
		conductorTunings: [2]heroes.Tuning{
			{"bounty": 1},
			{"favored": -2},
		},
	}
	data, err := encodeOnlineBattleState(session)
	if err != nil {
		t.Fatal(err)
	}
	var saved onlineBattleState
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	restored := saved.Session.restore()
	if !reflect.DeepEqual(restored.conductorTunings, session.conductorTunings) {
		t.Fatalf("restored Conductor tunings = %v, want %v",
			restored.conductorTunings, session.conductorTunings)
	}
}
