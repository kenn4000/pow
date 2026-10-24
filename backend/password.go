package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type requestError struct {
	status  int
	message string
}

func (e *requestError) Error() string { return e.message }

func writeRequestError(w http.ResponseWriter, err error) {
	var public *requestError
	if errors.As(err, &public) {
		writeError(w, public.status, public)
		return
	}
	databaseError(w, err)
}

type passwordChange struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *webServer) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u, _ := r.Context().Value(accountKey).(account)
	var input passwordChange
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !validPassword(input.NewPassword) || !validPassword(input.CurrentPassword) {
		writeError(w, http.StatusBadRequest, errors.New("passwords must be between 12 and 72 bytes"))
		return
	}
	if input.CurrentPassword == input.NewPassword {
		writeError(w, http.StatusBadRequest, errors.New("choose a different new password"))
		return
	}
	if !s.allowAuthAttempt(w, r, u.Email) {
		return
	}
	cookie, err := r.Cookie("pow_session")
	if err != nil {
		writeError(w, http.StatusUnauthorized, errors.New("sign in to continue"))
		return
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	token, err := s.changePassword(r.Context(), u, input, hash[:])
	if err != nil {
		writeRequestError(w, err)
		return
	}
	http.SetCookie(w, sessionCookie(token, 7*24*60*60))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *webServer) changePassword(ctx context.Context, u account, input passwordChange, sessionHash []byte) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var previous []byte
	err = tx.QueryRowContext(ctx, `SELECT u.password_hash FROM users u JOIN auth_sessions s ON s.user_id=u.id WHERE u.id=$1 AND s.token_hash=$2 AND s.expires_at>now() FOR UPDATE OF u`, u.ID, sessionHash).Scan(&previous)
	if errors.Is(err, sql.ErrNoRows) {
		return "", &requestError{http.StatusUnauthorized, "sign in to continue"}
	}
	if err != nil {
		return "", err
	}
	if bcrypt.CompareHashAndPassword(previous, []byte(input.CurrentPassword)) != nil {
		return "", &requestError{http.StatusBadRequest, "current password is incorrect"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET password_hash=$1 WHERE id=$2", hash, u.ID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM auth_sessions WHERE user_id=$1", u.ID); err != nil {
		return "", err
	}
	token, err := finishAuthentication(ctx, tx, u)
	if err != nil {
		return "", err
	}
	return token, tx.Commit()
}
