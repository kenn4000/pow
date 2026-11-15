package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type socialRating struct {
	Rating      int `json:"rating"`
	GamesPlayed int `json:"gamesPlayed"`
}

type socialRecord struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Draws  int `json:"draws"`
}

type socialConversation struct {
	Handle    string    `json:"handle"`
	Preview   string    `json:"preview"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *webServer) rankedRatings(ctx context.Context, userID string) (map[string]socialRating, error) {
	ratings := make(map[string]socialRating, 8)
	variants := []string{"main", "fog", "rummy", "puzzle-battle", "poker-duel", "ladder", "tug-war", "abandon-duel"}
	for _, variant := range variants {
		ratings[variant] = socialRating{Rating: 1000}
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT variant,rating FROM user_variant_ratings WHERE user_id=$1", userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var variant string
		var rating int
		if err := rows.Scan(&variant, &rating); err != nil {
			rows.Close()
			return nil, err
		}
		entry := ratings[variant]
		entry.Rating = rating
		ratings[variant] = entry
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, `
		SELECT variant,SUM(games_played) FROM (
			SELECT variant,player_a_id AS user_id,
				COALESCE((state #>> '{session,RankedSeries,gamesPlayed}')::integer,0) AS games_played
			FROM online_battle_matches WHERE status IN ('active','completed') AND match_mode='ranked'
			UNION ALL
			SELECT variant,player_b_id,
				COALESCE((state #>> '{session,RankedSeries,gamesPlayed}')::integer,0)
			FROM online_battle_matches WHERE status IN ('active','completed') AND match_mode='ranked'
			UNION ALL
			SELECT 'puzzle-battle',player_a_id,
				COALESCE((state #>> '{rankedSeries,gamesPlayed}')::integer,0)
			FROM puzzle_battle_matches WHERE status IN ('active','completed') AND match_mode='ranked'
			UNION ALL
			SELECT 'puzzle-battle',player_b_id,
				COALESCE((state #>> '{rankedSeries,gamesPlayed}')::integer,0)
			FROM puzzle_battle_matches WHERE status IN ('active','completed') AND match_mode='ranked'
		) ranked_games WHERE user_id=$1 GROUP BY variant`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var variant string
		var count int
		if err := rows.Scan(&variant, &count); err != nil {
			return nil, err
		}
		entry := ratings[variant]
		entry.GamesPlayed = count
		ratings[variant] = entry
	}
	return ratings, rows.Err()
}

func (s *webServer) friendRecord(ctx context.Context, userID, friendID string) (socialRecord, int64, error) {
	var record socialRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(wins),0),COALESCE(SUM(losses),0),COALESCE(SUM(draws),0)
		FROM (
			SELECT
				CASE WHEN winner=-1 THEN 0 WHEN winner=viewer_player THEN 1 ELSE 0 END AS wins,
				CASE WHEN winner=-1 OR winner=viewer_player THEN 0 ELSE 1 END AS losses,
				CASE WHEN winner=-1 THEN 1 ELSE 0 END AS draws
			FROM (
				SELECT
					CASE WHEN player_a_id=$1 THEN 0 ELSE 1 END AS viewer_player,
					COALESCE((state #>> '{session,Winner}')::integer,-1) AS winner
				FROM online_battle_matches
				WHERE status='completed' AND match_mode='friend'
					AND ((player_a_id=$1 AND player_b_id=$2) OR (player_a_id=$2 AND player_b_id=$1))
				UNION ALL
				SELECT
					CASE WHEN player_a_id=$1 THEN 0 ELSE 1 END,
					COALESCE((result->>'winner')::integer,-1)
				FROM puzzle_battle_matches
				WHERE status='completed' AND match_mode='friend'
					AND ((player_a_id=$1 AND player_b_id=$2) OR (player_a_id=$2 AND player_b_id=$1))
			) matches
		) outcomes`, userID, friendID).
		Scan(&record.Wins, &record.Losses, &record.Draws)
	if err != nil {
		return socialRecord{}, 0, err
	}
	var net int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE
			WHEN winner_player IS NULL THEN 0
			WHEN player_a_id=$1 AND winner_player=0 THEN stake_marks
			WHEN player_b_id=$1 AND winner_player=1 THEN stake_marks
			ELSE -stake_marks END),0)
		FROM friend_wagers
		WHERE status='settled' AND ((player_a_id=$1 AND player_b_id=$2) OR (player_a_id=$2 AND player_b_id=$1))`,
		userID, friendID).Scan(&net)
	return record, net, err
}

func (s *webServer) socialConversations(ctx context.Context, userID string) ([]socialConversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH visible AS (
			SELECT m.*,
				CASE WHEN m.sender_id=$1 THEN m.recipient_id ELSE m.sender_id END AS other_id
			FROM social_messages m
			WHERE (m.sender_id=$1 OR m.recipient_id=$1)
				AND m.created_at > COALESCE((
					SELECT d.deleted_at FROM social_conversation_deletions d
					WHERE d.owner_id=$1
						AND d.other_id=CASE WHEN m.sender_id=$1 THEN m.recipient_id ELSE m.sender_id END
				),'-infinity'::timestamptz)
		), latest AS (
			SELECT DISTINCT ON (other_id) other_id,body,created_at
			FROM visible ORDER BY other_id,created_at DESC,id DESC
		)
		SELECT u.public_handle,latest.body,latest.created_at
		FROM latest JOIN users u ON u.id=latest.other_id
		ORDER BY latest.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]socialConversation, 0)
	for rows.Next() {
		var item socialConversation
		if err := rows.Scan(&item.Handle, &item.Preview, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *webServer) handleSocialConversations(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	conversations, err := s.socialConversations(r.Context(), user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": conversations})
}

func (s *webServer) handleSocialConversation(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	handle := strings.TrimSpace(chi.URLParam(r, "handle"))
	var targetID string
	if err := s.db.QueryRowContext(r.Context(),
		"SELECT id FROM users WHERE lower(public_handle)=lower($1)", handle).Scan(&targetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("player not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	if targetID == user.ID {
		writeError(w, http.StatusBadRequest, errors.New("you cannot open a conversation with yourself"))
		return
	}
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT deleted_at FROM social_conversation_deletions
		WHERE owner_id=$1 AND other_id=$2`, user.ID, targetID).Scan(&deletedAt); err != nil && !errors.Is(err, sql.ErrNoRows) {
		databaseError(w, err)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT m.id,s.public_handle,d.public_handle,m.body,m.created_at
		FROM social_messages m
		JOIN users s ON s.id=m.sender_id
		JOIN users d ON d.id=m.recipient_id
		WHERE ((m.sender_id=$1 AND m.recipient_id=$2) OR (m.sender_id=$2 AND m.recipient_id=$1))
			AND ($3::timestamptz IS NULL OR m.created_at>$3)
		ORDER BY m.created_at,m.id`, user.ID, targetID, deletedAt)
	if err != nil {
		databaseError(w, err)
		return
	}
	messages := make([]socialMessage, 0)
	for rows.Next() {
		var item socialMessage
		if err := rows.Scan(&item.ID, &item.SenderHandle, &item.RecipientHandle, &item.Body, &item.CreatedAt); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		messages = append(messages, item)
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
	var blockedByMe, blockedByOther bool
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM player_blocks WHERE blocker_id=$1 AND blocked_id=$2),
			EXISTS(SELECT 1 FROM player_blocks WHERE blocker_id=$2 AND blocked_id=$1)`,
		user.ID, targetID).Scan(&blockedByMe, &blockedByOther); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"handle": handle, "messages": messages, "blockedByMe": blockedByMe, "blockedByOther": blockedByOther,
	})
}

func (s *webServer) handleSocialDeleteConversation(w http.ResponseWriter, r *http.Request) {
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
	input.Handle = strings.TrimSpace(input.Handle)
	var targetID string
	if err := s.db.QueryRowContext(r.Context(),
		"SELECT id FROM users WHERE lower(public_handle)=lower($1)", input.Handle).Scan(&targetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("player not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	if targetID == user.ID {
		writeError(w, http.StatusBadRequest, errors.New("you cannot delete a conversation with yourself"))
		return
	}
	_, err := s.db.ExecContext(r.Context(), `
		INSERT INTO social_conversation_deletions(owner_id,other_id,deleted_at)
		VALUES($1,$2,now())
		ON CONFLICT(owner_id,other_id) DO UPDATE SET deleted_at=EXCLUDED.deleted_at`,
		user.ID, targetID)
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *webServer) handleSocialUnfriend(w http.ResponseWriter, r *http.Request) {
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
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var targetID string
	if err := tx.QueryRowContext(r.Context(),
		"SELECT id FROM users WHERE lower(public_handle)=lower($1)", handle).Scan(&targetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("player not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		DELETE FROM friendships WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)`,
		user.ID, targetID)
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
		writeError(w, http.StatusNotFound, errors.New("friend not found"))
		return
	}
	now := time.Now().UTC()
	for _, table := range []string{"online_battle_invites", "puzzle_battle_invites"} {
		query := fmt.Sprintf(`
			UPDATE %s SET status='expired',responded_at=$1
			WHERE status='pending' AND ((sender_id=$2 AND recipient_id=$3) OR (sender_id=$3 AND recipient_id=$2))`, table)
		if _, err := tx.ExecContext(r.Context(), query, now, user.ID, targetID); err != nil {
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
