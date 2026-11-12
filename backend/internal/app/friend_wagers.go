package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"prisonersofwar/internal/profile"
)

const (
	friendWagerPerGameLimit int64 = 1000
	friendWagerDailyLimit   int64 = 5000
)

func reserveFriendWagerTx(ctx context.Context, tx *sql.Tx, playerIDs [2]string, stake int64) error {
	if stake == 0 {
		return nil
	}
	if stake < 0 || stake > friendWagerPerGameLimit {
		return errors.New("friend wagers must be between 0 and 1,000 Marks per player")
	}
	if err := lockPuzzleBattlePlayers(ctx, tx, playerIDs[:]...); err != nil {
		return fmt.Errorf("lock wager profiles: %w", err)
	}
	ordered := []string{playerIDs[0], playerIDs[1]}
	if ordered[0] > ordered[1] {
		ordered[0], ordered[1] = ordered[1], ordered[0]
	}
	profiles := make(map[string]*profile.Profile, 2)
	for _, id := range ordered {
		var data []byte
		if err := tx.QueryRowContext(ctx,
			"SELECT profile FROM player_data WHERE user_id=$1 FOR UPDATE", id).Scan(&data); err != nil {
			return fmt.Errorf("load wager profile: %w", err)
		}
		p := &profile.Profile{}
		if err := json.Unmarshal(data, p); err != nil {
			return fmt.Errorf("decode wager profile: %w", err)
		}
		profiles[id] = p
	}
	now := time.Now().UTC()
	for _, id := range playerIDs {
		var used int64
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(stake_marks),0) FROM friend_wagers
			WHERE (player_a_id=$1 OR player_b_id=$1) AND created_at >= $2`,
			id, now.Add(-24*time.Hour)).Scan(&used); err != nil {
			return fmt.Errorf("check rolling wager limit: %w", err)
		}
		if used+stake > friendWagerDailyLimit {
			return fmt.Errorf("this wager would exceed the 5,000 Marks rolling 24-hour limit; %d Marks are already wagered", used)
		}
		if profiles[id].Marks < stake {
			return fmt.Errorf("each player needs %d Marks available to accept this wager", stake)
		}
	}
	for _, id := range playerIDs {
		p := profiles[id]
		p.Marks -= stake
		p.UpdatedAt = now
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("encode wager profile: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE player_data SET profile=$1,updated_at=$2,version=version+1 WHERE user_id=$3`,
			data, now, id); err != nil {
			return fmt.Errorf("reserve wager Marks: %w", err)
		}
	}
	return nil
}

func createFriendWagerTx(ctx context.Context, tx *sql.Tx, gameType, matchID string, playerIDs [2]string, stake int64) error {
	if stake == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO friend_wagers(match_id,game_type,player_a_id,player_b_id,stake_marks)
		VALUES($1,$2,$3,$4,$5)`, matchID, gameType, playerIDs[0], playerIDs[1], stake)
	if err != nil {
		return fmt.Errorf("save friend wager: %w", err)
	}
	return nil
}

func settleFriendWagerTx(ctx context.Context, tx *sql.Tx, matchID, gameType string, winner int, now time.Time) error {
	var playerA, playerB string
	var stake int64
	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT player_a_id,player_b_id,stake_marks,status FROM friend_wagers
		WHERE match_id=$1 AND game_type=$2 FOR UPDATE`, matchID, gameType).
		Scan(&playerA, &playerB, &stake, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load friend wager settlement: %w", err)
	}
	if status == "settled" {
		return nil
	}
	if winner < -1 || winner > 1 {
		return fmt.Errorf("invalid friend wager winner %d", winner)
	}
	playerIDs := [2]string{playerA, playerB}
	if err := lockPuzzleBattlePlayers(ctx, tx, playerIDs[:]...); err != nil {
		return fmt.Errorf("lock wager settlement profiles: %w", err)
	}
	ordered := []string{playerA, playerB}
	if ordered[0] > ordered[1] {
		ordered[0], ordered[1] = ordered[1], ordered[0]
	}
	profiles := make(map[string]*profile.Profile, 2)
	for _, id := range ordered {
		var data []byte
		if err := tx.QueryRowContext(ctx,
			"SELECT profile FROM player_data WHERE user_id=$1 FOR UPDATE", id).Scan(&data); err != nil {
			return fmt.Errorf("load wager settlement profile: %w", err)
		}
		p := &profile.Profile{}
		if err := json.Unmarshal(data, p); err != nil {
			return fmt.Errorf("decode wager settlement profile: %w", err)
		}
		profiles[id] = p
	}
	if winner == -1 {
		profiles[playerA].Marks += stake
		profiles[playerB].Marks += stake
	} else {
		profiles[playerIDs[winner]].Marks += 2 * stake
	}
	for _, id := range ordered {
		p := profiles[id]
		p.UpdatedAt = now
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("encode wager settlement profile: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE player_data SET profile=$1,updated_at=$2,version=version+1 WHERE user_id=$3`,
			data, now, id); err != nil {
			return fmt.Errorf("settle wager Marks: %w", err)
		}
	}
	var persistedWinner any
	if winner >= 0 {
		persistedWinner = winner
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE friend_wagers SET status='settled',winner_player=$1,settled_at=$2
		WHERE match_id=$3 AND game_type=$4 AND status='active'`,
		persistedWinner, now, matchID, gameType)
	if err != nil {
		return fmt.Errorf("save friend wager result: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check friend wager result update: %w", err)
	}
	if rows != 1 {
		return errors.New("friend wager was already settled")
	}
	return nil
}
