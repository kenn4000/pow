package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/profile"
)

var errInsufficientEnergy = errors.New("not enough Energy to start this match")

func matchEnergyCost(matchMode string) int {
	return energy.MatchCost(matchMode == "ranked")
}

func spendAccountEnergyTx(ctx context.Context, tx *sql.Tx, playerIDs []string, amount int, now time.Time) error {
	ordered := append([]string(nil), playerIDs...)
	sort.Strings(ordered)
	for _, playerID := range ordered {
		var data []byte
		if err := tx.QueryRowContext(ctx,
			"SELECT profile FROM player_data WHERE user_id=$1 FOR UPDATE", playerID).Scan(&data); err != nil {
			return fmt.Errorf("load player Energy: %w", err)
		}
		var player profile.Profile
		if err := json.Unmarshal(data, &player); err != nil {
			return fmt.Errorf("decode player Energy: %w", err)
		}
		if err := (&app{}).initializeProgressionProfile(&player); err != nil {
			return err
		}
		if err := energy.Spend(&player, amount, now); err != nil {
			return fmt.Errorf("%w: %v", errInsufficientEnergy, err)
		}
		player.UpdatedAt = now
		data, err := json.Marshal(&player)
		if err != nil {
			return fmt.Errorf("encode player Energy: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE player_data SET profile=$1,updated_at=$2,version=version+1
			WHERE user_id=$3`, data, now, playerID); err != nil {
			return fmt.Errorf("save player Energy: %w", err)
		}
	}
	return nil
}
