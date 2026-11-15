package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	challengeAvailabilityLifetime = 2 * time.Minute
	challengeDeclineCooldown      = 2 * time.Minute
)

var challengeVariants = map[string]bool{
	"main": true, "fog": true, "rummy": true, "abandon-duel": true,
	"poker-duel": true, "ladder": true, "tug-war": true, "puzzle-battle": true,
}

func challengePreferenceKey(variant, matchMode string) string {
	return variant + ":" + matchMode
}

func validChallengePreference(value string) bool {
	variant, matchMode, ok := strings.Cut(value, ":")
	return ok && challengeVariants[variant] && (matchMode == "online" || matchMode == "ranked")
}

func saveChallengePreferencesTx(ctx context.Context, tx *sql.Tx, userID string, accepted []string, now time.Time) error {
	seen := make(map[string]bool, len(accepted))
	for _, value := range accepted {
		if !validChallengePreference(value) {
			return fmt.Errorf("unsupported online challenge preference %q", value)
		}
		if seen[value] {
			return errors.New("online challenge preferences contain a duplicate")
		}
		seen[value] = true
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO online_challenge_preferences(user_id,accepted_challenges,last_seen)
		VALUES($1,$2,$3)
		ON CONFLICT(user_id) DO UPDATE SET accepted_challenges=EXCLUDED.accepted_challenges,
			last_seen=EXCLUDED.last_seen`, userID, accepted, now)
	return err
}

func refreshChallengeAvailabilityTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) ([]string, int, error) {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO online_challenge_preferences(user_id,last_seen) VALUES($1,$2)
		ON CONFLICT(user_id) DO UPDATE SET last_seen=EXCLUDED.last_seen`, userID, now); err != nil {
		return nil, 0, fmt.Errorf("refresh online challenge availability: %w", err)
	}
	accepted := []string{}
	if err := tx.QueryRowContext(ctx,
		"SELECT accepted_challenges FROM online_challenge_preferences WHERE user_id=$1", userID).
		Scan(&accepted); err != nil {
		return nil, 0, fmt.Errorf("load online challenge preferences: %w", err)
	}
	cooldownSeconds, err := challengeCooldownSecondsTx(ctx, tx, userID, now)
	if err != nil {
		return nil, 0, err
	}
	if cooldownSeconds == 0 {
		if err := matchQueuedChallengeForRecipientTx(ctx, tx, userID, accepted, now); err != nil {
			return nil, 0, err
		}
	}
	return accepted, cooldownSeconds, nil
}

func challengeCooldownSecondsTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (int, error) {
	var seconds int
	err := tx.QueryRowContext(ctx, `
		SELECT GREATEST(0,CEIL(EXTRACT(EPOCH FROM (blocked_until-$2))))::integer
		FROM online_challenge_cooldowns WHERE user_id=$1`, userID, now).Scan(&seconds)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load online challenge cooldown: %w", err)
	}
	return seconds, nil
}

func requireChallengeCooldownExpiredTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) error {
	seconds, err := challengeCooldownSecondsTx(ctx, tx, userID, now)
	if err != nil {
		return err
	}
	if seconds > 0 {
		return fmt.Errorf("you cannot send or receive another challenge for %d more seconds", seconds)
	}
	return nil
}

func setChallengeDeclineCooldownTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO online_challenge_cooldowns(user_id,blocked_until)
		VALUES($1,$2)
		ON CONFLICT(user_id) DO UPDATE SET blocked_until=EXCLUDED.blocked_until`,
		userID, now.Add(challengeDeclineCooldown)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE online_battle_invites SET status='expired',responded_at=$2
		WHERE status='pending' AND (sender_id=$1 OR recipient_id=$1)`, userID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE puzzle_battle_invites SET status='expired',responded_at=$2
		WHERE status='pending' AND (sender_id=$1 OR recipient_id=$1)`, userID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM online_battle_queue WHERE user_id=$1", userID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM puzzle_battle_queue WHERE user_id=$1", userID)
	return err
}

func validateIncomingChallengeTx(ctx context.Context, tx *sql.Tx, senderID, recipientID, matchMode string) error {
	var friend, blocked bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM friendships
			WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)
		), EXISTS (
			SELECT 1 FROM player_blocks
			WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)
		)`, senderID, recipientID).Scan(&friend, &blocked); err != nil {
		return fmt.Errorf("check online challenge relationship: %w", err)
	}
	if blocked {
		return errors.New("this challenge is no longer available")
	}
	if matchMode == "friend" && !friend {
		return errors.New("friend challenges are limited to unblocked friends")
	}
	return nil
}

func (s *webServer) setOnlineChallengePreferences(w http.ResponseWriter, r *http.Request, user account, request onlineBattleRequest) {
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
	if err := saveChallengePreferencesTx(r.Context(), tx, user.ID, request.AcceptedChallenges, now); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cooldown, err := challengeCooldownSecondsTx(r.Context(), tx, user.ID, now)
	if err != nil {
		databaseError(w, err)
		return
	}
	if cooldown == 0 {
		if err := matchQueuedChallengeForRecipientTx(r.Context(), tx, user.ID, request.AcceptedChallenges, now); err != nil {
			databaseError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, onlineBattleResponse{
		Status: "idle", AcceptedChallenges: request.AcceptedChallenges,
		MatchmakingCooldownSeconds: cooldown,
	})
}

func (s *webServer) queueOnlineChallenge(w http.ResponseWriter, r *http.Request, user account, request onlineBattleRequest) {
	if !challengeVariants[request.Variant] {
		writeError(w, http.StatusBadRequest, errors.New("choose a supported gameplay variant"))
		return
	}
	if request.MatchMode != "online" && request.MatchMode != "ranked" {
		writeError(w, http.StatusBadRequest, errors.New("choose unranked or ranked matchmaking"))
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
	if err := requireChallengeCooldownExpiredTx(r.Context(), tx, user.ID, now); err != nil {
		writeError(w, http.StatusTooManyRequests, err)
		return
	}
	if err := lockPuzzleBattlePlayers(r.Context(), tx, user.ID); err != nil {
		databaseError(w, err)
		return
	}
	onlineActive, err := onlineBattlePlayersActive(r.Context(), tx, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	puzzleActive, err := puzzleBattlePlayersActive(r.Context(), tx, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if onlineActive || puzzleActive {
		writeError(w, http.StatusConflict, errors.New("finish your current online match before matchmaking"))
		return
	}

	if request.Variant == "puzzle-battle" {
		hero, err := puzzleBattleHero(r.Context(), tx, user.ID, request.HeroID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM online_battle_queue WHERE user_id=$1", user.ID); err != nil {
			databaseError(w, err)
			return
		}
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO puzzle_battle_queue(user_id,hero_id,queued_at,last_seen,match_mode,challenge_matchmaking)
			VALUES($1,$2,$3,$3,$4,true)
			ON CONFLICT(user_id) DO UPDATE SET hero_id=EXCLUDED.hero_id,queued_at=EXCLUDED.queued_at,
				last_seen=EXCLUDED.last_seen,match_mode=EXCLUDED.match_mode,challenge_matchmaking=true`,
			user.ID, hero.ID, now, request.MatchMode)
	} else {
		hero, _, err := onlineBattleHeroWithConductor(
			r.Context(), tx, user.ID, request.HeroID, request.ConductorModifier, request.ConductorAmount,
		)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM puzzle_battle_queue WHERE user_id=$1", user.ID); err != nil {
			databaseError(w, err)
			return
		}
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO online_battle_queue(
				user_id,variant,hero_id,queued_at,last_seen,match_mode,conductor_modifier,conductor_amount,challenge_matchmaking
			) VALUES($1,$2,$3,$4,$4,$5,$6,$7,true)
			ON CONFLICT(user_id) DO UPDATE SET variant=EXCLUDED.variant,hero_id=EXCLUDED.hero_id,
				queued_at=EXCLUDED.queued_at,last_seen=EXCLUDED.last_seen,match_mode=EXCLUDED.match_mode,
				conductor_modifier=EXCLUDED.conductor_modifier,conductor_amount=EXCLUDED.conductor_amount,
				challenge_matchmaking=true`,
			user.ID, request.Variant, hero.ID, now, request.MatchMode,
			request.ConductorModifier, request.ConductorAmount)
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, onlineBattleResponse{
		Status: "queued", Variant: request.Variant, MatchMode: request.MatchMode,
		Incoming: []onlineBattleInvite{}, Outgoing: []onlineBattleInvite{},
	})
}

type queuedOnlineChallenge struct {
	source          string
	senderID        string
	variant         string
	matchMode       string
	heroID          string
	conductor       string
	conductorAmount int
}

func matchQueuedChallengeForRecipientTx(ctx context.Context, tx *sql.Tx, recipientID string, accepted []string, now time.Time) error {
	if len(accepted) == 0 {
		return nil
	}
	var active, hasPendingInvite bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM online_battle_active_users WHERE user_id=$1)
			OR EXISTS(SELECT 1 FROM puzzle_battle_matches
				WHERE status='active' AND (player_a_id=$1 OR player_b_id=$1))`, recipientID).Scan(&active); err != nil {
		return fmt.Errorf("check challenge recipient activity: %w", err)
	}
	if active {
		return nil
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM online_battle_invites WHERE recipient_id=$1 AND status='pending')
			OR EXISTS(SELECT 1 FROM puzzle_battle_invites WHERE recipient_id=$1 AND status='pending')`,
		recipientID).Scan(&hasPendingInvite); err != nil {
		return fmt.Errorf("check pending challenges: %w", err)
	}
	if hasPendingInvite {
		return nil
	}

	var challenge queuedOnlineChallenge
	err := tx.QueryRowContext(ctx, `
		SELECT source,sender_id,variant,match_mode,hero_id,conductor_modifier,conductor_amount
		FROM (
			SELECT 'online'::text AS source,q.user_id AS sender_id,q.variant,q.match_mode,q.hero_id,
				q.conductor_modifier,q.conductor_amount,rating.rating
			FROM online_battle_queue q
			JOIN user_variant_ratings rating ON rating.user_id=q.user_id AND rating.variant=q.variant
			WHERE q.challenge_matchmaking AND q.last_seen>$2
				AND (q.variant || ':' || q.match_mode)=ANY($3)
			UNION ALL
			SELECT 'puzzle'::text AS source,q.user_id AS sender_id,'puzzle-battle'::text AS variant,
				q.match_mode,q.hero_id,''::text AS conductor_modifier,0::integer AS conductor_amount,rating.rating
			FROM puzzle_battle_queue q
			JOIN user_variant_ratings rating ON rating.user_id=q.user_id AND rating.variant='puzzle-battle'
			WHERE q.challenge_matchmaking AND q.last_seen>$2
				AND ('puzzle-battle:' || q.match_mode)=ANY($3)
		) queued
		WHERE sender_id<>$1
			AND NOT EXISTS (SELECT 1 FROM friendships f
				WHERE f.user_a=LEAST(sender_id,$1) AND f.user_b=GREATEST(sender_id,$1))
			AND NOT EXISTS (SELECT 1 FROM player_blocks b
				WHERE (b.blocker_id=sender_id AND b.blocked_id=$1)
					OR (b.blocker_id=$1 AND b.blocked_id=sender_id))
			AND NOT EXISTS (SELECT 1 FROM online_battle_active_users a WHERE a.user_id=sender_id)
			AND NOT EXISTS (SELECT 1 FROM puzzle_battle_matches a
				WHERE a.status='active' AND (a.player_a_id=sender_id OR a.player_b_id=sender_id))
			AND NOT EXISTS (SELECT 1 FROM online_challenge_cooldowns c
				WHERE c.user_id=sender_id AND c.blocked_until>$4)
			AND NOT EXISTS (SELECT 1 FROM online_battle_invites i
				WHERE i.status='pending' AND (i.sender_id=sender_id OR i.recipient_id=sender_id))
			AND NOT EXISTS (SELECT 1 FROM puzzle_battle_invites i
				WHERE i.status='pending' AND (i.sender_id=sender_id OR i.recipient_id=sender_id))
		ORDER BY ABS(queued.rating - COALESCE((
			SELECT target.rating FROM user_variant_ratings target
			WHERE target.user_id=$1 AND target.variant=queued.variant),1000)),queued.sender_id
		LIMIT 1`, recipientID, now.Add(-onlineBattleQueueLifetime), accepted, now).
		Scan(&challenge.source, &challenge.senderID, &challenge.variant, &challenge.matchMode,
			&challenge.heroID, &challenge.conductor, &challenge.conductorAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find closest online challenge: %w", err)
	}

	inviteID := randomID()
	if challenge.source == "puzzle" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO puzzle_battle_invites(id,sender_id,recipient_id,sender_hero_id,match_mode)
			VALUES($1,$2,$3,$4,$5)`,
			inviteID, challenge.senderID, recipientID, challenge.heroID, challenge.matchMode)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM puzzle_battle_queue WHERE user_id=$1", challenge.senderID)
		}
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO online_battle_invites(
				id,variant,sender_id,recipient_id,sender_hero_id,sender_conductor_modifier,
				sender_conductor_amount,wager_marks,match_mode
			) VALUES($1,$2,$3,$4,$5,$6,$7,0,$8)`,
			inviteID, challenge.variant, challenge.senderID, recipientID, challenge.heroID,
			challenge.conductor, challenge.conductorAmount, challenge.matchMode)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM online_battle_queue WHERE user_id=$1", challenge.senderID)
		}
	}
	if err != nil {
		return fmt.Errorf("create matched online challenge: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM online_battle_queue WHERE user_id=$1", recipientID); err != nil {
		return fmt.Errorf("remove challenge recipient from online queue: %w", err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM puzzle_battle_queue WHERE user_id=$1", recipientID)
	if err != nil {
		return fmt.Errorf("remove challenge recipient from queues: %w", err)
	}
	return nil
}
