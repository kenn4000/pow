package app

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var socialHandlePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{2,19}$`)

type socialPlayer struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	ELO       int       `json:"elo"`
	Rank      int       `json:"rank,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type socialFriendRequest struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type socialMessage struct {
	ID              string    `json:"id"`
	SenderHandle    string    `json:"senderHandle"`
	RecipientHandle string    `json:"recipientHandle"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"createdAt"`
}

func validSocialHandle(handle string) bool {
	return socialHandlePattern.MatchString(handle)
}

func (s *webServer) handleSocialOverview(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	response := map[string]any{
		"user": map[string]any{"handle": user.PublicHandle, "elo": 1000, "createdAt": user.CreatedAt},
	}
	var elo, rank int
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT elo_rating,ranking FROM (
			SELECT id,elo_rating,RANK() OVER (ORDER BY elo_rating DESC) AS ranking FROM users
		) ranked WHERE id=$1`, user.ID).Scan(&elo, &rank); err != nil {
		databaseError(w, err)
		return
	}
	response["user"].(map[string]any)["elo"] = elo
	response["user"].(map[string]any)["rank"] = rank

	friends, err := s.querySocialPlayers(r, `
		SELECT u.id,u.public_handle,u.elo_rating,f.created_at
		FROM friendships f
		JOIN users u ON u.id=CASE WHEN f.user_a=$1 THEN f.user_b ELSE f.user_a END
		WHERE f.user_a=$1 OR f.user_b=$1
		ORDER BY lower(u.public_handle)`, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	incoming, err := s.querySocialRequests(r, `
		SELECT fr.id,u.public_handle,fr.message,fr.created_at
		FROM friend_requests fr JOIN users u ON u.id=fr.sender_id
		WHERE fr.recipient_id=$1 AND fr.status='pending'
		ORDER BY fr.created_at`, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	outgoing, err := s.querySocialRequests(r, `
		SELECT fr.id,u.public_handle,fr.message,fr.created_at
		FROM friend_requests fr JOIN users u ON u.id=fr.recipient_id
		WHERE fr.sender_id=$1 AND fr.status='pending'
		ORDER BY fr.created_at`, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	blocks, err := s.querySocialPlayers(r, `
		SELECT u.id,u.public_handle,u.elo_rating,b.created_at
		FROM player_blocks b JOIN users u ON u.id=b.blocked_id
		WHERE b.blocker_id=$1 ORDER BY lower(u.public_handle)`, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	messages := make([]socialMessage, 0)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT m.id,s.public_handle,d.public_handle,m.body,m.created_at
		FROM social_messages m
		JOIN users s ON s.id=m.sender_id
		JOIN users d ON d.id=m.recipient_id
		WHERE m.sender_id=$1 OR m.recipient_id=$1
		ORDER BY m.created_at DESC LIMIT 50`, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	for rows.Next() {
		var message socialMessage
		if err := rows.Scan(&message.ID, &message.SenderHandle, &message.RecipientHandle, &message.Body, &message.CreatedAt); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		databaseError(w, err)
		return
	}
	if err := rows.Close(); err != nil {
		databaseError(w, err)
		return
	}
	leaderRows, err := s.db.QueryContext(r.Context(), `
		SELECT id,public_handle,elo_rating,ranking,created_at FROM (
			SELECT id,public_handle,elo_rating,created_at,
				RANK() OVER (ORDER BY elo_rating DESC) AS ranking,
				ROW_NUMBER() OVER (ORDER BY elo_rating DESC,lower(public_handle)) AS position
			FROM users
		) ranked WHERE position <= 25 ORDER BY position`)
	if err != nil {
		databaseError(w, err)
		return
	}
	leaderboard := make([]socialPlayer, 0)
	for leaderRows.Next() {
		var player socialPlayer
		if err := leaderRows.Scan(&player.ID, &player.Handle, &player.ELO, &player.Rank, &player.CreatedAt); err != nil {
			leaderRows.Close()
			databaseError(w, err)
			return
		}
		leaderboard = append(leaderboard, player)
	}
	if err := leaderRows.Err(); err != nil {
		leaderRows.Close()
		databaseError(w, err)
		return
	}
	if err := leaderRows.Close(); err != nil {
		databaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": response["user"], "friends": friends, "incomingRequests": incoming,
		"outgoingRequests": outgoing, "blocks": blocks, "messages": messages, "leaderboard": leaderboard,
		"wagerLimits": map[string]int{"perGame": 1000, "per24Hours": 5000},
	})
}

func (s *webServer) querySocialPlayers(r *http.Request, query string, args ...any) ([]socialPlayer, error) {
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	players := make([]socialPlayer, 0)
	for rows.Next() {
		var player socialPlayer
		if err := rows.Scan(&player.ID, &player.Handle, &player.ELO, &player.CreatedAt); err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	return players, rows.Err()
}

func (s *webServer) querySocialRequests(r *http.Request, query string, args ...any) ([]socialFriendRequest, error) {
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]socialFriendRequest, 0)
	for rows.Next() {
		var request socialFriendRequest
		if err := rows.Scan(&request.ID, &request.Handle, &request.Message, &request.CreatedAt); err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (s *webServer) handleSocialSetHandle(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var input struct {
		Handle string `json:"handle"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	handle := strings.TrimSpace(input.Handle)
	if !validSocialHandle(handle) {
		writeError(w, http.StatusBadRequest, errors.New("handles must be 3–20 characters, start with a letter, and contain only letters, numbers, or underscores"))
		return
	}
	_, err := s.db.ExecContext(r.Context(), "UPDATE users SET public_handle=$1 WHERE id=$2", handle, user.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, errors.New("that handle is already taken"))
			return
		}
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"handle": handle})
}

func (s *webServer) handleSocialFriendRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var input struct {
		Handle  string `json:"handle"`
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Handle = strings.TrimSpace(input.Handle)
	input.Message = strings.TrimSpace(input.Message)
	if len([]rune(input.Message)) > 140 {
		writeError(w, http.StatusBadRequest, errors.New("friend request messages are limited to 140 characters"))
		return
	}
	if input.Handle == "" {
		writeError(w, http.StatusBadRequest, errors.New("enter a player handle"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var targetID string
	err = tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE lower(public_handle)=lower($1)", input.Handle).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("player not found"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if targetID == user.ID {
		writeError(w, http.StatusBadRequest, errors.New("you cannot send a friend request to yourself"))
		return
	}
	var restricted bool
	err = tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM player_blocks
			WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)
		) OR EXISTS (
			SELECT 1 FROM friendships
			WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)
		)`, user.ID, targetID).Scan(&restricted)
	if err != nil {
		databaseError(w, err)
		return
	}
	if restricted {
		writeError(w, http.StatusConflict, errors.New("a friend request cannot be sent to this player"))
		return
	}
	_, err = tx.ExecContext(r.Context(), "INSERT INTO friend_requests(id,sender_id,recipient_id,message) VALUES($1,$2,$3,$4)", randomID(), user.ID, targetID, input.Message)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, errors.New("a pending request already exists between these players"))
			return
		}
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *webServer) handleSocialRequestResponse(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	action := chi.URLParam(r, "decision")
	status := "accepted"
	if action == "reject" {
		status = "rejected"
	} else if action != "accept" {
		http.NotFound(w, r)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var senderID, recipientID string
	err = tx.QueryRowContext(r.Context(), `
		UPDATE friend_requests SET status=$1,responded_at=now()
		WHERE id=$2 AND recipient_id=$3 AND status='pending'
		RETURNING sender_id,recipient_id`, status, chi.URLParam(r, "requestID"), user.ID).Scan(&senderID, &recipientID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("pending friend request not found"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if status == "accepted" {
		var blocked bool
		if err := tx.QueryRowContext(r.Context(), `
			SELECT EXISTS (SELECT 1 FROM player_blocks
			WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`,
			senderID, recipientID).Scan(&blocked); err != nil {
			databaseError(w, err)
			return
		}
		if blocked {
			writeError(w, http.StatusConflict, errors.New("this player is blocked"))
			return
		}
		a, b := senderID, recipientID
		if b < a {
			a, b = b, a
		}
		if _, err := tx.ExecContext(r.Context(), "INSERT INTO friendships(user_a,user_b) VALUES($1,$2) ON CONFLICT DO NOTHING", a, b); err != nil {
			databaseError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *webServer) handleSocialBlock(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var input struct {
		Handle string `json:"handle"`
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
	var targetID string
	err = tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE lower(public_handle)=lower($1)", strings.TrimSpace(input.Handle)).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("player not found"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if targetID == user.ID {
		writeError(w, http.StatusBadRequest, errors.New("you cannot block yourself"))
		return
	}
	if _, err := tx.ExecContext(r.Context(), "INSERT INTO player_blocks(blocker_id,blocked_id) VALUES($1,$2) ON CONFLICT DO NOTHING", user.ID, targetID); err != nil {
		databaseError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM friendships
		WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)`, user.ID, targetID); err != nil {
		databaseError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE friend_requests SET status='rejected',responded_at=now()
		WHERE status='pending' AND ((sender_id=$1 AND recipient_id=$2) OR (sender_id=$2 AND recipient_id=$1))`,
		user.ID, targetID); err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *webServer) handleSocialUnblock(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var input struct {
		Handle string `json:"handle"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	handle := strings.TrimSpace(input.Handle)
	if handle == "" {
		writeError(w, http.StatusBadRequest, errors.New("enter a player handle"))
		return
	}
	result, err := s.db.ExecContext(r.Context(), `
		DELETE FROM player_blocks b
		USING users u
		WHERE b.blocker_id=$1 AND b.blocked_id=u.id AND lower(u.public_handle)=lower($2)`,
		user.ID, handle)
	if err != nil {
		databaseError(w, err)
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		databaseError(w, err)
		return
	}
	if count == 0 {
		writeError(w, http.StatusNotFound, errors.New("blocked player not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *webServer) handleSocialMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var input struct {
		Handle string `json:"handle"`
		Body   string `json:"body"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input.Handle = strings.TrimSpace(input.Handle)
	input.Body = strings.TrimSpace(input.Body)
	if input.Handle == "" || input.Body == "" || utf8.RuneCountInString(input.Body) > 1000 {
		writeError(w, http.StatusBadRequest, errors.New("enter a player handle and a message of 1–1000 characters"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var targetID string
	err = tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE lower(public_handle)=lower($1)", input.Handle).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("player not found"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if targetID == user.ID {
		writeError(w, http.StatusBadRequest, errors.New("you cannot message yourself"))
		return
	}
	var allowed bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (SELECT 1 FROM friendships
			WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2))
		AND NOT EXISTS (SELECT 1 FROM player_blocks
			WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`,
		user.ID, targetID).Scan(&allowed); err != nil {
		databaseError(w, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, errors.New("messages can only be sent to unblocked friends"))
		return
	}
	_, err = tx.ExecContext(r.Context(), "INSERT INTO social_messages(id,sender_id,recipient_id,body) VALUES($1,$2,$3,$4)", randomID(), user.ID, targetID, input.Body)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}
