package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/ranked"
)

func updateVariantRatingsTx(ctx context.Context, tx *sql.Tx, ids [2]string, variant string, winner int, now time.Time) error {
	if winner == -1 {
		return nil
	}
	if winner != 0 && winner != 1 {
		return errors.New("ranked winner must be -1, 0, or 1")
	}
	ordered := append([]string(nil), ids[:]...)
	sort.Strings(ordered)
	ratings := make(map[string]int, len(ids))
	for _, id := range ordered {
		var rating int
		if err := tx.QueryRowContext(ctx, `
			SELECT rating FROM user_variant_ratings
			WHERE user_id=$1 AND variant=$2 FOR UPDATE`, id, variant).Scan(&rating); err != nil {
			return fmt.Errorf("load %s rating for player %s: %w", variant, id, err)
		}
		ratings[id] = rating
	}
	nextA, nextB, err := ranked.UpdatedRatings(ratings[ids[0]], ratings[ids[1]], winner)
	if err != nil {
		return err
	}
	nextA = clampInt(nextA, 0, 10000)
	nextB = clampInt(nextB, 0, 10000)
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_variant_ratings SET rating=$1,updated_at=$2
		WHERE user_id=$3 AND variant=$4`, nextA, now, ids[0], variant); err != nil {
		return fmt.Errorf("save %s rating for player %s: %w", variant, ids[0], err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_variant_ratings SET rating=$1,updated_at=$2
		WHERE user_id=$3 AND variant=$4`, nextB, now, ids[1], variant); err != nil {
		return fmt.Errorf("save %s rating for player %s: %w", variant, ids[1], err)
	}
	return nil
}

func onlineBattleScores(engine *webServer) [2]int {
	session := engine.session
	var scores [2]int
	switch session.variant {
	case "main", "fog":
		scores[0] = game.ScorePlayer(session.state.P1, session.state.Bounty.Suit, session.state.Modifiers).Total
		scores[1] = game.ScorePlayer(session.state.P2, session.state.Bounty.Suit, session.state.Modifiers).Total
	case "rummy":
		if session.rummy != nil {
			scores[0], scores[1] = session.rummy.Results[0].Total, session.rummy.Results[1].Total
		}
	case "poker-duel":
		if session.pokerDuel != nil {
			for player, score := range session.pokerDuel.FinalScores {
				if player < len(scores) {
					scores[player] = int(math.Round(score.FinalScore))
				}
			}
		}
	case "abandon-duel":
		if session.abandonDuel != nil {
			scores = session.abandonDuel.TotalScores
		}
	case "tug-war":
		if session.tugWar != nil {
			scores = session.tugWar.Result().Scores
		}
	case "ladder":
		if session.winner >= 0 {
			scores[session.winner] = 1
		}
	}
	return scores
}

func (s *webServer) creditOnlineBattleProfiles(ctx context.Context, tx *sql.Tx, match onlineBattleStoredMatch, engine *webServer, now time.Time) error {
	ids := [2]string{match.PlayerA, match.PlayerB}
	if err := lockPuzzleBattlePlayers(ctx, tx, ids[:]...); err != nil {
		return err
	}
	scores := onlineBattleScores(engine)
	winner := engine.session.winner
	seriesTie := match.MatchMode == "ranked" && engine.session.rankedSeries != nil &&
		engine.session.rankedSeries.Complete && engine.session.rankedSeries.Draw
	profiles := [2]*profile.Profile{}
	ordered := []int{0, 1}
	if ids[0] > ids[1] {
		ordered[0], ordered[1] = 1, 0
	}
	for _, player := range ordered {
		var data []byte
		if err := tx.QueryRowContext(ctx, "SELECT profile FROM player_data WHERE user_id=$1 FOR UPDATE", ids[player]).Scan(&data); err != nil {
			return fmt.Errorf("load online battle reward profile: %w", err)
		}
		profiles[player] = &profile.Profile{}
		if err := json.Unmarshal(data, profiles[player]); err != nil {
			return fmt.Errorf("decode online battle reward profile: %w", err)
		}
	}
	payoutMode := economy.ModeFriendCasual
	if match.MatchMode == "ranked" {
		payoutMode = economy.ModeRanked
	}
	for player, p := range profiles {
		won := winner == player || seriesTie
		draw := winner == -1
		payout, err := economy.CalculateMultiplayerPayout(
			match.Variant, scores[player], won, draw, won, "", payoutMode, p, now,
		)
		if err != nil {
			return fmt.Errorf("calculate online battle rewards: %w", err)
		}
		if won {
			p.TotalWins++
			p.CurrentStreak++
			if p.CurrentStreak > p.BestStreak {
				p.BestStreak = p.CurrentStreak
			}
			p.WinsSinceCampDraw++
			if p.WinsSinceCampDraw >= lootbox.RequiredWins(p) {
				if p.DrawsAvailable < lootbox.MaxDrawsStored {
					p.DrawsAvailable++
				}
				p.WinsSinceCampDraw = 0
			}
		} else if draw {
			p.TotalDraws++
			p.CurrentStreak = 0
		} else {
			p.TotalLosses++
			p.CurrentStreak = 0
		}
		if p.HeroPlayCounts == nil {
			p.HeroPlayCounts = make(map[string]int)
		}
		if p.HeroWinCounts == nil {
			p.HeroWinCounts = make(map[string]int)
		}
		heroID := [2]string{match.HeroA, match.HeroB}[player]
		p.HeroPlayCounts[heroID]++
		if won {
			p.HeroWinCounts[heroID]++
		}
		result := onlineBattleAchievementResult(engine, player, heroID, won, draw || seriesTie, scores)
		updateAchievementStatsForGameMode(p, result, payoutMode, match.Variant, now)
		achievementContext := achievements.NewMatchContext(p, result, economy.ModeID(payoutMode), result.PlayerSeat.String())
		achievementContext.Now = now
		achievementContext.GameMode = match.Variant
		for _, event := range achievements.EvaluateAll(achievementContext) {
			applyUnlockEvent(p, event)
		}
		p.RecordMatch(profile.MatchRecord{
			GameID: match.ID, At: now, Mode: economy.ModeID(payoutMode),
			PlayerSeat: result.PlayerSeat.String(), PlayerScore: scores[player],
			OppScore: scores[1-player], Won: won, MarksEarned: payout.TotalMarks,
		})
		p.UpdatedAt = now
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("encode online battle rewards: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE player_data SET profile=$1,updated_at=$2,version=version+1
			WHERE user_id=$3`, data, now, ids[player]); err != nil {
			return fmt.Errorf("save online battle rewards: %w", err)
		}
	}
	if match.MatchMode == "ranked" && winner >= 0 {
		if err := updateVariantRatingsTx(ctx, tx, ids, match.Variant, winner, now); err != nil {
			return err
		}
	}
	return nil
}

func onlineBattleAchievementResult(engine *webServer, player int, heroID string, won, tied bool, scores [2]int) game.MatchResult {
	playerState, opponentState := &engine.session.state.P1, &engine.session.state.P2
	if player == 1 {
		playerState, opponentState = opponentState, playerState
	}
	seat := playerState.Seat
	result := game.MatchResult{
		Won: won, Tied: tied, PlayerSeat: seat, PlayerScore: scores[player],
		OpponentScore: scores[1-player],
	}
	matchGame := game.MatchGame{
		Won: won, PlayerSeat: seat, PlayerScore: scores[player], OpponentScore: scores[1-player],
		PlayerHeroID: heroID,
	}
	if engine.session.variant == "main" || engine.session.variant == "fog" {
		matchGame.PlayerJail = playerState.Jail
		matchGame.OpponentJail = opponentState.Jail
		matchGame.PlayerCapturedByRound = playerState.CapturedByRound
		matchGame.OpponentCapturedByRound = opponentState.CapturedByRound
		matchGame.Modifiers = engine.session.state.Modifiers
		matchGame.BountySuit = engine.session.state.Bounty.Suit
		matchGame.Clashes = engine.session.state.Clashes
		roundTwoScores := engine.scoresAfterRound(2)
		if player == 0 {
			matchGame.ScoreAfterR2, matchGame.OpponentAfterR2 = roundTwoScores[0], roundTwoScores[1]
		} else {
			matchGame.ScoreAfterR2, matchGame.OpponentAfterR2 = roundTwoScores[1], roundTwoScores[0]
		}
	}
	result.Games = []game.MatchGame{matchGame}
	return result
}
