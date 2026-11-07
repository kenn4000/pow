package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/puzzlebattle"
)

func (s *webServer) handlePuzzleBattleCPU(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getPuzzleBattleCPU(w, r, user)
	case http.MethodPost:
		var request puzzleBattleRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if request.Action != "start" {
			writeError(w, http.StatusBadRequest, errors.New("unknown Puzzle Battle CPU action"))
			return
		}
		s.startPuzzleBattleCPU(w, r, user, request)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *webServer) getPuzzleBattleCPU(w http.ResponseWriter, r *http.Request, user account) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()

	var matchID string
	active := true
	err = tx.QueryRowContext(r.Context(), `
		SELECT id FROM puzzle_battle_cpu_matches
		WHERE player_id=$1 AND status='active'
		ORDER BY updated_at DESC LIMIT 1 FOR UPDATE`, user.ID).Scan(&matchID)
	if errors.Is(err, sql.ErrNoRows) {
		active = false
		err = tx.QueryRowContext(r.Context(), `
			SELECT id FROM puzzle_battle_cpu_matches
			WHERE player_id=$1 AND status='completed'
			ORDER BY completed_at DESC LIMIT 1`, user.ID).Scan(&matchID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: "idle", CPUPractice: true})
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	match, _, err := loadPuzzleBattleCPUMatch(r.Context(), tx, matchID, user.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if active {
		now := time.Now().UTC()
		match.LastHeartbeatAt = [2]time.Time{now, now}
		if err := match.Advance(now); err != nil {
			databaseError(w, err)
			return
		}
		if err := s.savePuzzleBattleCPUTx(r.Context(), tx, match, true, now); err != nil {
			databaseError(w, err)
			return
		}
	}
	response, err := buildPuzzleBattleCPUResponse(match, time.Now().UTC())
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

func (s *webServer) startPuzzleBattleCPU(w http.ResponseWriter, r *http.Request, user account, request puzzleBattleRequest) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()

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
		writeError(w, http.StatusConflict, errors.New("finish your current online battle before starting a CPU Puzzle Battle"))
		return
	}
	var cpuActive bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM puzzle_battle_cpu_matches
			WHERE player_id=$1 AND status='active'
		)`, user.ID).Scan(&cpuActive); err != nil {
		databaseError(w, err)
		return
	}
	if cpuActive {
		writeError(w, http.StatusConflict, errors.New("you already have an active CPU Puzzle Battle"))
		return
	}
	playerHero, err := puzzleBattleHero(r.Context(), tx, user.ID, request.HeroID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cpuHero, ok := heroes.ByID("recruit")
	if !ok {
		databaseError(w, errors.New("Easy CPU hero is unavailable"))
		return
	}
	rng, err := secureRand()
	if err != nil {
		databaseError(w, fmt.Errorf("create Easy CPU Puzzle Battle random source: %w", err))
		return
	}
	now := time.Now().UTC()
	matchID := randomID()
	match, err := puzzlebattle.New(
		matchID, [2]string{user.ID, "easy-cpu:" + matchID},
		[2]string{playerHero.ID, cpuHero.ID}, [2]heroes.Hero{playerHero, cpuHero}, rng, now,
	)
	if err != nil {
		databaseError(w, fmt.Errorf("create Easy CPU Puzzle Battle: %w", err))
		return
	}
	match.MatchMode = "cpu_easy"
	if err := match.StartImmediately(now); err != nil {
		databaseError(w, err)
		return
	}
	match.Deadline = nil
	state, err := json.Marshal(match)
	if err != nil {
		databaseError(w, fmt.Errorf("encode Easy CPU Puzzle Battle: %w", err))
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO puzzle_battle_cpu_matches(id,player_id,hero_id,status,state,created_at,updated_at)
		VALUES($1,$2,$3,'active',$4,$5,$5)`, matchID, user.ID, playerHero.ID, state, now); err != nil {
		databaseError(w, fmt.Errorf("save Easy CPU Puzzle Battle: %w", err))
		return
	}
	response, err := buildPuzzleBattleCPUResponse(match, now)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (s *webServer) handlePuzzleBattleCPUAction(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var request puzzleBattleRequest
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

	matchID := chi.URLParam(r, "matchID")
	match, status, err := loadPuzzleBattleCPUMatch(r.Context(), tx, matchID, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("CPU Puzzle Battle not found"))
		} else {
			databaseError(w, err)
		}
		return
	}
	wasActive := status == "active"
	now := time.Now().UTC()
	if wasActive {
		match.LastHeartbeatAt = [2]time.Time{now, now}
		if err := match.Advance(now); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		if match.Status != puzzlebattle.PhaseComplete {
			if request.Action != "submit-pair" {
				writeError(w, http.StatusBadRequest, errors.New("unknown CPU Puzzle Battle action"))
				return
			}
			if err := match.SubmitPair(0, request.FirstIndex, request.SecondIndex, now); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if match.Status == puzzlebattle.PhaseRoundOne && match.Rounds[0].Players[0].Submitted ||
				match.Status == puzzlebattle.PhaseRoundTwo && match.Rounds[1].Players[0].Submitted {
				if err := submitPuzzleBattleCPUPairs(match, now); err != nil {
					databaseError(w, err)
					return
				}
			}
		}
		if err := s.savePuzzleBattleCPUTx(r.Context(), tx, match, true, now); err != nil {
			databaseError(w, err)
			return
		}
	}
	response, err := buildPuzzleBattleCPUResponse(match, now)
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

func loadPuzzleBattleCPUMatch(ctx context.Context, tx *sql.Tx, matchID, userID string) (*puzzlebattle.Match, string, error) {
	var playerID, heroID, status string
	var state []byte
	err := tx.QueryRowContext(ctx, `
		SELECT player_id,hero_id,status,state FROM puzzle_battle_cpu_matches
		WHERE id=$1 AND player_id=$2 FOR UPDATE`, matchID, userID).
		Scan(&playerID, &heroID, &status, &state)
	if err != nil {
		return nil, "", err
	}
	var match puzzlebattle.Match
	if err := json.Unmarshal(state, &match); err != nil {
		return nil, "", fmt.Errorf("decode CPU Puzzle Battle: %w", err)
	}
	if match.ID != matchID || match.PlayerIDs != [2]string{userID, "easy-cpu:" + matchID} ||
		match.PlayerHeroIDs != [2]string{heroID, "recruit"} || match.MatchMode != "cpu_easy" {
		return nil, "", errors.New("stored CPU Puzzle Battle does not match its owner")
	}
	return &match, status, nil
}

func buildPuzzleBattleCPUResponse(match *puzzlebattle.Match, now time.Time) (puzzleBattleResponse, error) {
	view, err := match.ViewFor(0, now)
	if err != nil {
		return puzzleBattleResponse{}, err
	}
	players := [2]puzzleBattlePlayerView{
		{Handle: "You"},
		{Handle: "Easy CPU"},
	}
	for index, heroID := range match.PlayerHeroIDs {
		hero, ok := heroes.ByID(heroID)
		if !ok {
			return puzzleBattleResponse{}, fmt.Errorf("unknown stored CPU Puzzle Battle hero %q", heroID)
		}
		players[index].HeroID = hero.ID
		players[index].Hero = hero.Name
		players[index].Draft = match.PlayerDraft[index]
	}
	return puzzleBattleResponse{
		Status: match.Status, MatchMode: "easy", CPUPractice: true,
		Match:  &puzzleBattlePublicMatch{View: view, Players: players},
		Payout: puzzleBattlePayoutForPlayer(match, 0),
	}, nil
}

func (s *webServer) savePuzzleBattleCPUTx(ctx context.Context, tx *sql.Tx, match *puzzlebattle.Match, wasActive bool, now time.Time) error {
	if wasActive && match.Status == puzzlebattle.PhaseComplete {
		var payouts [2]puzzlebattle.RewardPayout
		if err := creditPuzzleBattleProfiles(ctx, tx, match, &payouts); err != nil {
			return err
		}
		match.Payouts = &payouts
	}
	state, err := json.Marshal(match)
	if err != nil {
		return fmt.Errorf("encode CPU Puzzle Battle state: %w", err)
	}
	status := "active"
	var encodedResult any
	var completedAt any
	if match.Status == puzzlebattle.PhaseComplete {
		status = "completed"
		encodedResult, err = json.Marshal(match.Result)
		if err != nil {
			return fmt.Errorf("encode CPU Puzzle Battle result: %w", err)
		}
		completedAt = match.Result.CompletedAt
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE puzzle_battle_cpu_matches
		SET state=$1,result=$2,status=$3,updated_at=$4,
			completed_at=CASE WHEN $3='completed' THEN COALESCE(completed_at,$5) ELSE NULL END
		WHERE id=$6 AND status='active'`, state, encodedResult, status, now, completedAt, match.ID)
	if err != nil {
		return fmt.Errorf("save CPU Puzzle Battle: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check CPU Puzzle Battle update: %w", err)
	}
	if rows != 1 {
		return errors.New("CPU Puzzle Battle is no longer active")
	}
	return nil
}

func submitPuzzleBattleCPUPairs(match *puzzlebattle.Match, now time.Time) error {
	roundIndex := 0
	switch match.Status {
	case puzzlebattle.PhaseRoundOne:
		roundIndex = 0
	case puzzlebattle.PhaseRoundTwo:
		roundIndex = 1
	default:
		return errors.New("CPU Puzzle Battle round is not active")
	}
	round := &match.Rounds[roundIndex]
	requiredPairs := round.Number + 2
	if len(round.Board) < requiredPairs*2 {
		return fmt.Errorf("round %d board is too small for CPU pair selection", round.Number)
	}
	rng, err := secureRand()
	if err != nil {
		return fmt.Errorf("create CPU pair-selection randomness: %w", err)
	}
	indices := rng.Perm(len(round.Board))
	for pairIndex := 0; pairIndex < requiredPairs; pairIndex++ {
		match.LastHeartbeatAt[1] = now
		if err := match.SubmitPair(1, indices[pairIndex*2], indices[pairIndex*2+1], now); err != nil {
			return fmt.Errorf("submit CPU pair %d: %w", pairIndex+1, err)
		}
	}
	return nil
}
