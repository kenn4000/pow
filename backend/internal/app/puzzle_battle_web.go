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
	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/puzzlebattle"
)

const puzzleBattleInviteLifetime = 10 * time.Minute

type puzzleBattleQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type puzzleBattleRequest struct {
	Action    string `json:"action"`
	HeroID    string `json:"heroId"`
	Recipient string `json:"recipient"`
	InviteID  string `json:"inviteId"`
	Index     int    `json:"index"`
}

type puzzleBattleInvite struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	CreatedAt time.Time `json:"createdAt"`
}

type puzzleBattlePlayerView struct {
	Handle string `json:"handle"`
	HeroID string `json:"heroId"`
	Hero   string `json:"hero"`
	Draft  int    `json:"draft"`
}

type puzzleBattlePublicMatch struct {
	View    puzzlebattle.View         `json:"view"`
	Players [2]puzzleBattlePlayerView `json:"players"`
}

type puzzleBattleHistoryEntry struct {
	ID           string               `json:"id"`
	Opponent     string               `json:"opponent"`
	PlayerHero   string               `json:"playerHero"`
	OpponentHero string               `json:"opponentHero"`
	Player       int                  `json:"player"`
	Result       *puzzlebattle.Result `json:"result"`
	CreatedAt    time.Time            `json:"createdAt"`
	CompletedAt  time.Time            `json:"completedAt"`
}

type puzzleBattleResponse struct {
	Status   string                     `json:"status"`
	Match    *puzzleBattlePublicMatch   `json:"match,omitempty"`
	Incoming []puzzleBattleInvite       `json:"incomingInvites"`
	Outgoing []puzzleBattleInvite       `json:"outgoingInvites"`
	History  []puzzleBattleHistoryEntry `json:"history"`
	InviteID string                     `json:"inviteId,omitempty"`
}

type puzzleBattleHistoryResponse struct {
	Matches []puzzleBattleHistoryEntry `json:"matches"`
}

func puzzleBattleHero(ctx context.Context, queryer puzzleBattleQueryRower, userID, heroID string) (heroes.Hero, error) {
	var data []byte
	if err := queryer.QueryRowContext(ctx, "SELECT profile FROM player_data WHERE user_id=$1", userID).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return heroes.Hero{}, errors.New("player profile not found")
		}
		return heroes.Hero{}, fmt.Errorf("load player profile: %w", err)
	}
	var playerProfile profile.Profile
	if err := json.Unmarshal(data, &playerProfile); err != nil {
		return heroes.Hero{}, fmt.Errorf("decode player profile: %w", err)
	}
	if heroID == "" {
		heroID = playerProfile.DefaultHeroID
	}
	if heroID == "" {
		heroID = profile.DefaultHeroID
	}
	if !containsHeroID(playerProfile.UnlockedHeroes, heroID) {
		return heroes.Hero{}, fmt.Errorf("hero %q is not unlocked", heroID)
	}
	hero, ok := heroes.ByID(heroID)
	if !ok {
		return heroes.Hero{}, fmt.Errorf("unknown hero %q", heroID)
	}
	return hero, nil
}

func (s *webServer) handlePuzzleBattle(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getPuzzleBattle(w, r, user)
	case http.MethodPost:
		var request puzzleBattleRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		switch request.Action {
		case "queue":
			s.queuePuzzleBattle(w, r, user, request)
		case "cancel-queue":
			s.cancelPuzzleBattleQueue(w, r, user)
		case "invite":
			s.invitePuzzleBattleFriend(w, r, user, request)
		case "accept":
			s.respondPuzzleBattleInvite(w, r, user, request, true)
		case "decline", "cancel-invite":
			s.respondPuzzleBattleInvite(w, r, user, request, false)
		default:
			writeError(w, http.StatusBadRequest, errors.New("unknown Puzzle Battle action"))
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *webServer) getPuzzleBattle(w http.ResponseWriter, r *http.Request, user account) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE puzzle_battle_invites SET status='expired',responded_at=$1
		WHERE status='pending' AND created_at <= $2`, now, now.Add(-puzzleBattleInviteLifetime)); err != nil {
		databaseError(w, err)
		return
	}

	response := puzzleBattleResponse{
		Status:   "idle",
		Incoming: []puzzleBattleInvite{},
		Outgoing: []puzzleBattleInvite{},
		History:  []puzzleBattleHistoryEntry{},
	}
	match, player, matchErr := loadActivePuzzleBattle(r.Context(), tx, user.ID)
	if matchErr != nil && !errors.Is(matchErr, sql.ErrNoRows) {
		databaseError(w, matchErr)
		return
	}
	if matchErr == nil {
		wasActive := match.Status != puzzlebattle.PhaseComplete
		if wasActive {
			if err := match.Heartbeat(player, now); err != nil && match.Status != puzzlebattle.PhaseComplete {
				writeError(w, http.StatusConflict, err)
				return
			}
			if err := s.savePuzzleBattleTx(r.Context(), tx, match, wasActive, now); err != nil {
				databaseError(w, err)
				return
			}
		}
		public, err := buildPuzzleBattlePublicMatch(r.Context(), tx, match, player, now)
		if err != nil {
			databaseError(w, err)
			return
		}
		response.Match = &public
		response.Status = match.Status
	} else {
		var queued bool
		if err := tx.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM puzzle_battle_queue WHERE user_id=$1)", user.ID).Scan(&queued); err != nil {
			databaseError(w, err)
			return
		}
		if queued {
			response.Status = "queued"
			if _, err := tx.ExecContext(r.Context(), "UPDATE puzzle_battle_queue SET last_seen=$1 WHERE user_id=$2", now, user.ID); err != nil {
				databaseError(w, err)
				return
			}
		} else {
			match, player, err := loadLatestCompletedPuzzleBattle(r.Context(), tx, user.ID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				databaseError(w, err)
				return
			}
			if err == nil {
				public, err := buildPuzzleBattlePublicMatch(r.Context(), tx, match, player, now)
				if err != nil {
					databaseError(w, err)
					return
				}
				response.Match = &public
				response.Status = puzzlebattle.PhaseComplete
			}
		}
	}
	response.Incoming, err = queryPuzzleBattleInvites(r.Context(), tx, user.ID, true)
	if err != nil {
		databaseError(w, err)
		return
	}
	response.Outgoing, err = queryPuzzleBattleInvites(r.Context(), tx, user.ID, false)
	if err != nil {
		databaseError(w, err)
		return
	}
	response.History, err = queryPuzzleBattleHistory(r.Context(), tx, user.ID, 10)
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

func (s *webServer) queuePuzzleBattle(w http.ResponseWriter, r *http.Request, user account, request puzzleBattleRequest) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(783094201)"); err != nil {
		databaseError(w, err)
		return
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE last_seen <= $1", now.Add(-puzzlebattle.HeartbeatTimeout)); err != nil {
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
		SELECT q.user_id,q.hero_id
		FROM puzzle_battle_queue q
		WHERE q.user_id<>$1
		  AND q.last_seen>$2
		  AND NOT EXISTS (
			SELECT 1 FROM player_blocks b
			WHERE (b.blocker_id=$1 AND b.blocked_id=q.user_id)
			   OR (b.blocker_id=q.user_id AND b.blocked_id=$1)
		  )
		ORDER BY q.queued_at,q.user_id
		LIMIT 1
		`, user.ID, now.Add(-puzzlebattle.HeartbeatTimeout)).Scan(&opponentID, &opponentHeroID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := lockPuzzleBattlePlayers(r.Context(), tx, user.ID); err != nil {
			databaseError(w, err)
			return
		}
		active, err := puzzleBattlePlayersActive(r.Context(), tx, user.ID)
		if err != nil {
			databaseError(w, err)
			return
		}
		if active {
			writeError(w, http.StatusConflict, errors.New("finish your current Puzzle Battle before queuing again"))
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO puzzle_battle_queue(user_id,hero_id,queued_at,last_seen)
			VALUES($1,$2,$3,$3)
			ON CONFLICT(user_id) DO UPDATE SET hero_id=EXCLUDED.hero_id,queued_at=EXCLUDED.queued_at,last_seen=EXCLUDED.last_seen`,
			user.ID, hero.ID, now); err != nil {
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
		writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: "queued", Incoming: []puzzleBattleInvite{}, Outgoing: []puzzleBattleInvite{}, History: []puzzleBattleHistoryEntry{}})
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
	currentActive, err := puzzleBattlePlayersActive(r.Context(), tx, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if currentActive {
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id=$1", user.ID); err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeError(w, http.StatusConflict, errors.New("finish your current Puzzle Battle before queuing again"))
		return
	}
	opponentActive, err := puzzleBattlePlayersActive(r.Context(), tx, opponentID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if opponentActive {
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id=$1", opponentID); err != nil {
			databaseError(w, err)
			return
		}
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE user_id IN ($1,$2)", user.ID, opponentID); err != nil {
			databaseError(w, err)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO puzzle_battle_queue(user_id,hero_id,queued_at,last_seen)
			VALUES($1,$2,$3,$3)
			ON CONFLICT(user_id) DO UPDATE SET hero_id=EXCLUDED.hero_id,queued_at=EXCLUDED.queued_at,last_seen=EXCLUDED.last_seen`,
			user.ID, hero.ID, now); err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: "queued", Incoming: []puzzleBattleInvite{}, Outgoing: []puzzleBattleInvite{}, History: []puzzleBattleHistoryEntry{}})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO puzzle_battle_queue(user_id,hero_id,queued_at,last_seen)
		VALUES($1,$2,$3,$3)
		ON CONFLICT(user_id) DO UPDATE SET hero_id=EXCLUDED.hero_id,last_seen=EXCLUDED.last_seen`,
		user.ID, hero.ID, now); err != nil {
		databaseError(w, err)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `
		SELECT user_id,hero_id FROM puzzle_battle_queue
		WHERE user_id IN ($1,$2) ORDER BY user_id FOR UPDATE`, user.ID, opponentID)
	if err != nil {
		databaseError(w, err)
		return
	}
	queueHeroes := make(map[string]string, 2)
	for rows.Next() {
		var queuedUserID, queuedHeroID string
		if err := rows.Scan(&queuedUserID, &queuedHeroID); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		queueHeroes[queuedUserID] = queuedHeroID
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		databaseError(w, err)
		return
	}
	rows.Close()
	if queueHeroes[user.ID] == "" || queueHeroes[opponentID] == "" {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO puzzle_battle_queue(user_id,hero_id,queued_at,last_seen)
			VALUES($1,$2,$3,$3)
			ON CONFLICT(user_id) DO UPDATE SET hero_id=EXCLUDED.hero_id,queued_at=EXCLUDED.queued_at,last_seen=EXCLUDED.last_seen`,
			user.ID, hero.ID, now); err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: "queued", Incoming: []puzzleBattleInvite{}, Outgoing: []puzzleBattleInvite{}, History: []puzzleBattleHistoryEntry{}})
		return
	}
	opponentHeroID = queueHeroes[opponentID]
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO puzzle_battle_queue(user_id,hero_id,queued_at,last_seen)
		VALUES($1,$2,$3,$3)
		ON CONFLICT(user_id) DO UPDATE SET hero_id=EXCLUDED.hero_id,queued_at=EXCLUDED.queued_at,last_seen=EXCLUDED.last_seen`,
		user.ID, hero.ID, now); err != nil {
		databaseError(w, err)
		return
	}
	match, err := s.createPuzzleBattleTx(r.Context(), tx, [2]string{opponentID, user.ID}, [2]string{opponentHeroID, hero.ID}, now)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	public, err := buildPuzzleBattlePublicMatch(r.Context(), tx, match, 1, now)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, puzzleBattleResponse{Status: match.Status, Match: &public})
}

func (s *webServer) cancelPuzzleBattleQueue(w http.ResponseWriter, r *http.Request, user account) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(783094201)"); err != nil {
		databaseError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id=$1", user.ID); err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: "idle", Incoming: []puzzleBattleInvite{}, Outgoing: []puzzleBattleInvite{}, History: []puzzleBattleHistoryEntry{}})
}

func (s *webServer) invitePuzzleBattleFriend(w http.ResponseWriter, r *http.Request, user account, request puzzleBattleRequest) {
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
	if _, err := tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(783094201)"); err != nil {
		databaseError(w, err)
		return
	}

	var recipientID string
	if err := tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE lower(public_handle)=lower($1)", handle).Scan(&recipientID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("player not found"))
			return
		}
		databaseError(w, err)
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
	var friend, blocked bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM friendships
			WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)
		), EXISTS (
			SELECT 1 FROM player_blocks
			WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)
		)`, user.ID, recipientID).Scan(&friend, &blocked); err != nil {
		databaseError(w, err)
		return
	}
	if !friend || blocked {
		writeError(w, http.StatusForbidden, errors.New("Puzzle Battle invitations are limited to unblocked friends"))
		return
	}
	active, err := puzzleBattlePlayersActive(r.Context(), tx, user.ID, recipientID)
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
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id IN ($1,$2)", user.ID, recipientID); err != nil {
		databaseError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE user_id IN ($1,$2)", user.ID, recipientID); err != nil {
		databaseError(w, err)
		return
	}
	inviteID := randomID()
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO puzzle_battle_invites(id,sender_id,recipient_id,sender_hero_id)
		VALUES($1,$2,$3,$4)`, inviteID, user.ID, recipientID, hero.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, errors.New("a pending Puzzle Battle invitation already exists between these players"))
		} else {
			databaseError(w, err)
		}
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, puzzleBattleResponse{Status: "invite-sent", InviteID: inviteID})
}

func (s *webServer) respondPuzzleBattleInvite(w http.ResponseWriter, r *http.Request, user account, request puzzleBattleRequest, accept bool) {
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
	if _, err := tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(783094201)"); err != nil {
		databaseError(w, err)
		return
	}

	ownerColumn := "recipient_id"
	if !accept && request.Action == "cancel-invite" {
		ownerColumn = "sender_id"
	}
	var senderID, senderHeroID string
	query := fmt.Sprintf(`
		SELECT sender_id,sender_hero_id FROM puzzle_battle_invites
		WHERE id=$1 AND %s=$2 AND status='pending' AND created_at>$3`, ownerColumn)
	if err := tx.QueryRowContext(r.Context(), query,
		request.InviteID, user.ID, time.Now().UTC().Add(-puzzleBattleInviteLifetime)).Scan(&senderID, &senderHeroID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("pending Puzzle Battle invitation not found"))
			return
		}
		databaseError(w, err)
		return
	}
	if accept {
		if err := lockPuzzleBattlePlayers(r.Context(), tx, senderID, user.ID); err != nil {
			databaseError(w, err)
			return
		}
		active, err := puzzleBattlePlayersActive(r.Context(), tx, senderID, user.ID)
		if err != nil {
			databaseError(w, err)
			return
		}
		if active {
			writeError(w, http.StatusConflict, errors.New("one of these players already has an active match"))
			return
		}
		var friend, blocked bool
		if err := tx.QueryRowContext(r.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM friendships
				WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)
			), EXISTS (
				SELECT 1 FROM player_blocks
				WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)
			)`, senderID, user.ID).Scan(&friend, &blocked); err != nil {
			databaseError(w, err)
			return
		}
		if !friend || blocked {
			writeError(w, http.StatusForbidden, errors.New("this invitation is no longer available"))
			return
		}
		hero, err := puzzleBattleHero(r.Context(), tx, user.ID, request.HeroID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		now := time.Now().UTC()
		result, err := tx.ExecContext(r.Context(), `
			UPDATE puzzle_battle_invites SET status='accepted',recipient_hero_id=$1,responded_at=$2
			WHERE id=$3 AND recipient_id=$4 AND status='pending'`, hero.ID, now, request.InviteID, user.ID)
		if err != nil {
			databaseError(w, err)
			return
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			if err != nil {
				databaseError(w, err)
			} else {
				writeError(w, http.StatusConflict, errors.New("invitation is no longer pending"))
			}
			return
		}
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id IN ($1,$2)", senderID, user.ID); err != nil {
			databaseError(w, err)
			return
		}
		match, err := s.createPuzzleBattleTx(r.Context(), tx, [2]string{senderID, user.ID}, [2]string{senderHeroID, hero.ID}, now)
		if err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		public, err := buildPuzzleBattlePublicMatch(r.Context(), tx, match, 1, now)
		if err != nil {
			databaseError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, puzzleBattleResponse{Status: match.Status, Match: &public})
		return
	}
	status := "declined"
	if strings.TrimSpace(request.Action) == "cancel-invite" {
		status = "expired"
	}
	updateQuery := fmt.Sprintf(`
		UPDATE puzzle_battle_invites SET status=$1,responded_at=now()
		WHERE id=$2 AND status='pending' AND %s=$3`, ownerColumn)
	result, err := tx.ExecContext(r.Context(), updateQuery, status, request.InviteID, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		if err != nil {
			databaseError(w, err)
		} else {
			writeError(w, http.StatusNotFound, errors.New("pending Puzzle Battle invitation not found"))
		}
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: "idle"})
}

func (s *webServer) handlePuzzleBattleAction(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var request puzzleBattleRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	matchID := chi.URLParam(r, "matchID")
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	match, player, err := loadPuzzleBattleMatch(r.Context(), tx, matchID, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("Puzzle Battle not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	wasActive := match.Status != puzzlebattle.PhaseComplete
	now := time.Now().UTC()
	if wasActive {
		if err := match.Heartbeat(player, now); err != nil && match.Status != puzzlebattle.PhaseComplete {
			writeError(w, http.StatusConflict, err)
			return
		}
	}
	if match.Status != puzzlebattle.PhaseComplete {
		switch request.Action {
		case "ready":
			err = match.SetReady(player, now)
		case "toggle-pair":
			err = match.TogglePair(player, request.Index, now)
		case "submit":
			err = match.Submit(player, now)
		default:
			err = errors.New("unknown Puzzle Battle match action")
		}
		if err != nil && match.Status != puzzlebattle.PhaseComplete {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if err := s.savePuzzleBattleTx(r.Context(), tx, match, wasActive, now); err != nil {
		databaseError(w, err)
		return
	}
	public, err := buildPuzzleBattlePublicMatch(r.Context(), tx, match, player, now)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: match.Status, Match: &public})
}

func (s *webServer) handlePuzzleBattleHistory(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	matches, err := queryPuzzleBattleHistory(r.Context(), s.db, user.ID, 50)
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, puzzleBattleHistoryResponse{Matches: matches})
}

func (s *webServer) createPuzzleBattleTx(ctx context.Context, tx *sql.Tx, ids, heroIDs [2]string, now time.Time) (*puzzlebattle.Match, error) {
	if err := lockPuzzleBattlePlayers(ctx, tx, ids[0], ids[1]); err != nil {
		return nil, err
	}
	active, err := puzzleBattlePlayersActive(ctx, tx, ids[0], ids[1])
	if err != nil {
		return nil, fmt.Errorf("check existing Puzzle Battle: %w", err)
	}
	if active {
		return nil, errors.New("one of these players already has an active match")
	}
	var playerHeroes [2]heroes.Hero
	for player := range ids {
		playerHeroes[player], err = puzzleBattleHero(ctx, tx, ids[player], heroIDs[player])
		if err != nil {
			return nil, fmt.Errorf("validate player %d hero: %w", player, err)
		}
	}
	rng, err := secureRand()
	if err != nil {
		return nil, fmt.Errorf("create Puzzle Battle random source: %w", err)
	}
	match, err := puzzlebattle.New(randomID(), ids, heroIDs, playerHeroes, rng, now)
	if err != nil {
		return nil, fmt.Errorf("create Puzzle Battle: %w", err)
	}
	state, err := json.Marshal(match)
	if err != nil {
		return nil, fmt.Errorf("encode Puzzle Battle state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO puzzle_battle_matches(id,player_a_id,player_b_id,player_a_hero_id,player_b_hero_id,state,status)
		VALUES($1,$2,$3,$4,$5,$6,'active')`, match.ID, ids[0], ids[1], heroIDs[0], heroIDs[1], state); err != nil {
		return nil, fmt.Errorf("save new Puzzle Battle: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM puzzle_battle_queue WHERE user_id IN ($1,$2)", ids[0], ids[1]); err != nil {
		return nil, fmt.Errorf("remove matched players from queue: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM online_battle_queue WHERE user_id IN ($1,$2)", ids[0], ids[1]); err != nil {
		return nil, fmt.Errorf("remove matched players from online queue: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE online_battle_invites SET status='expired',responded_at=$1
		WHERE status='pending' AND (sender_id IN ($2,$3) OR recipient_id IN ($2,$3))`,
		now, ids[0], ids[1]); err != nil {
		return nil, fmt.Errorf("close online battle invitations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE puzzle_battle_invites SET status='expired',responded_at=$1
		WHERE status='pending' AND (sender_id IN ($2,$3) OR recipient_id IN ($2,$3))`,
		now, ids[0], ids[1]); err != nil {
		return nil, fmt.Errorf("close stale Puzzle Battle invitations: %w", err)
	}
	return match, nil
}

func loadActivePuzzleBattle(ctx context.Context, tx *sql.Tx, userID string) (*puzzlebattle.Match, int, error) {
	var matchID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM puzzle_battle_matches
		WHERE status='active' AND (player_a_id=$1 OR player_b_id=$1)
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, userID).Scan(&matchID)
	if err != nil {
		return nil, 0, err
	}
	return loadPuzzleBattleMatch(ctx, tx, matchID, userID)
}

func loadLatestCompletedPuzzleBattle(ctx context.Context, tx *sql.Tx, userID string) (*puzzlebattle.Match, int, error) {
	var matchID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM puzzle_battle_matches
		WHERE status='completed' AND (player_a_id=$1 OR player_b_id=$1)
		ORDER BY completed_at DESC LIMIT 1`, userID).Scan(&matchID)
	if err != nil {
		return nil, 0, err
	}
	return loadPuzzleBattleMatch(ctx, tx, matchID, userID)
}

func loadPuzzleBattleMatch(ctx context.Context, tx *sql.Tx, matchID, userID string) (*puzzlebattle.Match, int, error) {
	var playerA, playerB string
	var state []byte
	err := tx.QueryRowContext(ctx, `
		SELECT player_a_id,player_b_id,state FROM puzzle_battle_matches
		WHERE id=$1 AND (player_a_id=$2 OR player_b_id=$2) FOR UPDATE`, matchID, userID).
		Scan(&playerA, &playerB, &state)
	if err != nil {
		return nil, 0, err
	}
	var match puzzlebattle.Match
	if err := json.Unmarshal(state, &match); err != nil {
		return nil, 0, fmt.Errorf("decode Puzzle Battle state: %w", err)
	}
	if match.ID != matchID || match.PlayerIDs != [2]string{playerA, playerB} {
		return nil, 0, errors.New("stored Puzzle Battle participants do not match its state")
	}
	if userID == playerA {
		return &match, 0, nil
	}
	return &match, 1, nil
}

func buildPuzzleBattlePublicMatch(ctx context.Context, queryer puzzleBattleQueryRower, match *puzzlebattle.Match, player int, now time.Time) (puzzleBattlePublicMatch, error) {
	view, err := match.ViewFor(player, now)
	if err != nil {
		return puzzleBattlePublicMatch{}, err
	}
	public := puzzleBattlePublicMatch{View: view}
	for index, userID := range match.PlayerIDs {
		if err := queryer.QueryRowContext(ctx, "SELECT public_handle FROM users WHERE id=$1", userID).Scan(&public.Players[index].Handle); err != nil {
			return puzzleBattlePublicMatch{}, fmt.Errorf("load Puzzle Battle player handle: %w", err)
		}
		hero, ok := heroes.ByID(match.PlayerHeroIDs[index])
		if !ok {
			return puzzleBattlePublicMatch{}, fmt.Errorf("unknown stored Puzzle Battle hero %q", match.PlayerHeroIDs[index])
		}
		public.Players[index].HeroID = hero.ID
		public.Players[index].Hero = hero.Name
		public.Players[index].Draft = match.PlayerDraft[index]
	}
	return public, nil
}

func (s *webServer) savePuzzleBattleTx(ctx context.Context, tx *sql.Tx, match *puzzlebattle.Match, wasActive bool, now time.Time) error {
	state, err := json.Marshal(match)
	if err != nil {
		return fmt.Errorf("encode Puzzle Battle state: %w", err)
	}
	var result any
	status := "active"
	var completedAt any
	if match.Status == puzzlebattle.PhaseComplete {
		status = "completed"
		encoded, err := json.Marshal(match.Result)
		if err != nil {
			return fmt.Errorf("encode Puzzle Battle result: %w", err)
		}
		result = encoded
		completedAt = match.Result.CompletedAt
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE puzzle_battle_matches
		SET state=$1,result=$2,status=$3,player_a_last_seen=$4,player_b_last_seen=$5,
			updated_at=$6,completed_at=COALESCE(completed_at,$7)
		WHERE id=$8 AND status='active'`,
		state, result, status, match.LastHeartbeatAt[0], match.LastHeartbeatAt[1], now, completedAt, match.ID)
	if err != nil {
		return fmt.Errorf("save Puzzle Battle state: %w", err)
	}
	if wasActive && match.Status == puzzlebattle.PhaseComplete {
		if err := creditPuzzleBattleProfiles(ctx, tx, match); err != nil {
			return err
		}
	}
	return nil
}

func lockPuzzleBattlePlayers(ctx context.Context, tx *sql.Tx, ids ...string) error {
	ordered := append([]string(nil), ids...)
	sort.Strings(ordered)
	for _, id := range ordered {
		var lockedID string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", id).Scan(&lockedID); err != nil {
			return fmt.Errorf("lock Puzzle Battle player %s: %w", id, err)
		}
	}
	return nil
}

func puzzleBattlePlayersActive(ctx context.Context, tx *sql.Tx, ids ...string) (bool, error) {
	for _, id := range ids {
		var active bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM puzzle_battle_matches
				WHERE status='active' AND (player_a_id=$1 OR player_b_id=$1)
			) OR EXISTS (
				SELECT 1 FROM online_battle_active_users WHERE user_id=$1
			)`, id).Scan(&active); err != nil {
			return false, err
		}
		if active {
			return true, nil
		}
	}
	return false, nil
}

func queryPuzzleBattleInvites(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID string, incoming bool) ([]puzzleBattleInvite, error) {
	ownerColumn, otherColumn := "sender_id", "recipient_id"
	if incoming {
		ownerColumn, otherColumn = otherColumn, ownerColumn
	}
	rows, err := queryer.QueryContext(ctx, fmt.Sprintf(`
		SELECT i.id,u.public_handle,i.created_at
		FROM puzzle_battle_invites i JOIN users u ON u.id=i.%s
		WHERE i.%s=$1 AND i.status='pending'
		ORDER BY i.created_at`, otherColumn, ownerColumn), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invites := []puzzleBattleInvite{}
	for rows.Next() {
		var invite puzzleBattleInvite
		if err := rows.Scan(&invite.ID, &invite.Handle, &invite.CreatedAt); err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invites, nil
}

func queryPuzzleBattleHistory(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID string, limit int) ([]puzzleBattleHistoryEntry, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT m.id,
			CASE WHEN m.player_a_id=$1 THEN b.public_handle ELSE a.public_handle END,
			CASE WHEN m.player_a_id=$1 THEN m.player_a_hero_id ELSE m.player_b_hero_id END,
			CASE WHEN m.player_a_id=$1 THEN m.player_b_hero_id ELSE m.player_a_hero_id END,
			CASE WHEN m.player_a_id=$1 THEN 0 ELSE 1 END,
			m.result,m.created_at,m.completed_at
		FROM puzzle_battle_matches m
		JOIN users a ON a.id=m.player_a_id
		JOIN users b ON b.id=m.player_b_id
		WHERE m.status='completed' AND (m.player_a_id=$1 OR m.player_b_id=$1)
		ORDER BY m.completed_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matches := []puzzleBattleHistoryEntry{}
	for rows.Next() {
		var item puzzleBattleHistoryEntry
		var playerHero, opponentHero string
		var result []byte
		var completedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Opponent, &playerHero, &opponentHero, &item.Player, &result, &item.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		if result != nil {
			var parsed puzzlebattle.Result
			if err := json.Unmarshal(result, &parsed); err != nil {
				return nil, fmt.Errorf("decode Puzzle Battle history result: %w", err)
			}
			item.Result = &parsed
		}
		if hero, ok := heroes.ByID(playerHero); ok {
			item.PlayerHero = hero.Name
		} else {
			item.PlayerHero = playerHero
		}
		if hero, ok := heroes.ByID(opponentHero); ok {
			item.OpponentHero = hero.Name
		} else {
			item.OpponentHero = opponentHero
		}
		if item.Result != nil {
			item.CompletedAt = item.Result.CompletedAt
		} else if completedAt.Valid {
			item.CompletedAt = completedAt.Time
		}
		matches = append(matches, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}

func creditPuzzleBattleProfiles(ctx context.Context, tx *sql.Tx, match *puzzlebattle.Match) error {
	if match.Result == nil {
		return errors.New("completed Puzzle Battle has no result")
	}
	var players [2]*profile.Profile
	indices := []int{0, 1}
	if match.PlayerIDs[0] > match.PlayerIDs[1] {
		indices[0], indices[1] = 1, 0
	}
	for _, player := range indices {
		var data []byte
		if err := tx.QueryRowContext(ctx, "SELECT profile FROM player_data WHERE user_id=$1 FOR UPDATE", match.PlayerIDs[player]).Scan(&data); err != nil {
			return fmt.Errorf("load Puzzle Battle reward profile: %w", err)
		}
		players[player] = &profile.Profile{}
		if err := json.Unmarshal(data, players[player]); err != nil {
			return fmt.Errorf("decode Puzzle Battle reward profile: %w", err)
		}
	}
	for player, p := range players {
		opponent := 1 - player
		won := match.Result.Winner == player
		tied := match.Result.Draw
		playerScore, opponentScore := match.Result.Scores[player], match.Result.Scores[opponent]
		seat := game.SeatX
		if player == 1 {
			seat = game.SeatY
		}
		jails, captured, clashes := puzzleBattleResultCards(match.Result)
		playerJail, opponentJail := jails[player], jails[opponent]
		gameResult := game.MatchResult{
			Won: won, Tied: tied, PlayerSeat: seat,
			PlayerScore: playerScore, OpponentScore: opponentScore,
			Games: []game.MatchGame{{
				Won: won, PlayerSeat: seat, PlayerScore: playerScore, OpponentScore: opponentScore,
				PlayerJail: playerJail, OpponentJail: opponentJail,
				PlayerCapturedByRound: captured[player], OpponentCapturedByRound: captured[opponent],
				Modifiers: match.Modifiers, PlayerHeroID: match.PlayerHeroIDs[player],
				OpponentHeroID: match.PlayerHeroIDs[opponent], BountySuit: match.BountySuit,
				Clashes: clashes[player], ScoreAfterR2: playerScore, OpponentAfterR2: opponentScore,
			}},
		}
		mode := economy.ModeFriendCasual
		updateAchievementStats(p, gameResult, mode)
		if tied {
			p.TotalDraws++
			p.CurrentStreak = 0
		} else if won {
			p.TotalWins++
			p.CurrentStreak++
			p.WinsSinceCampDraw++
			if p.WinsSinceCampDraw >= lootbox.WinsPerCampDraw {
				if p.DrawsAvailable < lootbox.MaxDrawsStored {
					p.DrawsAvailable++
				}
				p.WinsSinceCampDraw -= lootbox.WinsPerCampDraw
			}
			if p.CurrentStreak > p.BestStreak {
				p.BestStreak = p.CurrentStreak
			}
		} else {
			p.TotalLosses++
			p.CurrentStreak = 0
		}
		payout := awardWebMatchMarks(p, playerScore, won)
		achievementContext := achievements.NewMatchContext(p, gameResult, economy.ModeID(mode), seat.String())
		achievementContext.Now = match.Result.CompletedAt
		for _, event := range achievements.EvaluateAll(achievementContext) {
			applyUnlockEvent(p, event)
		}
		p.RecordMatch(profile.MatchRecord{
			GameID: match.ID, At: match.Result.CompletedAt, Mode: economy.ModeID(mode),
			PlayerSeat: seat.String(), PlayerScore: playerScore, OppScore: opponentScore, Won: won,
			BountySuit:      achievements.SuitRoleID(match.BountySuit),
			PlayerPrisoners: len(playerJail), OppPrisoners: len(opponentJail),
			PlayerPrisonerSuits: puzzleBattleSuitCounts(playerJail), Clashes: clashes[player],
			MarksEarned: payout.TotalMarks,
		})
		if p.HeroPlayCounts == nil {
			p.HeroPlayCounts = make(map[string]int)
		}
		if p.HeroWinCounts == nil {
			p.HeroWinCounts = make(map[string]int)
		}
		p.HeroPlayCounts[match.PlayerHeroIDs[player]]++
		if won {
			p.HeroWinCounts[match.PlayerHeroIDs[player]]++
		}
		p.UpdatedAt = match.Result.CompletedAt
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("encode Puzzle Battle reward profile: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE player_data SET profile=$1,updated_at=$2,version=version+1
			WHERE user_id=$3`, data, match.Result.CompletedAt, match.PlayerIDs[player]); err != nil {
			return fmt.Errorf("save Puzzle Battle rewards: %w", err)
		}
	}
	return nil
}

func puzzleBattleResultCards(result *puzzlebattle.Result) ([2][]game.Card, [2][3][3]int, [2]int) {
	var jails [2][]game.Card
	var captured [2][3][3]int
	var clashes [2]int
	for roundIndex, round := range result.Rounds {
		for side := 0; side < 2; side++ {
			jails[side] = append(jails[side], round.Players[side].Jail...)
			for _, card := range round.Players[side].Jail {
				captured[side][roundIndex][int(card.Suit)]++
			}
			for _, pair := range round.Players[side].PairResults {
				if pair.WinnerPlayer == -1 {
					clashes[side]++
				}
			}
		}
	}
	return jails, captured, clashes
}

func puzzleBattleSuitCounts(cards []game.Card) map[string]int {
	counts := make(map[string]int)
	for _, card := range cards {
		counts[achievements.SuitRoleID(card.Suit)]++
	}
	return counts
}
