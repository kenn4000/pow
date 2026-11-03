package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

const (
	onlineBattleInviteLifetime = 10 * time.Minute
	onlineBattleQueueLifetime  = 2 * time.Minute
	onlineBattleLockID         = 783094201
)

var onlineBattleVariants = map[string]bool{
	"main": true, "fog": true, "rummy": true, "ladder": true, "poker-duel": true, "tug-war": true,
}

type onlineBattleRequest struct {
	Action    string `json:"action"`
	Variant   string `json:"variant"`
	HeroID    string `json:"heroId"`
	Recipient string `json:"recipient"`
	InviteID  string `json:"inviteId"`
	Index     int    `json:"index"`
	Slot      int    `json:"slot"`
	Cards     []int  `json:"cards"`
	FaceUp    int    `json:"faceUpIndex"`
	Buried    int    `json:"buriedIndex"`
}

type onlineBattleInvite struct {
	ID        string    `json:"id"`
	Variant   string    `json:"variant"`
	Handle    string    `json:"handle"`
	HeroID    string    `json:"heroId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type onlineBattlePlayer struct {
	Handle string `json:"handle"`
	HeroID string `json:"heroId"`
	Hero   string `json:"hero"`
}

type onlineBattlePublicMatch struct {
	ID      string                `json:"id"`
	Variant string                `json:"variant"`
	Status  string                `json:"status"`
	Player  int                   `json:"player"`
	Players [2]onlineBattlePlayer `json:"players"`
	View    webView               `json:"view"`
}

type onlineBattleResponse struct {
	Status   string                   `json:"status"`
	Variant  string                   `json:"variant,omitempty"`
	Match    *onlineBattlePublicMatch `json:"match,omitempty"`
	Incoming []onlineBattleInvite     `json:"incomingInvites"`
	Outgoing []onlineBattleInvite     `json:"outgoingInvites"`
	InviteID string                   `json:"inviteId,omitempty"`
}

type onlineBattleState struct {
	Session *sessionSnapshot `json:"session"`
}

func (s *webServer) handleOnlineBattle(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getOnlineBattle(w, r, user)
	case http.MethodPost:
		var request onlineBattleRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		switch request.Action {
		case "queue":
			s.queueOnlineBattle(w, r, user, request)
		case "cancel-queue":
			s.cancelOnlineBattleQueue(w, r, user)
		case "invite":
			s.inviteOnlineBattleFriend(w, r, user, request)
		case "accept":
			s.respondOnlineBattleInvite(w, r, user, request, true)
		case "decline", "cancel-invite":
			s.respondOnlineBattleInvite(w, r, user, request, false)
		default:
			writeError(w, http.StatusBadRequest, errors.New("unknown online battle action"))
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *webServer) handleOnlineBattleAction(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var request onlineBattleRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()

	match, player, variant, state, err := loadOnlineBattleMatch(r.Context(), tx, chi.URLParam(r, "matchID"), user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("online battle not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	if match.Status == "completed" {
		public, err := s.onlineBattlePublicMatch(r.Context(), tx, match, player, variant, state)
		if err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, onlineBattleResponse{Status: "completed", Variant: variant, Match: &public})
		return
	}
	profileData, err := onlineBattleProfile(r.Context(), tx, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	engine, err := s.restoreOnlineBattleEngine(profileData, match.ID, state, player)
	if err != nil {
		databaseError(w, err)
		return
	}
	if engine.session.actor != player || engine.session.phase == "gameover" {
		writeError(w, http.StatusConflict, errors.New("it is not your turn"))
		return
	}
	if err := engine.applyAction(actionRequest{
		Action: request.Action, Index: request.Index, Slot: request.Slot,
		Cards: request.Cards, FaceUpIndex: request.FaceUp, BuriedIndex: request.Buried,
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := advanceOnlineBattle(engine); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	state, err = encodeOnlineBattleState(engine.session)
	if err != nil {
		databaseError(w, err)
		return
	}
	status := "active"
	if engine.session.phase == "gameover" {
		status = "completed"
	}
	if err := saveOnlineBattleState(r.Context(), tx, match.ID, state, status); err != nil {
		databaseError(w, err)
		return
	}
	match.State = state
	match.Status = status
	public, err := s.onlineBattlePublicMatch(r.Context(), tx, match, player, variant, state)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, onlineBattleResponse{Status: status, Variant: variant, Match: &public})
}

type onlineBattleStoredMatch struct {
	ID      string
	Variant string
	PlayerA string
	PlayerB string
	HeroA   string
	HeroB   string
	Status  string
	State   []byte
	Created time.Time
}

func (s *webServer) getOnlineBattle(w http.ResponseWriter, r *http.Request, user account) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE online_battle_invites SET status='expired',responded_at=$1
		WHERE status='pending' AND created_at<=$2`, now, now.Add(-onlineBattleInviteLifetime)); err != nil {
		databaseError(w, err)
		return
	}
	response := onlineBattleResponse{
		Status: "idle", Incoming: []onlineBattleInvite{}, Outgoing: []onlineBattleInvite{},
	}
	match, player, err := loadCurrentOnlineBattle(r.Context(), tx, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		databaseError(w, err)
		return
	}
	if err == nil {
		public, err := s.onlineBattlePublicMatch(r.Context(), tx, match, player, match.Variant, match.State)
		if err != nil {
			databaseError(w, err)
			return
		}
		response.Status, response.Variant, response.Match = match.Status, match.Variant, &public
	} else {
		var queuedVariant string
		err = tx.QueryRowContext(r.Context(), `
			SELECT variant FROM online_battle_queue
			WHERE user_id=$1 AND last_seen>$2`, user.ID, now.Add(-onlineBattleQueueLifetime)).Scan(&queuedVariant)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			databaseError(w, err)
			return
		}
		if err == nil {
			response.Status, response.Variant = "queued", queuedVariant
			if _, err := tx.ExecContext(r.Context(), "UPDATE online_battle_queue SET last_seen=$1 WHERE user_id=$2", now, user.ID); err != nil {
				databaseError(w, err)
				return
			}
		} else {
			if _, err := tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE user_id=$1", user.ID); err != nil {
				databaseError(w, err)
				return
			}
			match, player, err = loadLatestOnlineBattle(r.Context(), tx, user.ID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				databaseError(w, err)
				return
			}
			if err == nil {
				public, err := s.onlineBattlePublicMatch(r.Context(), tx, match, player, match.Variant, match.State)
				if err != nil {
					databaseError(w, err)
					return
				}
				response.Status, response.Variant, response.Match = match.Status, match.Variant, &public
			}
		}
	}
	response.Incoming, err = queryOnlineBattleInvites(r.Context(), tx, user.ID, true)
	if err != nil {
		databaseError(w, err)
		return
	}
	response.Outgoing, err = queryOnlineBattleInvites(r.Context(), tx, user.ID, false)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *webServer) queueOnlineBattle(w http.ResponseWriter, r *http.Request, user account, request onlineBattleRequest) {
	if !onlineBattleVariants[request.Variant] {
		writeError(w, http.StatusBadRequest, errors.New("choose a supported online battle variant"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	if err := lockOnlineBattleQueue(r.Context(), tx); err != nil {
		databaseError(w, err)
		return
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE last_seen<=$1", now.Add(-onlineBattleQueueLifetime)); err != nil {
		databaseError(w, err)
		return
	}
	hero, err := puzzleBattleHero(r.Context(), tx, user.ID, request.HeroID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var opponentID, opponentHeroID string
	err = tx.QueryRowContext(r.Context(), `
		SELECT q.user_id,q.hero_id FROM online_battle_queue q
		WHERE q.user_id<>$1 AND q.variant=$2 AND q.last_seen>$3
		  AND NOT EXISTS (
			SELECT 1 FROM player_blocks b
			WHERE (b.blocker_id=$1 AND b.blocked_id=q.user_id)
			   OR (b.blocker_id=q.user_id AND b.blocked_id=$1)
		  )
		ORDER BY q.queued_at,q.user_id LIMIT 1`,
		user.ID, request.Variant, now.Add(-onlineBattleQueueLifetime)).Scan(&opponentID, &opponentHeroID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := lockPuzzleBattlePlayers(r.Context(), tx, user.ID); err != nil {
			databaseError(w, err)
			return
		}
		active, err := onlineBattlePlayersActive(r.Context(), tx, user.ID)
		if err != nil {
			databaseError(w, err)
			return
		}
		if active {
			writeError(w, http.StatusConflict, errors.New("finish your current online battle before queuing again"))
			return
		}
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id=$1", user.ID); err != nil {
			databaseError(w, err)
			return
		}
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO online_battle_queue(user_id,variant,hero_id,queued_at,last_seen)
			VALUES($1,$2,$3,$4,$4)
			ON CONFLICT(user_id) DO UPDATE SET variant=EXCLUDED.variant,hero_id=EXCLUDED.hero_id,
				queued_at=EXCLUDED.queued_at,last_seen=EXCLUDED.last_seen`,
			user.ID, request.Variant, hero.ID, now)
		if err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, onlineBattleResponse{Status: "queued", Variant: request.Variant})
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := lockPuzzleBattlePlayers(r.Context(), tx, user.ID, opponentID); err != nil {
		databaseError(w, err)
		return
	}
	active, err := onlineBattlePlayersActive(r.Context(), tx, user.ID, opponentID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if active {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE user_id=$1", opponentID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `
				INSERT INTO online_battle_queue(user_id,variant,hero_id,queued_at,last_seen)
				VALUES($1,$2,$3,$4,$4)
				ON CONFLICT(user_id) DO UPDATE SET variant=EXCLUDED.variant,hero_id=EXCLUDED.hero_id,
					queued_at=EXCLUDED.queued_at,last_seen=EXCLUDED.last_seen`,
				user.ID, request.Variant, hero.ID, now)
		}
		if err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, onlineBattleResponse{Status: "queued", Variant: request.Variant})
		return
	}
	ids := [2]string{opponentID, user.ID}
	heroIDs := [2]string{opponentHeroID, hero.ID}
	public, err := s.createOnlineBattleTx(r.Context(), tx, request.Variant, ids, heroIDs, 1, now)
	if err != nil {
		if strings.Contains(err.Error(), "already has an active match") {
			writeError(w, http.StatusConflict, err)
		} else {
			databaseError(w, err)
		}
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, onlineBattleResponse{Status: "active", Variant: request.Variant, Match: &public})
}

func (s *webServer) cancelOnlineBattleQueue(w http.ResponseWriter, r *http.Request, user account) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	if err := lockOnlineBattleQueue(r.Context(), tx); err != nil {
		databaseError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE user_id=$1", user.ID); err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, onlineBattleResponse{Status: "idle"})
}

func (s *webServer) inviteOnlineBattleFriend(w http.ResponseWriter, r *http.Request, user account, request onlineBattleRequest) {
	if !onlineBattleVariants[request.Variant] {
		writeError(w, http.StatusBadRequest, errors.New("choose a supported online battle variant"))
		return
	}
	handle := strings.TrimSpace(request.Recipient)
	if !validSocialHandle(handle) {
		writeError(w, http.StatusBadRequest, errors.New("enter a valid friend handle"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	if err := lockOnlineBattleQueue(r.Context(), tx); err != nil {
		databaseError(w, err)
		return
	}
	var recipientID string
	if err := tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE lower(public_handle)=lower($1)", handle).Scan(&recipientID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("player not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	if recipientID == user.ID {
		writeError(w, http.StatusBadRequest, errors.New("you cannot invite yourself"))
		return
	}
	if err := lockPuzzleBattlePlayers(r.Context(), tx, user.ID, recipientID); err != nil {
		databaseError(w, err)
		return
	}
	if err := ensureOnlineBattleFriends(r.Context(), tx, user.ID, recipientID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	active, err := onlineBattlePlayersActive(r.Context(), tx, user.ID, recipientID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if active {
		writeError(w, http.StatusConflict, errors.New("one of these players already has an active match"))
		return
	}
	hero, err := puzzleBattleHero(r.Context(), tx, user.ID, request.HeroID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE user_id IN ($1,$2)", user.ID, recipientID); err != nil {
		databaseError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id IN ($1,$2)", user.ID, recipientID); err != nil {
		databaseError(w, err)
		return
	}
	inviteID := randomID()
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO online_battle_invites(id,variant,sender_id,recipient_id,sender_hero_id)
		VALUES($1,$2,$3,$4,$5)`, inviteID, request.Variant, user.ID, recipientID, hero.ID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, errors.New("a pending invitation already exists for this friend and variant"))
		} else {
			databaseError(w, err)
		}
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, onlineBattleResponse{Status: "invite-sent", Variant: request.Variant, InviteID: inviteID})
}

func (s *webServer) respondOnlineBattleInvite(w http.ResponseWriter, r *http.Request, user account, request onlineBattleRequest, accept bool) {
	if strings.TrimSpace(request.InviteID) == "" {
		writeError(w, http.StatusBadRequest, errors.New("invitation id is required"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	if err := lockOnlineBattleQueue(r.Context(), tx); err != nil {
		databaseError(w, err)
		return
	}
	ownerColumn := "recipient_id"
	if !accept && request.Action == "cancel-invite" {
		ownerColumn = "sender_id"
	}
	var senderID, variant, senderHeroID string
	err = tx.QueryRowContext(r.Context(), fmt.Sprintf(`
		SELECT sender_id,variant,sender_hero_id FROM online_battle_invites
		WHERE id=$1 AND %s=$2 AND status='pending' AND created_at>$3 FOR UPDATE`, ownerColumn),
		request.InviteID, user.ID, time.Now().UTC().Add(-onlineBattleInviteLifetime)).
		Scan(&senderID, &variant, &senderHeroID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("pending invitation not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	if accept {
		if err := lockPuzzleBattlePlayers(r.Context(), tx, senderID, user.ID); err != nil {
			databaseError(w, err)
			return
		}
		if err := ensureOnlineBattleFriends(r.Context(), tx, senderID, user.ID); err != nil {
			writeError(w, http.StatusForbidden, errors.New("this invitation is no longer available"))
			return
		}
		active, err := onlineBattlePlayersActive(r.Context(), tx, senderID, user.ID)
		if err != nil {
			databaseError(w, err)
			return
		}
		if active {
			writeError(w, http.StatusConflict, errors.New("one of these players already has an active match"))
			return
		}
		hero, err := puzzleBattleHero(r.Context(), tx, user.ID, request.HeroID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		now := time.Now().UTC()
		ids, heroIDs := [2]string{senderID, user.ID}, [2]string{senderHeroID, hero.ID}
		public, err := s.createOnlineBattleTx(r.Context(), tx, variant, ids, heroIDs, 1, now)
		if err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			UPDATE online_battle_invites SET status='accepted',recipient_hero_id=$1,responded_at=$2
			WHERE id=$3`, hero.ID, now, request.InviteID); err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, onlineBattleResponse{Status: "active", Variant: variant, Match: &public})
		return
	}
	status := "declined"
	if request.Action == "cancel-invite" {
		status = "expired"
	}
	if _, err := tx.ExecContext(r.Context(), "UPDATE online_battle_invites SET status=$1,responded_at=$2 WHERE id=$3", status, time.Now().UTC(), request.InviteID); err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, onlineBattleResponse{Status: "idle"})
}

func lockOnlineBattleQueue(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", onlineBattleLockID)
	return err
}

func ensureOnlineBattleFriends(ctx context.Context, tx *sql.Tx, firstID, secondID string) error {
	var friend, blocked bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM friendships
			WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)
		), EXISTS (
			SELECT 1 FROM player_blocks
			WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)
		)`, firstID, secondID).Scan(&friend, &blocked); err != nil {
		return err
	}
	if !friend || blocked {
		return errors.New("online battle invitations are limited to unblocked friends")
	}
	return nil
}

func onlineBattlePlayersActive(ctx context.Context, tx *sql.Tx, ids ...string) (bool, error) {
	for _, id := range ids {
		var active bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM online_battle_active_users WHERE user_id=$1
			) OR EXISTS (
				SELECT 1 FROM puzzle_battle_matches
				WHERE status='active' AND (player_a_id=$1 OR player_b_id=$1)
			)`, id).Scan(&active); err != nil {
			return false, err
		}
		if active {
			return true, nil
		}
	}
	return false, nil
}

func (s *webServer) createOnlineBattleTx(
	ctx context.Context,
	tx *sql.Tx,
	variant string,
	ids, heroIDs [2]string,
	viewer int,
	now time.Time,
) (onlineBattlePublicMatch, error) {
	if err := lockPuzzleBattlePlayers(ctx, tx, ids[0], ids[1]); err != nil {
		return onlineBattlePublicMatch{}, err
	}
	active, err := onlineBattlePlayersActive(ctx, tx, ids[0], ids[1])
	if err != nil {
		return onlineBattlePublicMatch{}, fmt.Errorf("check active battles: %w", err)
	}
	if active {
		return onlineBattlePublicMatch{}, errors.New("one of these players already has an active match")
	}
	engine, err := s.newOnlineBattleEngine(ctx, tx, variant, ids, heroIDs, now)
	if err != nil {
		return onlineBattlePublicMatch{}, err
	}
	if err := advanceOnlineBattle(engine); err != nil {
		return onlineBattlePublicMatch{}, err
	}
	state, err := encodeOnlineBattleState(engine.session)
	if err != nil {
		return onlineBattlePublicMatch{}, err
	}
	matchID := randomID()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO online_battle_matches(id,variant,player_a_id,player_b_id,player_a_hero_id,player_b_hero_id,state)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, matchID, variant, ids[0], ids[1], heroIDs[0], heroIDs[1], state)
	if err != nil {
		return onlineBattlePublicMatch{}, fmt.Errorf("save new online battle: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO online_battle_active_users(user_id,match_id) VALUES($1,$3),($2,$3)`,
		ids[0], ids[1], matchID); err != nil {
		return onlineBattlePublicMatch{}, fmt.Errorf("reserve players for online battle: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM online_battle_queue WHERE user_id IN ($1,$2)", ids[0], ids[1]); err != nil {
		return onlineBattlePublicMatch{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM puzzle_battle_queue WHERE user_id IN ($1,$2)", ids[0], ids[1]); err != nil {
		return onlineBattlePublicMatch{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE online_battle_invites SET status='expired',responded_at=$1
		WHERE status='pending' AND (sender_id IN ($2,$3) OR recipient_id IN ($2,$3))`,
		now, ids[0], ids[1]); err != nil {
		return onlineBattlePublicMatch{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE puzzle_battle_invites SET status='expired',responded_at=$1
		WHERE status='pending' AND (sender_id IN ($2,$3) OR recipient_id IN ($2,$3))`,
		now, ids[0], ids[1]); err != nil {
		return onlineBattlePublicMatch{}, err
	}
	match := onlineBattleStoredMatch{
		ID: matchID, Variant: variant, PlayerA: ids[0], PlayerB: ids[1],
		HeroA: heroIDs[0], HeroB: heroIDs[1], Status: "active", State: state, Created: now,
	}
	return s.onlineBattlePublicMatch(ctx, tx, match, viewer, variant, state)
}

func (s *webServer) newOnlineBattleEngine(
	ctx context.Context,
	tx *sql.Tx,
	variant string,
	ids, heroIDs [2]string,
	now time.Time,
) (*webServer, error) {
	profiles := [2]*profile.Profile{}
	playerHeroes := [2]heroes.Hero{}
	for player := range ids {
		p, err := onlineBattleProfile(ctx, tx, ids[player])
		if err != nil {
			return nil, err
		}
		profiles[player] = p
		playerHeroes[player], err = puzzleBattleHero(ctx, tx, ids[player], heroIDs[player])
		if err != nil {
			return nil, err
		}
	}
	rng, err := secureRand()
	if err != nil {
		return nil, fmt.Errorf("create online battle random source: %w", err)
	}
	engine := &webServer{
		rng: rng, defaultMode: "easy", playerProfile: profiles[0],
		viewerPlayer: -1,
	}
	switch variant {
	case "main", "fog":
		modifiers, err := heroes.GameModifiersFor(playerHeroes[0], playerHeroes[1], rng)
		if err != nil {
			return nil, err
		}
		state, err := game.NewDraftGameWithDraftMode(rng, modifiers, heroes.DraftMode(playerHeroes[0], playerHeroes[1]))
		if err != nil {
			return nil, err
		}
		if err := engine.startWebGameSession(state, "hvh", heroIDs[0], heroIDs[1]); err != nil {
			return nil, err
		}
		engine.session.variant = variant
	case "rummy":
		engine.startWebRummy("hvh", heroIDs[0], heroIDs[1], playerHeroes[0], playerHeroes[1])
	case "ladder":
		if err := engine.startWebLadder("hvh", heroIDs[0], heroIDs[1], playerHeroes[0], playerHeroes[1]); err != nil {
			return nil, err
		}
	case "poker-duel":
		duel, err := newPokerDuelGame(playerHeroes[0], playerHeroes[1], rng, now)
		if err != nil {
			return nil, err
		}
		engine.session = &webSession{
			mode: "hvh", variant: variant, pokerDuel: duel, phase: "handoff",
			actor: duel.ActivePlayer, handoffTarget: duel.ActivePlayer,
			handoffPhase: "poker-duel-play", winner: -1, playerHeroID: heroIDs[0],
			opponentHeroID: heroIDs[1], message: fmt.Sprintf("Player %d is up.", duel.ActivePlayer+1),
			initialAchievements: achievementLevels(profiles[0]),
		}
	case "tug-war":
		if err := engine.startWebTugWar("hvh", heroIDs[0], heroIDs[1], playerHeroes[0], playerHeroes[1]); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported online battle variant")
	}
	engine.gameID = randomID()
	engine.session.mode = "hvh"
	engine.session.initialAchievements = achievementLevels(profiles[0])
	return engine, nil
}

func advanceOnlineBattle(engine *webServer) error {
	for engine.session != nil && engine.session.phase == "handoff" {
		if err := engine.readyAction(); err != nil {
			return err
		}
	}
	return nil
}

func encodeOnlineBattleState(session *webSession) ([]byte, error) {
	state, err := json.Marshal(onlineBattleState{Session: snapshotSession(session)})
	if err != nil {
		return nil, fmt.Errorf("encode online battle state: %w", err)
	}
	return state, nil
}

func onlineBattleProfile(ctx context.Context, queryer puzzleBattleQueryRower, userID string) (*profile.Profile, error) {
	var data []byte
	if err := queryer.QueryRowContext(ctx, "SELECT profile FROM player_data WHERE user_id=$1", userID).Scan(&data); err != nil {
		return nil, fmt.Errorf("load online battle profile: %w", err)
	}
	var p profile.Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode online battle profile: %w", err)
	}
	p.SetSaveHandler(func(*profile.Profile) error { return nil })
	if err := (&app{}).initializeProgressionProfile(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func loadCurrentOnlineBattle(ctx context.Context, tx *sql.Tx, userID string) (onlineBattleStoredMatch, int, error) {
	var matchID string
	err := tx.QueryRowContext(ctx, "SELECT match_id FROM online_battle_active_users WHERE user_id=$1 FOR UPDATE", userID).Scan(&matchID)
	if err != nil {
		return onlineBattleStoredMatch{}, 0, err
	}
	return loadOnlineBattleByID(ctx, tx, matchID, userID)
}

func loadLatestOnlineBattle(ctx context.Context, tx *sql.Tx, userID string) (onlineBattleStoredMatch, int, error) {
	var matchID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM online_battle_matches
		WHERE status='completed' AND (player_a_id=$1 OR player_b_id=$1)
		ORDER BY completed_at DESC NULLS LAST,updated_at DESC LIMIT 1`, userID).Scan(&matchID)
	if err != nil {
		return onlineBattleStoredMatch{}, 0, err
	}
	return loadOnlineBattleByID(ctx, tx, matchID, userID)
}

func loadOnlineBattleMatch(ctx context.Context, tx *sql.Tx, matchID, userID string) (onlineBattleStoredMatch, int, string, []byte, error) {
	match, player, err := loadOnlineBattleByID(ctx, tx, matchID, userID)
	if err != nil {
		return match, player, "", nil, err
	}
	return match, player, match.Variant, match.State, nil
}

func loadOnlineBattleByID(ctx context.Context, tx *sql.Tx, matchID, userID string) (onlineBattleStoredMatch, int, error) {
	var match onlineBattleStoredMatch
	err := tx.QueryRowContext(ctx, `
		SELECT id,variant,player_a_id,player_b_id,player_a_hero_id,player_b_hero_id,status,state,created_at
		FROM online_battle_matches
		WHERE id=$1 AND (player_a_id=$2 OR player_b_id=$2)
		FOR UPDATE`, matchID, userID).Scan(
		&match.ID, &match.Variant, &match.PlayerA, &match.PlayerB, &match.HeroA, &match.HeroB,
		&match.Status, &match.State, &match.Created)
	if err != nil {
		return onlineBattleStoredMatch{}, 0, err
	}
	if userID == match.PlayerA {
		return match, 0, nil
	}
	if userID == match.PlayerB {
		return match, 1, nil
	}
	return onlineBattleStoredMatch{}, 0, sql.ErrNoRows
}

func (s *webServer) restoreOnlineBattleEngine(viewerProfile *profile.Profile, matchID string, state []byte, viewer int) (*webServer, error) {
	var saved onlineBattleState
	if err := json.Unmarshal(state, &saved); err != nil {
		return nil, fmt.Errorf("decode online battle state: %w", err)
	}
	if saved.Session == nil {
		return nil, errors.New("online battle has no saved game state")
	}
	if viewerProfile == nil {
		return nil, errors.New("online battle viewer profile is unavailable")
	}
	viewerProfile.SetSaveHandler(func(*profile.Profile) error { return nil })
	if err := (&app{}).initializeProgressionProfile(viewerProfile); err != nil {
		return nil, err
	}
	rng, err := secureRand()
	if err != nil {
		return nil, fmt.Errorf("restore online battle random source: %w", err)
	}
	engine := &webServer{
		rng: rng, defaultMode: "easy", playerProfile: viewerProfile,
		onlineViewer: true,
		session:      saved.Session.restore(), gameID: matchID, viewerPlayer: viewer,
	}
	if engine.session == nil || !onlineBattleVariants[engine.session.variant] || engine.session.campaign != nil {
		return nil, errors.New("online battle contains an unsupported game session")
	}
	if engine.session.variant == "tug-war" && engine.session.tugWar == nil {
		return nil, errors.New("online Tug War state is unavailable")
	}
	engine.session.mode = "hvh"
	return engine, nil
}

func (s *webServer) onlineBattlePublicMatch(
	ctx context.Context,
	queryer interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	match onlineBattleStoredMatch,
	player int,
	variant string,
	state []byte,
) (onlineBattlePublicMatch, error) {
	viewerProfile, err := onlineBattleProfile(ctx, queryer, [2]string{match.PlayerA, match.PlayerB}[player])
	if err != nil {
		return onlineBattlePublicMatch{}, err
	}
	engine, err := s.restoreOnlineBattleEngine(viewerProfile, match.ID, state, player)
	if err != nil {
		return onlineBattlePublicMatch{}, err
	}
	publicView := onlineBattleView(engine, player)
	result := onlineBattlePublicMatch{
		ID: match.ID, Variant: variant, Status: match.Status, Player: 0, View: publicView,
	}
	ids, heroIDs := [2]string{match.PlayerA, match.PlayerB}, [2]string{match.HeroA, match.HeroB}
	for index := range result.Players {
		var handle string
		if err := queryer.QueryRowContext(ctx, "SELECT public_handle FROM users WHERE id=$1", ids[index]).Scan(&handle); err != nil {
			return onlineBattlePublicMatch{}, fmt.Errorf("load online battle player handle: %w", err)
		}
		hero, ok := heroes.ByID(heroIDs[index])
		if !ok {
			return onlineBattlePublicMatch{}, fmt.Errorf("unknown stored online battle hero %q", heroIDs[index])
		}
		result.Players[index] = onlineBattlePlayer{Handle: handle, HeroID: hero.ID, Hero: hero.Name}
	}
	if player == 1 {
		result.Players[0], result.Players[1] = result.Players[1], result.Players[0]
	}
	return result, nil
}

func onlineBattleView(engine *webServer, viewer int) webView {
	view := engine.view()
	session := engine.session
	flip := viewer == 1
	view.Mode = "online"
	view.CanAct = session.phase != "gameover" && session.actor == viewer
	view.Viewer = 0
	view.Actor = onlineRelativePlayer(view.Actor, flip)
	view.Attacker = onlineRelativePlayer(view.Attacker, flip)
	view.Winner = onlineRelativePlayer(view.Winner, flip)
	if flip {
		view.Message = strings.NewReplacer("Player 1", "\x00", "Player 2", "Player 1", "\x00", "Player 2").
			Replace(view.Message)
		view.PlayerHero, view.OpponentHero = view.OpponentHero, view.PlayerHero
		view.PlayerHeroID, view.OpponentHeroID = view.OpponentHeroID, view.PlayerHeroID
		view.DefaultHeroID = view.PlayerHeroID
		if len(view.Players) == 2 {
			view.Players[0], view.Players[1] = view.Players[1], view.Players[0]
		}
		view.Scores[0], view.Scores[1] = view.Scores[1], view.Scores[0]
		for i := range view.Battles {
			view.Battles[i].AttackerIndex = onlineRelativePlayer(view.Battles[i].AttackerIndex, true)
			view.Battles[i].Scores[0], view.Battles[i].Scores[1] = view.Battles[i].Scores[1], view.Battles[i].Scores[0]
		}
	}
	for index := range view.Players {
		view.Players[index].You = index == 0
	}
	view.Payout = nil
	view.AchievementUpdates = nil
	if view.Rummy != nil {
		for i := range view.Rummy.Results {
			view.Rummy.Results[i].Player = onlineRelativePlayer(view.Rummy.Results[i].Player, flip)
		}
		sort.Slice(view.Rummy.Results, func(i, j int) bool { return view.Rummy.Results[i].Player < view.Rummy.Results[j].Player })
	}
	if view.Ladder != nil && flip {
		view.Ladder.CurrentPlayer = onlineRelativePlayer(view.Ladder.CurrentPlayer, true)
		if len(view.Ladder.FinalHands) == 2 {
			view.Ladder.FinalHands[0], view.Ladder.FinalHands[1] = view.Ladder.FinalHands[1], view.Ladder.FinalHands[0]
		}
	}
	if view.PokerDuel != nil {
		duel := view.PokerDuel
		duel.ActivePlayer = onlineRelativePlayer(duel.ActivePlayer, flip)
		duel.Winner = onlineRelativePlayer(duel.Winner, flip)
		if flip {
			duel.Players[0], duel.Players[1] = duel.Players[1], duel.Players[0]
			duel.Heroes[0], duel.Heroes[1] = duel.Heroes[1], duel.Heroes[0]
			if len(duel.FinalScores) == 2 {
				duel.FinalScores[0], duel.FinalScores[1] = duel.FinalScores[1], duel.FinalScores[0]
			}
		}
	}
	if view.TugWar != nil {
		tugWar := view.TugWar
		tugWar.Viewer = onlineRelativePlayer(tugWar.Viewer, flip)
		tugWar.Current = onlineRelativePlayer(tugWar.Current, flip)
		if flip {
			tugWar.Players[0], tugWar.Players[1] = tugWar.Players[1], tugWar.Players[0]
			for index := range tugWar.Piles {
				tugWar.Piles[index].Zone = -tugWar.Piles[index].Zone
			}
			for index := range tugWar.Log {
				entry := &tugWar.Log[index]
				entry.Player = onlineRelativePlayer(entry.Player, true)
				entry.ZoneBefore = -entry.ZoneBefore
				entry.ZoneAfter = -entry.ZoneAfter
				entry.Refilled[0], entry.Refilled[1] = entry.Refilled[1], entry.Refilled[0]
			}
			if tugWar.Result != nil {
				tugWar.Result.Winner = onlineRelativePlayer(tugWar.Result.Winner, true)
				tugWar.Result.Scores[0], tugWar.Result.Scores[1] = tugWar.Result.Scores[1], tugWar.Result.Scores[0]
				tugWar.Result.ZoneCards[0], tugWar.Result.ZoneCards[1] = tugWar.Result.ZoneCards[1], tugWar.Result.ZoneCards[0]
				for index := range tugWar.Result.Piles {
					tugWar.Result.Piles[index].Zone = -tugWar.Result.Piles[index].Zone
				}
			}
		}
		for index := range tugWar.Players {
			tugWar.Players[index].You = index == 0
		}
	}
	return view
}

func onlineRelativePlayer(player int, flip bool) int {
	if player < 0 || !flip {
		return player
	}
	return 1 - player
}

func saveOnlineBattleState(ctx context.Context, tx *sql.Tx, matchID string, state []byte, status string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE online_battle_matches
		SET state=$1,status=$2,updated_at=now(),
			completed_at=CASE WHEN $2='completed' THEN COALESCE(completed_at,now()) ELSE completed_at END
		WHERE id=$3`, state, status, matchID)
	if err != nil {
		return err
	}
	if status == "completed" {
		_, err = tx.ExecContext(ctx, "DELETE FROM online_battle_active_users WHERE match_id=$1", matchID)
	}
	return err
}

func queryOnlineBattleInvites(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID string, incoming bool) ([]onlineBattleInvite, error) {
	ownerColumn, otherColumn := "sender_id", "recipient_id"
	if incoming {
		ownerColumn, otherColumn = otherColumn, ownerColumn
	}
	rows, err := queryer.QueryContext(ctx, fmt.Sprintf(`
		SELECT i.id,i.variant,u.public_handle,i.sender_hero_id,i.created_at
		FROM online_battle_invites i JOIN users u ON u.id=i.%s
		WHERE i.%s=$1 AND i.status='pending' AND i.created_at>$2
		ORDER BY i.created_at DESC`, otherColumn, ownerColumn), userID, time.Now().UTC().Add(-onlineBattleInviteLifetime))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invites := []onlineBattleInvite{}
	for rows.Next() {
		var invite onlineBattleInvite
		if err := rows.Scan(&invite.ID, &invite.Variant, &invite.Handle, &invite.HeroID, &invite.CreatedAt); err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}
	return invites, rows.Err()
}
