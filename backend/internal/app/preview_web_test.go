package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
)

func TestPreviewResetClearsOnlyPreviewState(t *testing.T) {
	real := &profile.Profile{Marks: 100}
	realDraw := &lootbox.CampDrawResult{BoxType: "gold"}
	web := &webServer{playerProfile: real, previewProfile: &profile.Profile{Marks: 999}, lastCampDraw: realDraw}
	recorder := httptest.NewRecorder()
	web.handlePreviewReset(recorder, httptest.NewRequest(http.MethodPost, "/api/preview/reset", nil))
	if recorder.Code != http.StatusOK || web.previewProfile != nil {
		t.Fatalf("status %d, preview profile %+v; want 200 and cleared", recorder.Code, web.previewProfile)
	}
	if web.playerProfile != real || real.Marks != 100 || web.lastCampDraw != realDraw {
		t.Fatal("reset must not touch the real profile or a real Camp Draw result")
	}
	web.lastCampDraw, web.lastCampDrawPreview = &lootbox.CampDrawResult{}, true
	web.handlePreviewReset(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/preview/reset", nil))
	if web.lastCampDraw != nil {
		t.Fatal("reset must clear a preview Camp Draw result")
	}
}
