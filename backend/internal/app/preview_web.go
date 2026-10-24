package app

import (
	"errors"
	"net/http"
)

// handlePreviewReset discards the dashboard preview's temporary profile copy and
// any preview-only Camp Draw state, so the next ?preview=dashboard request starts
// from a fresh copy of the real profile. The real profile is never modified.
func (s *webServer) handlePreviewReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previewProfile = nil
	if s.campDrawPreview {
		s.campDrawSession, s.campDrawPreview = nil, false
	}
	if s.lastCampDrawPreview {
		s.lastCampDraw, s.lastCampDrawPreview = nil, false
	}
	writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
}
