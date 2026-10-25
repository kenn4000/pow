package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
)

type sessionSnapshot struct {
	State               game.GameState
	Mode                string
	WhitePlayer         int
	Phase               string
	Actor               int
	HandoffTarget       int
	HandoffPhase        string
	Exchange            int
	Attacker            int
	AttackIndex         int
	AttackCard          game.Card
	LastAttack          game.Card
	LastDefense         game.Card
	LastOutcome         game.Outcome
	HasLastBattle       bool
	Slots               [9]jailCell
	Message             string
	Winner              int
	Tiebreak            string
	PlayerHeroID        string
	OpponentHeroID      string
	ProfileError        string
	Payout              *webMatchPayout
	Campaign            *webCampaignContext
	Battles             []webBattleRecord
	InitialAchievements map[string]int
	AchievementUpdates  []string
}

func snapshotSession(s *webSession) *sessionSnapshot {
	if s == nil {
		return nil
	}
	return &sessionSnapshot{s.state, s.mode, s.whitePlayer, s.phase, s.actor, s.handoffTarget, s.handoffPhase, s.exchange, s.attacker, s.attackIndex, s.attackCard, s.lastAttack, s.lastDefense, s.lastOutcome, s.hasLastBattle, s.slots, s.message, s.winner, s.tiebreak, s.playerHeroID, s.opponentHeroID, s.profileError, s.payout, s.campaign, s.battles, s.initialAchievements, s.achievementUpdates}
}

func (s *sessionSnapshot) restore() *webSession {
	if s == nil {
		return nil
	}
	return &webSession{s.State, s.Mode, s.WhitePlayer, s.Phase, s.Actor, s.HandoffTarget, s.HandoffPhase, s.Exchange, s.Attacker, s.AttackIndex, s.AttackCard, s.LastAttack, s.LastDefense, s.LastOutcome, s.HasLastBattle, s.Slots, s.Message, s.Winner, s.Tiebreak, s.PlayerHeroID, s.OpponentHeroID, s.ProfileError, s.Payout, s.Campaign, s.Battles, s.InitialAchievements, s.AchievementUpdates}
}

type playerSnapshot struct {
	Schema              int                      `json:"schema"`
	GameID              string                   `json:"gameId"`
	StartedAt           time.Time                `json:"startedAt"`
	Session             *sessionSnapshot         `json:"session,omitempty"`
	PreviewProfile      *profile.Profile         `json:"previewProfile,omitempty"`
	CampDraw            *lootbox.SessionSnapshot `json:"campDraw,omitempty"`
	CampDrawPreview     bool                     `json:"campDrawPreview"`
	LastCampDraw        *lootbox.CampDrawResult  `json:"lastCampDraw,omitempty"`
	LastCampDrawPreview bool                     `json:"lastCampDrawPreview"`
}

type bufferedResponse struct {
	bytes.Buffer
	header http.Header
	status int
}

func (b *bufferedResponse) Header() http.Header { return b.header }
func (b *bufferedResponse) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *bufferedResponse) Write(data []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.Buffer.Write(data)
}
func (b *bufferedResponse) send(w http.ResponseWriter) {
	for key, values := range b.header {
		w.Header()[key] = values
	}
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	w.Write(b.Bytes())
}

func (s *webServer) withPlayer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(accountKey).(account)
		body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		if err != nil || len(body) > 1<<20 {
			writeError(w, http.StatusRequestEntityTooLarge, errors.New("request body too large"))
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			databaseError(w, err)
			return
		}
		defer tx.Rollback()
		var profileJSON, stateJSON []byte
		var version int64
		err = tx.QueryRowContext(ctx, "SELECT profile,state,version FROM player_data WHERE user_id=$1 FOR UPDATE", u.ID).Scan(&profileJSON, &stateJSON, &version)
		if err != nil {
			databaseError(w, err)
			return
		}
		if match := r.Header.Get("If-Match"); match != "" && match != strconv.FormatInt(version, 10) {
			writeError(w, http.StatusConflict, errors.New("game state changed; refresh before trying again"))
			return
		}
		current, snapshot, err := s.restorePlayer(profileJSON, stateJSON)
		if err != nil {
			databaseError(w, err)
			return
		}
		response := &bufferedResponse{header: make(http.Header)}
		next.ServeHTTP(response, r.WithContext(context.WithValue(r.Context(), gameServerKey, current)))
		if response.status >= 400 {
			response.send(w)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/new") {
			if _, err := tx.ExecContext(ctx, "UPDATE games SET status='abandoned',updated_at=now() WHERE user_id=$1 AND status='active'", u.ID); err != nil {
				databaseError(w, err)
				return
			}
			snapshot.GameID, snapshot.StartedAt = randomID(), time.Now().UTC()
		}
		snapshot.Schema = 1
		snapshot.Session = snapshotSession(current.session)
		snapshot.PreviewProfile = current.previewProfile
		snapshot.CampDraw = current.campDrawSession.Snapshot()
		snapshot.CampDrawPreview = current.campDrawPreview
		snapshot.LastCampDraw = current.lastCampDraw
		snapshot.LastCampDrawPreview = current.lastCampDrawPreview
		attachMatchRecordID(current, snapshot.GameID)
		stateJSON, profileJSON, err = encodePlayerSnapshot(snapshot, current.playerProfile)
		if err != nil {
			databaseError(w, err)
			return
		}
		if r.Method == http.MethodPost && snapshot.GameID != "" && (strings.HasSuffix(r.URL.Path, "/new") || strings.HasSuffix(r.URL.Path, "/action")) {
			if err := saveGame(ctx, tx, u.ID, snapshot, current, stateJSON, body); err != nil {
				databaseError(w, err)
				return
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE player_data SET profile=$1,state=$2,version=version+1,updated_at=now() WHERE user_id=$3", profileJSON, stateJSON, u.ID)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			databaseError(w, err)
			return
		}
		response.header.Set("X-State-Version", strconv.FormatInt(version+1, 10))
		response.send(w)
	})
}

func encodePlayerSnapshot(snapshot playerSnapshot, p *profile.Profile) ([]byte, []byte, error) {
	state, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(p)
	return state, data, err
}

func (s *webServer) restorePlayer(profileJSON, stateJSON []byte) (*webServer, playerSnapshot, error) {
	var p profile.Profile
	var snapshot playerSnapshot
	if err := json.Unmarshal(profileJSON, &p); err != nil {
		return nil, snapshot, err
	}
	if err := json.Unmarshal(stateJSON, &snapshot); err != nil {
		return nil, snapshot, err
	}
	if snapshot.Schema > 1 || p.SchemaVersion > profile.SchemaVersion {
		return nil, snapshot, errors.New("saved data is newer than this server")
	}
	if snapshot.Session != nil && snapshot.Session.State.Draft != nil {
		draft := snapshot.Session.State.Draft
		if draft.Mode == 0 {
			draft.Mode = 3
			snapshot.Session.State.DraftMode = 3
		}
		if draft.Mode >= 1 && draft.Mode <= 3 &&
			len(draft.DealtX) == 6-draft.Mode && len(draft.DealtY) == 5-draft.Mode &&
			len(draft.DrawPile) == 13 {
			card := draft.DealtX[len(draft.DealtX)-1]
			draft.DealtX = draft.DealtX[:len(draft.DealtX)-1]
			draft.DrawPile = append([]game.Card{card}, draft.DrawPile...)
		}
	}
	p.SetSaveHandler(func(*profile.Profile) error { return nil })
	rng, err := secureRand()
	if err != nil {
		return nil, snapshot, err
	}
	current := &webServer{rng: rng, defaultMode: s.defaultMode, playerProfile: &p, session: snapshot.Session.restore(), previewProfile: snapshot.PreviewProfile, campDrawSession: lootbox.RestoreSession(snapshot.CampDraw), campDrawPreview: snapshot.CampDrawPreview, lastCampDraw: snapshot.LastCampDraw, lastCampDrawPreview: snapshot.LastCampDrawPreview}
	current.gameID = snapshot.GameID
	if err := (&app{}).initializeProgressionProfile(&p); err != nil {
		return nil, snapshot, err
	}
	return current, snapshot, nil
}

func saveGame(ctx context.Context, tx *sql.Tx, userID string, snapshot playerSnapshot, current *webServer, state, action []byte) error {
	status := "active"
	var record []byte
	if current.session.phase == "gameover" {
		status = "completed"
		var err error
		record, err = completedGameRecord(current)
		if err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO games(id,user_id,mode,status,state,record,started_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,CASE WHEN $4='completed' THEN now() END) ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status,state=EXCLUDED.state,record=EXCLUDED.record,updated_at=now(),completed_at=COALESCE(games.completed_at,EXCLUDED.completed_at)`, snapshot.GameID, userID, current.session.mode, status, state, record, snapshot.StartedAt)
	if err != nil {
		return err
	}
	view, err := json.Marshal(current.view())
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO game_events(game_id,sequence,action,view,state) SELECT $1,COALESCE(max(sequence),0)+1,$2,$3,$4 FROM game_events WHERE game_id=$1`, snapshot.GameID, action, view, state)
	return err
}

func completedGameRecord(current *webServer) ([]byte, error) {
	session := current.session
	if session.campaign != nil && session.campaign.Replay {
		view := current.view()
		return json.Marshal(profile.MatchRecord{At: time.Now().UTC(), PlayerScore: view.Players[0].Score, OppScore: view.Players[1].Score, Won: session.winner == 0})
	}
	if len(current.playerProfile.MatchHistory) > 0 {
		return json.Marshal(current.playerProfile.MatchHistory[0])
	}
	return nil, nil
}

func attachMatchRecordID(current *webServer, id string) {
	if current.session == nil || current.session.phase != "gameover" || len(current.playerProfile.MatchHistory) == 0 {
		return
	}
	if current.session.campaign != nil && current.session.campaign.Replay {
		return
	}
	current.playerProfile.MatchHistory[0].GameID = id
}

func (s *webServer) handleGames(w http.ResponseWriter, r *http.Request, u account) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	if chi.URLParam(r, "gameID") == "" {
		rows, err := s.db.QueryContext(r.Context(), `SELECT id,mode,status,record,started_at,completed_at,`+gameCampaignNameSQL+` FROM games WHERE user_id=$1 AND ($2='' OR (started_at,id)<(SELECT started_at,id FROM games WHERE id=$2 AND user_id=$1)) ORDER BY started_at DESC,id DESC LIMIT 51`, u.ID, r.URL.Query().Get("before"))
		if err != nil {
			databaseError(w, err)
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, mode, status, campaign string
			var record []byte
			var started time.Time
			var completed sql.NullTime
			if err := rows.Scan(&id, &mode, &status, &record, &started, &completed, &campaign); err != nil {
				databaseError(w, err)
				return
			}
			items = append(items, map[string]any{"id": id, "mode": mode, "status": status, "record": json.RawMessage(record), "startedAt": started, "completedAt": completed.Time, "campaignName": campaign})
		}
		if err := rows.Err(); err != nil {
			databaseError(w, err)
			return
		}
		next := ""
		if len(items) > 50 {
			items = items[:50]
			next = items[49]["id"].(string)
		}
		writeJSON(w, http.StatusOK, map[string]any{"games": items, "next": next})
		return
	}
	id := chi.URLParam(r, "gameID")
	var mode, status, campaign string
	var started time.Time
	err := s.db.QueryRowContext(r.Context(), "SELECT mode,status,started_at,"+gameCampaignNameSQL+" FROM games WHERE id=$1 AND user_id=$2", id, u.ID).Scan(&mode, &status, &started, &campaign)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("game not found"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	after := int64(0)
	if value := r.URL.Query().Get("after"); value != "" {
		var err error
		after, err = strconv.ParseInt(value, 10, 64)
		if err != nil || after < 0 {
			writeError(w, http.StatusBadRequest, errors.New("invalid event cursor"))
			return
		}
	}
	rows, err := s.db.QueryContext(r.Context(), "SELECT sequence,action,view,created_at FROM game_events WHERE game_id=$1 AND sequence>$2 ORDER BY sequence LIMIT 101", id, after)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var sequence int64
		var action, view json.RawMessage
		var at time.Time
		if err := rows.Scan(&sequence, &action, &view, &at); err != nil {
			databaseError(w, err)
			return
		}
		items = append(items, map[string]any{"sequence": sequence, "action": action, "view": view, "at": at})
	}
	if err := rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	next := ""
	if len(items) > 100 {
		items = items[:100]
		next = fmt.Sprint(items[99]["sequence"])
	}
	writeJSON(w, http.StatusOK, map[string]any{"gameId": id, "events": items, "next": next, "game": map[string]any{"id": id, "mode": mode, "status": status, "startedAt": started, "campaignName": campaign}})
}

// gameCampaignNameSQL selects the campaign mission name recorded in a game's
// first event, or an empty string for ordinary matches.
const gameCampaignNameSQL = `COALESCE((SELECT e.view->'campaign'->>'name' FROM game_events e WHERE e.game_id=games.id ORDER BY e.sequence LIMIT 1),'')`
