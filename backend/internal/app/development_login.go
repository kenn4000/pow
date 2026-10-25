package app

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
)

// developmentLoginEnabled reports whether the passwordless account picker
// (GET /api/auth/users, POST /api/auth/assume) is available. It is off unless
// DEVELOPMENT_LOGIN=true, because it lets anyone sign in as any account,
// including administrators.
func developmentLoginEnabled() bool {
	return os.Getenv("DEVELOPMENT_LOGIN") == "true"
}

func (s *webServer) handleDevelopmentLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID string `json:"userId"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var user account
	err = tx.QueryRowContext(r.Context(), "SELECT id,email,role,public_handle,created_at FROM users WHERE id=$1", input.UserID).Scan(&user.ID, &user.Email, &user.Role, &user.PublicHandle, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("account no longer exists; refresh the account list"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	token, err := finishAuthentication(r.Context(), tx, user)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	http.SetCookie(w, sessionCookie(token, 7*24*60*60))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}
