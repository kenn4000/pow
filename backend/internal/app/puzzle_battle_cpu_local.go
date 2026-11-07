package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/puzzlebattle"
)

func (s *webServer) handleLocalPuzzleBattleCPU(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getLocalPuzzleBattleCPU(w)
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
		s.startLocalPuzzleBattleCPU(w, request)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *webServer) getLocalPuzzleBattleCPU(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	match := s.cpuPuzzleBattle
	if match == nil {
		writeJSON(w, http.StatusOK, puzzleBattleResponse{Status: "idle", CPUPractice: true})
		return
	}
	now := time.Now().UTC()
	if match.Status != puzzlebattle.PhaseComplete {
		match.LastHeartbeatAt = [2]time.Time{now, now}
		if err := match.Advance(now); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
	}
	if match.Status == puzzlebattle.PhaseComplete && match.Payouts == nil {
		if err := s.settleLocalPuzzleBattleCPU(match); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	response, err := buildPuzzleBattleCPUResponse(match, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *webServer) startLocalPuzzleBattleCPU(w http.ResponseWriter, request puzzleBattleRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.playerProfile == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("local player profile is unavailable"))
		return
	}
	if s.cpuPuzzleBattle != nil && s.cpuPuzzleBattle.Status != puzzlebattle.PhaseComplete {
		writeError(w, http.StatusConflict, errors.New("finish your current CPU Puzzle Battle before starting another"))
		return
	}
	playerHeroID := s.requestedHeroID(request.HeroID)
	playerHero, ok := heroes.ByID(playerHeroID)
	if !ok || !s.heroUnlocked(playerHeroID) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("player hero %q is not unlocked", playerHeroID))
		return
	}
	cpuHero, ok := heroes.ByID("recruit")
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("Easy CPU hero is unavailable"))
		return
	}
	rng, err := secureRand()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("create Easy CPU Puzzle Battle random source: %w", err))
		return
	}
	now := time.Now().UTC()
	matchID := randomID()
	match, err := puzzlebattle.New(
		matchID, [2]string{"local-player", "easy-cpu:" + matchID},
		[2]string{playerHero.ID, cpuHero.ID}, [2]heroes.Hero{playerHero, cpuHero}, rng, now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("create Easy CPU Puzzle Battle: %w", err))
		return
	}
	match.MatchMode = "cpu_easy"
	if err := match.StartImmediately(now); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	match.Deadline = nil
	response, err := buildPuzzleBattleCPUResponse(match, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.cpuPuzzleBattle = match
	writeJSON(w, http.StatusCreated, response)
}

func (s *webServer) handleLocalPuzzleBattleCPUAction(w http.ResponseWriter, r *http.Request) {
	var request puzzleBattleRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	match := s.cpuPuzzleBattle
	if match == nil || match.ID != chi.URLParam(r, "matchID") {
		writeError(w, http.StatusNotFound, errors.New("CPU Puzzle Battle not found"))
		return
	}
	now := time.Now().UTC()
	if match.Status != puzzlebattle.PhaseComplete {
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
					writeError(w, http.StatusInternalServerError, err)
					return
				}
			}
		}
	}
	if match.Status == puzzlebattle.PhaseComplete && match.Payouts == nil {
		if err := s.settleLocalPuzzleBattleCPU(match); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	response, err := buildPuzzleBattleCPUResponse(match, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *webServer) settleLocalPuzzleBattleCPU(match *puzzlebattle.Match) error {
	if s.playerProfile == nil {
		return errors.New("local player profile is unavailable")
	}
	var payouts [2]puzzlebattle.RewardPayout
	payout, err := creditPuzzleBattleProfile(s.playerProfile, match, 0)
	if err != nil {
		return err
	}
	payouts[0] = payout
	match.Payouts = &payouts
	if err := s.playerProfile.Save(); err != nil {
		return fmt.Errorf("save Easy CPU Puzzle Battle rewards: %w", err)
	}
	return nil
}
