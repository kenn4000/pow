package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"prisonersofwar/achievements"
	"prisonersofwar/ai"
	"prisonersofwar/campaigns"
	"prisonersofwar/catalog"
	"prisonersofwar/economy"
	"prisonersofwar/energy"
	"prisonersofwar/game"
	"prisonersofwar/heroes"
	"prisonersofwar/lootbox"
	"prisonersofwar/profile"
	"prisonersofwar/skills"
	"prisonersofwar/skins"
	"prisonersofwar/storage"
)

var webFiles = skins.Files

type webServer struct {
	db                  *sql.DB
	mu                  sync.Mutex
	rng                 *rand.Rand
	defaultMode         string
	session             *webSession
	playerProfile       *profile.Profile
	previewProfile      *profile.Profile
	campDrawSession     *lootbox.CampDrawSession
	campDrawPreview     bool
	lastCampDraw        *lootbox.CampDrawResult
	lastCampDrawPreview bool
}

type webSession struct {
	state          game.GameState
	mode           string
	whitePlayer    int
	phase          string
	actor          int
	handoffTarget  int
	handoffPhase   string
	exchange       int
	attacker       int
	attackIndex    int
	attackCard     game.Card
	lastAttack     game.Card
	lastDefense    game.Card
	lastOutcome    game.Outcome
	hasLastBattle  bool
	slots          [9]jailCell
	message        string
	winner         int
	tiebreak       string
	playerHeroID   string
	opponentHeroID string
	profileError   string
	payout         *webMatchPayout
}

type webMatchPayout struct {
	PlayerScore   int   `json:"playerScore"`
	PlayerWon     bool  `json:"playerWon"`
	BaseMarks     int64 `json:"baseMarks"`
	WinMultiplier int   `json:"winMultiplier"`
	SkillBonusPct int   `json:"skillBonusPercent"`
	TotalMarks    int64 `json:"totalMarks"`
}

type webBattleOutcome struct {
	AttackerValue int `json:"attackerValue"`
	DefenderValue int `json:"defenderValue"`
	Winner        int `json:"winner"`
}

type jailCell struct {
	Card       *game.Card `json:"card,omitempty"`
	CapturedBy string     `json:"capturedBy,omitempty"`
	Clash      bool       `json:"clash,omitempty"`
}

type webCard struct {
	Card       game.Card `json:"card"`
	Index      int       `json:"index"`
	Selectable bool      `json:"selectable"`
}

type webPlayer struct {
	Color         string    `json:"color"`
	You           bool      `json:"you"`
	Seat          string    `json:"seat"`
	Score         int       `json:"score"`
	Hand          []webCard `json:"hand"`
	HiddenHand    int       `json:"hiddenHand"`
	Pool          []webCard `json:"pool"`
	HiddenPool    int       `json:"hiddenPool"`
	CanSelectHand bool      `json:"canSelectHand"`
	CanSelectPool bool      `json:"canSelectPool"`
}

type webView struct {
	Started        bool               `json:"started"`
	Dashboard      *webDashboard      `json:"dashboard,omitempty"`
	Mode           string             `json:"mode,omitempty"`
	Phase          string             `json:"phase,omitempty"`
	Round          int                `json:"round,omitempty"`
	Exchange       int                `json:"exchange,omitempty"`
	Actor          int                `json:"actor"`
	Attacker       int                `json:"attacker"`
	Bounty         game.Card          `json:"bounty"`
	Players        []webPlayer        `json:"players,omitempty"`
	JailSlots      []jailCell         `json:"jailSlots,omitempty"`
	BattleAttack   *game.Card         `json:"battleAttack,omitempty"`
	BattleDefend   *game.Card         `json:"battleDefend,omitempty"`
	BattleOutcome  *webBattleOutcome  `json:"battleOutcome,omitempty"`
	Message        string             `json:"message,omitempty"`
	ScoreLine      string             `json:"scoreLine,omitempty"`
	CanContinue    bool               `json:"canContinue"`
	Winner         int                `json:"winner"`
	Payout         *webMatchPayout    `json:"payout,omitempty"`
	Tiebreak       string             `json:"tiebreak,omitempty"`
	Modifiers      game.GameModifiers `json:"modifiers"`
	PlayerHero     string             `json:"playerHero,omitempty"`
	OpponentHero   string             `json:"opponentHero,omitempty"`
	PlayerHeroID   string             `json:"playerHeroId,omitempty"`
	OpponentHeroID string             `json:"opponentHeroId,omitempty"`
	DefaultHeroID  string             `json:"defaultHeroId,omitempty"`
	OpponentHint   string             `json:"opponentHint,omitempty"`
	Heroes         []webHeroOption    `json:"heroes,omitempty"`
}

type webDashboard struct {
	Commander          string                `json:"commander"`
	Title              string                `json:"title"`
	Marks              int64                 `json:"marks"`
	Crowns             int64                 `json:"crowns"`
	Energy             int                   `json:"energy"`
	MaxEnergy          int                   `json:"maxEnergy"`
	EnergyNextSeconds  int                   `json:"energyNextSeconds"`
	AdsLeft            int                   `json:"adsLeft"`
	DailyFeaturedItem  *webDailyFeaturedItem `json:"dailyFeaturedItem,omitempty"`
	LoginStreakCurrent int                   `json:"loginStreakCurrent"`
	LoginStreakBest    int                   `json:"loginStreakBest"`
	WinsSinceCampDraw  int                   `json:"winsSinceCampDraw"`
	WinsPerCampDraw    int                   `json:"winsPerCampDraw"`
	DrawsAvailable     int                   `json:"drawsAvailable"`
	PendingBoxes       int                   `json:"pendingBoxes"`
	SkillPoints        int                   `json:"skillPoints"`
	UnseenAchievements int                   `json:"unseenAchievements"`
}

type webDailyFeaturedItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Rarity   string `json:"rarity"`
	Currency string `json:"currency"`
	Cost     int64  `json:"cost"`
	Date     string `json:"date"`
	Owned    bool   `json:"owned"`
}

type webHeroOption struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Unlocked bool   `json:"unlocked"`
}

type actionRequest struct {
	Action         string `json:"action"`
	Mode           string `json:"mode"`
	Index          int    `json:"index"`
	Cards          []int  `json:"cards"`
	Seat           string `json:"seat"`
	PlayerHeroID   string `json:"playerHeroId"`
	OpponentHeroID string `json:"opponentHeroId"`
	ID             string `json:"id"`
	Payment        string `json:"payment"`
	BoxType        string `json:"boxType"`
	Suit           string `json:"suit"`
}

type webAchievement struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Category string               `json:"category"`
	Details  string               `json:"details"`
	Hidden   bool                 `json:"hidden,omitempty"`
	Unlocked bool                 `json:"unlocked"`
	New      bool                 `json:"new"`
	Progress int                  `json:"progress"`
	Goal     int                  `json:"goal"`
	Tiers    []webAchievementTier `json:"tiers,omitempty"`
}

type webAchievementTier struct {
	Name   string `json:"name"`
	Goal   int    `json:"goal"`
	Marks  int64  `json:"marks"`
	Crowns int64  `json:"crowns"`
}

type webAchievementSet struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Complete bool   `json:"complete"`
	Progress int    `json:"progress"`
	Total    int    `json:"total"`
}

type webAchievementsPage struct {
	Items     []webAchievement    `json:"items"`
	Sets      []webAchievementSet `json:"sets"`
	Dashboard *webDashboard       `json:"dashboard"`
}

type webShopChroma struct {
	Chroma catalog.Chroma `json:"chroma"`
	Owned  bool           `json:"owned"`
}

type webShopBundle struct {
	Bundle catalog.Bundle `json:"bundle"`
	Owned  bool           `json:"owned"`
}

type webShopHero struct {
	Hero   webHeroOption `json:"hero"`
	Owned  bool          `json:"owned"`
	CanBuy bool          `json:"canBuy"`
	Plays  int           `json:"plays"`
	Wins   int           `json:"wins"`
}

type webShopBoxOffer struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Currency  string `json:"currency"`
	Cost      int64  `json:"cost"`
	Available int    `json:"available"`
}

const (
	shopBoxMarksCost        int64 = 1000
	shopGoldenBoxCrownsCost int64 = 1000
)

type webShopPage struct {
	Chromas            []webShopChroma       `json:"chromas"`
	Bundles            []webShopBundle       `json:"bundles"`
	Heroes             []webShopHero         `json:"heroes"`
	BoxOffers          []webShopBoxOffer     `json:"boxOffers"`
	Dashboard          *webDashboard         `json:"dashboard"`
	HeroBoxesAvailable int                   `json:"heroBoxesAvailable"`
	HeroBoxCost        int64                 `json:"heroBoxCost"`
	DirectHeroCost     int64                 `json:"directHeroCost"`
	ChromaPickTokens   int                   `json:"chromaPickTokens"`
	Message            string                `json:"message,omitempty"`
	HeroBoxResult      *heroes.HeroBoxResult `json:"heroBoxResult,omitempty"`
}

type webSkillNode struct {
	Node                skills.Node `json:"node"`
	Unlocked            bool        `json:"unlocked"`
	CanUnlockCrowns     bool        `json:"canUnlockCrowns"`
	CanUnlockSkillPoint bool        `json:"canUnlockSkillPoint"`
	CrownsReason        string      `json:"crownsReason,omitempty"`
	SkillPointReason    string      `json:"skillPointReason,omitempty"`
}

type webSkillsPage struct {
	Nodes     []webSkillNode `json:"nodes"`
	Dashboard *webDashboard  `json:"dashboard"`
	Message   string         `json:"message,omitempty"`
}

type webHeroesPage struct {
	Heroes    []webHeroOption `json:"heroes"`
	DefaultID string          `json:"defaultId"`
	Dashboard *webDashboard   `json:"dashboard"`
	Message   string          `json:"message,omitempty"`
}

type webCampDrawPage struct {
	DrawsAvailable int                      `json:"drawsAvailable"`
	PendingBoxes   map[string]int           `json:"pendingBoxes"`
	Dashboard      *webDashboard            `json:"dashboard"`
	Started        bool                     `json:"started"`
	StartingHand   []game.Card              `json:"startingHand,omitempty"`
	Prompt         *lootbox.CampDrawPrompt  `json:"prompt,omitempty"`
	Exchanges      []lootbox.ExchangeResult `json:"exchanges,omitempty"`
	Result         *lootbox.CampDrawResult  `json:"result,omitempty"`
	BoxResult      *lootbox.BoxOpenResult   `json:"boxResult,omitempty"`
}

type webCommanderPage struct {
	Dashboard     *webDashboard `json:"dashboard"`
	PlayerID      string        `json:"playerId"`
	CreatedAt     time.Time     `json:"createdAt"`
	TotalGames    int           `json:"totalGames"`
	TotalWins     int           `json:"totalWins"`
	TotalLosses   int           `json:"totalLosses"`
	TotalDraws    int           `json:"totalDraws"`
	CurrentStreak int           `json:"currentStreak"`
	BestStreak    int           `json:"bestStreak"`
	TotalClashes  int           `json:"totalClashes"`
	MatchTies     int           `json:"matchTies"`
	WinsAsSeatX   int           `json:"winsAsSeatX"`
	WinsAsSeatY   int           `json:"winsAsSeatY"`
}

type webCampaignPage struct {
	Dashboard *webDashboard        `json:"dashboard"`
	Campaigns []webCampaignSummary `json:"campaigns"`
}

type webCampaignSummary struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	Requires          string               `json:"requires,omitempty"`
	Unlocked          bool                 `json:"unlocked"`
	Started           bool                 `json:"started"`
	Completed         bool                 `json:"completed"`
	MissionsCompleted int                  `json:"missionsCompleted"`
	TotalMissions     int                  `json:"totalMissions"`
	Missions          []webCampaignMission `json:"missions"`
}

type webCampaignMission struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RewardMarks int64  `json:"rewardMarks"`
	Completed   bool   `json:"completed"`
}

type webHistoryPage struct {
	Dashboard   *webDashboard     `json:"dashboard"`
	TotalGames  int               `json:"totalGames"`
	TotalWins   int               `json:"totalWins"`
	TotalLosses int               `json:"totalLosses"`
	TotalDraws  int               `json:"totalDraws"`
	Matches     []webHistoryMatch `json:"matches"`
}

type webHistoryMatch struct {
	Record   profile.MatchRecord `json:"record"`
	ModeName string              `json:"modeName"`
}

func startWebUI(listenAddress, defaultMode string) error {
	server := &webServer{rng: rand.New(rand.NewSource(time.Now().UnixNano())), defaultMode: defaultMode}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" && os.Getenv("PGHOST") != "" {
		port := os.Getenv("PGPORT")
		if port == "" {
			port = "5432"
		}
		password := os.Getenv("PGPASSWORD")
		if password == "" {
			password = os.Getenv("POSTGRES_PASSWORD")
		}
		address := net.JoinHostPort(os.Getenv("PGHOST"), port)
		u := url.URL{Scheme: "postgres", Host: address, Path: os.Getenv("PGDATABASE"), User: url.UserPassword(os.Getenv("PGUSER"), password)}
		u.RawQuery = "sslmode=disable"
		dsn = u.String()
	}
	if dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		var err error
		for {
			server.db, err = storage.Open(ctx, dsn)
			if err == nil || ctx.Err() != nil {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			return fmt.Errorf("connect to PostgreSQL: %w", err)
		}
		defer server.db.Close()
		if err := server.bootstrapAdmin(ctx); err != nil {
			return fmt.Errorf("bootstrap admin: %w", err)
		}
	} else {
		playerProfile, err := profile.LoadProfile()
		if err != nil {
			return fmt.Errorf("load local profile: %w", err)
		}
		if err := (&app{}).initializeProgressionProfile(playerProfile); err != nil {
			return err
		}
		server.playerProfile = playerProfile
	}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("start local game server: %w", err)
	}
	mux := server.routes()
	address := "http://" + listener.Addr().String()
	fmt.Printf("Prisoners of War API running at %s\n", address)
	return (&http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}).Serve(listener)
}

func (s *webServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *webServer) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.db != nil && s.db.PingContext(ctx) != nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("database unavailable"))
		return
	}
	s.handleHealth(w, r)
}

func (s *webServer) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"service": "prisoners-of-war", "frontend": "Run the Vite frontend or deploy the Helm chart"})
}

func (s *webServer) handleSkinAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := "skins/" + chi.URLParam(r, "*")
	if chi.URLParam(r, "*") == "" {
		name = strings.TrimPrefix(r.URL.Path, "/")
	}
	if !strings.HasPrefix(name, "skins/") || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	asset, err := webFiles.ReadFile(strings.TrimPrefix(name, "skins/"))
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "skin asset unavailable", http.StatusInternalServerError)
		return
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(asset); err != nil {
		fmt.Fprintf(os.Stderr, "write skin asset response: %v\n", err)
	}
}

func (s *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.viewForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *webServer) handleNewGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request actionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	mode := request.Mode
	if mode == "" {
		mode = s.defaultMode
	}
	if !validWebMode(mode) {
		writeError(w, http.StatusBadRequest, errors.New("mode must be hvh, easy, normal, or hard"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	playerHeroID := s.requestedHeroID(request.PlayerHeroID)
	playerHero, ok := heroes.ByID(playerHeroID)
	if !ok || !s.heroUnlocked(playerHeroID) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("player hero %q is not unlocked", playerHeroID))
		return
	}
	opponentHeroID := request.OpponentHeroID
	if mode == "hvh" {
		if opponentHeroID == "" {
			opponentHeroID = playerHeroID
		}
		if _, ok := heroes.ByID(opponentHeroID); !ok || !s.heroUnlocked(opponentHeroID) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("opponent hero %q is not unlocked", opponentHeroID))
			return
		}
	} else {
		unlockedIDs := s.unlockedHeroIDs()
		opponentHero, chooseErr := heroes.ChooseForOpponent(unlockedIDs, playerHeroID, s.rng)
		if chooseErr != nil {
			writeError(w, http.StatusInternalServerError, chooseErr)
			return
		}
		opponentHeroID = opponentHero.ID
	}
	opponentHero, ok := heroes.ByID(opponentHeroID)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown opponent hero %q", opponentHeroID))
		return
	}
	gameModifiers, err := heroes.GameModifiersFor(playerHero, opponentHero, s.rng)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	state, err := game.NewGameWithModifiers(s.rng, gameModifiers)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	whitePlayer, err := state.PlayerAtSeat(game.SeatX)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.session = &webSession{
		state:          state,
		mode:           mode,
		whitePlayer:    whitePlayer,
		phase:          "commit",
		actor:          0,
		winner:         -1,
		playerHeroID:   playerHeroID,
		opponentHeroID: opponentHeroID,
		message:        "Choose three cards from your hand to commit for Round 1.",
	}
	if s.playerProfile != nil {
		s.playerProfile.LastHeroPicked = playerHeroID
		s.playerProfile.OpponentLastHeroID = opponentHeroID
		s.playerProfile.LastOpponentHeroID = opponentHeroID
		s.playerProfile.LastOpponentHeroIDs = append([]string{opponentHeroID}, s.playerProfile.LastOpponentHeroIDs...)
		if len(s.playerProfile.LastOpponentHeroIDs) > 2 {
			s.playerProfile.LastOpponentHeroIDs = s.playerProfile.LastOpponentHeroIDs[:2]
		}
		if err := s.playerProfile.Save(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := s.advanceAI(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}

func (s *webServer) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request actionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		writeError(w, http.StatusConflict, errors.New("start a game first"))
		return
	}
	if err := s.applyAction(request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.advanceAI(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}

func (s *webServer) requestedHeroID(id string) string {
	if id == "" && s.playerProfile != nil {
		id = s.playerProfile.DefaultHeroID
	}
	if id == "" {
		id = profile.DefaultHeroID
	}
	return id
}

func (s *webServer) profileForRequest(r *http.Request) (*profile.Profile, bool, error) {
	if r.URL.Query().Get("preview") != "dashboard" {
		if s.playerProfile == nil {
			return nil, false, errors.New("profile is unavailable")
		}
		return s.playerProfile, false, nil
	}
	if s.previewProfile == nil {
		if s.playerProfile == nil {
			return nil, true, errors.New("profile is unavailable")
		}
		data, err := json.Marshal(s.playerProfile)
		if err != nil {
			return nil, true, fmt.Errorf("copy profile for dashboard preview: %w", err)
		}
		var preview profile.Profile
		if err := json.Unmarshal(data, &preview); err != nil {
			return nil, true, fmt.Errorf("decode dashboard preview profile: %w", err)
		}
		preview.DrawsAvailable = 1
		preview.SkillPoints = 2
		s.previewProfile = &preview
	}
	return s.previewProfile, true, nil
}

func saveWebProfile(p *profile.Profile, preview bool) error {
	if preview {
		return nil
	}
	return p.Save()
}

func (s *webServer) handleCampDraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, preview, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, s.campDrawPage(p, preview, nil))
		return
	}
	var request actionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var boxResult *lootbox.BoxOpenResult
	switch request.Action {
	case "start-draw":
		session, err := lootbox.BeginCampDraw(p, s.rng)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.campDrawSession = session
		s.campDrawPreview = preview
		s.lastCampDraw = nil
	case "play-card":
		if s.campDrawSession == nil || s.campDrawPreview != preview {
			writeError(w, http.StatusConflict, errors.New("start a Camp Draw first"))
			return
		}
		if _, err := s.campDrawSession.Play(request.Index, s.rng); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if _, hasPrompt := s.campDrawSession.Prompt(); !hasPrompt {
			result, err := s.campDrawSession.Finish(p, s.rng, nil)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			s.lastCampDraw = &result
			s.lastCampDrawPreview = preview
			evaluateAchievementEvent(p, achievements.EventCampDrawOpened, "browser")
			if err := saveWebProfile(p, preview); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Errorf("save Camp Draw progress: %w", err))
				return
			}
			s.campDrawSession = nil
		}
	case "open-box":
		result, openErr := lootbox.OpenPendingBox(p, request.BoxType, s.rng)
		if openErr != nil {
			writeError(w, http.StatusBadRequest, openErr)
			return
		}
		boxResult = &result
		if err := saveWebProfile(p, preview); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save pending-box opening: %w", err))
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errors.New("unknown Camp Draw action"))
		return
	}
	page := s.campDrawPage(p, preview, boxResult)
	writeJSON(w, http.StatusOK, page)
}

func (s *webServer) campDrawPage(
	p *profile.Profile,
	preview bool,
	boxResult *lootbox.BoxOpenResult,
) webCampDrawPage {
	page := webCampDrawPage{
		DrawsAvailable: p.DrawsAvailable,
		PendingBoxes:   cloneBoxCounts(p.PendingBoxes),
		Dashboard:      dashboardFor(p),
		BoxResult:      boxResult,
	}
	if s.campDrawSession != nil && s.campDrawPreview == preview {
		page.Started = true
		page.StartingHand = s.campDrawSession.StartingHand()
		page.Exchanges = s.campDrawSession.Exchanges()
		if prompt, ok := s.campDrawSession.Prompt(); ok {
			page.Prompt = &prompt
		}
	}
	if s.lastCampDraw != nil && s.lastCampDrawPreview == preview {
		page.Result = s.lastCampDraw
	}
	return page
}

func cloneBoxCounts(boxes map[string]int) map[string]int {
	result := make(map[string]int, len(boxes))
	for boxType, count := range boxes {
		result[boxType] = count
	}
	return result
}

func (s *webServer) handleAchievements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, preview, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	page := buildAchievementsPage(p)
	markAchievementsSeen(p)
	if err := saveWebProfile(p, preview); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("save achievement views: %w", err))
		return
	}
	page.Dashboard = dashboardFor(p)
	writeJSON(w, http.StatusOK, page)
}

func buildAchievementsPage(p *profile.Profile) webAchievementsPage {
	seen := make(map[string]bool, len(p.SeenAchievementIDs))
	for _, id := range p.SeenAchievementIDs {
		seen[id] = true
	}
	ctx := achievements.Context{Profile: p, Now: time.Now(), UnlockedHeroCount: len(p.UnlockedHeroes)}
	page := webAchievementsPage{
		Items: make([]webAchievement, 0, len(achievements.All())),
		Sets:  make([]webAchievementSet, 0, len(achievements.AllSets())),
	}
	for _, definition := range achievements.All() {
		progress, _, tier, ok := achievements.ProgressFor(ctx, definition.ID)
		if !ok {
			continue
		}
		unlocked := achievements.IsUnlocked(p, definition)
		discovered := unlocked || len(definition.Tiers) > 0 && p.AchievementTiers[definition.ID] > 0
		item := webAchievement{
			ID: definition.ID, Name: definition.Name, Category: categoryName(definition.Category),
			Details: definition.Description, Hidden: definition.Hidden && !unlocked,
			Unlocked: unlocked, New: discovered && !seen[definition.ID], Progress: progress,
			Tiers: make([]webAchievementTier, 0, len(definition.Tiers)),
		}
		if item.Hidden {
			item.Name, item.Details = "???", "Hidden achievement"
			item.Progress = 0
		} else if len(definition.Tiers) == 0 {
			item.Goal = 1
		} else {
			nextTier := tier
			if nextTier >= len(definition.Tiers) {
				nextTier = len(definition.Tiers) - 1
			}
			if nextTier >= 0 {
				item.Goal = definition.Tiers[nextTier].Goal
			}
			for _, achievementTier := range definition.Tiers {
				item.Tiers = append(item.Tiers, webAchievementTier{
					Name: achievementTier.Name, Goal: achievementTier.Goal,
					Marks: achievementTier.RewardMarks, Crowns: achievementTier.RewardCrowns,
				})
			}
		}
		page.Items = append(page.Items, item)
	}
	for _, set := range achievements.AllSets() {
		completed := 0
		for _, id := range set.Members {
			definition, ok := achievements.Find(id)
			if ok && achievements.IsUnlocked(p, definition) {
				completed++
			}
		}
		page.Sets = append(page.Sets, webAchievementSet{
			ID: set.ID, Name: set.Name, Complete: p.CompletedSets[set.ID],
			Progress: completed, Total: len(set.Members),
		})
	}
	return page
}

func (s *webServer) handleShop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, preview, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, s.shopPage(p))
		return
	}
	var request actionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	message, err := s.applyShopAction(p, request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := saveWebProfile(p, preview); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("save shop purchase: %w", err))
		return
	}
	page := s.shopPage(p)
	page.Message = message
	writeJSON(w, http.StatusOK, page)
}

func (s *webServer) shopPage(p *profile.Profile) webShopPage {
	page := webShopPage{
		Chromas:   make([]webShopChroma, 0, len(catalog.AllChromas())),
		Bundles:   make([]webShopBundle, 0, len(catalog.AllBundles())),
		Heroes:    make([]webShopHero, 0, len(heroes.All())),
		Dashboard: dashboardFor(p), HeroBoxesAvailable: p.HeroBoxesAvailable,
		HeroBoxCost: heroes.HeroBoxCost(p), DirectHeroCost: heroes.DirectHeroCrownsCost,
		ChromaPickTokens: p.ChromaPickTokens,
		BoxOffers: []webShopBoxOffer{
			{Type: "wooden", Name: "Wooden Box", Currency: "marks", Cost: shopBoxMarksCost, Available: p.PendingBoxes["wooden"]},
			{Type: "silver", Name: "Silver Box", Currency: "marks", Cost: shopBoxMarksCost, Available: p.PendingBoxes["silver"]},
			{Type: "hero", Name: "Hero Box", Currency: "marks", Cost: shopBoxMarksCost, Available: p.HeroBoxesAvailable},
			{Type: "gold", Name: "Golden Box", Currency: "crowns", Cost: shopGoldenBoxCrownsCost, Available: p.PendingBoxes["gold"]},
		},
	}
	for _, chroma := range catalog.AllChromas() {
		page.Chromas = append(page.Chromas, webShopChroma{Chroma: chroma, Owned: p.OwnedChromas[chroma.ID]})
	}
	for _, bundle := range catalog.AllBundles() {
		page.Bundles = append(page.Bundles, webShopBundle{Bundle: bundle, Owned: p.PurchasedBundles[bundle.ID]})
	}
	for _, hero := range heroes.All() {
		owned := containsHeroID(p.UnlockedHeroes, hero.ID)
		page.Heroes = append(page.Heroes, webShopHero{
			Hero:  webHeroOption{ID: hero.ID, Name: hero.Name, Unlocked: owned},
			Owned: owned, CanBuy: !owned && hero.ID != "recruit" && p.Crowns >= heroes.DirectHeroCrownsCost,
			Plays: p.HeroPlayCounts[hero.ID], Wins: p.HeroWinCounts[hero.ID],
		})
	}
	return page
}

func (s *webServer) applyShopAction(p *profile.Profile, request actionRequest) (string, error) {
	switch request.Action {
	case "buy-chroma":
		chroma, ok := catalog.FindChroma(request.ID)
		if !ok {
			return "", fmt.Errorf("unknown chroma %q", request.ID)
		}
		if err := catalog.BuyChroma(p, request.ID); err != nil {
			return "", err
		}
		evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
		return "Purchased " + chroma.Name + ".", nil
	case "buy-bundle":
		if err := catalog.BuyBundle(p, request.ID); err != nil {
			return "", err
		}
		evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
		return "Purchased bundle " + request.ID + ".", nil
	case "buy-box":
		return buyShopBox(p, request.BoxType)
	case "open-hero-box":
		var result heroes.HeroBoxResult
		var err error
		if request.BoxType == "free" {
			result, err = heroes.OpenHeroBox(p, s.rng)
		} else if request.BoxType == "marks" {
			result, err = heroes.OpenHeroBoxWithMarks(p, s.rng)
		} else {
			return "", errors.New("choose free or Marks Hero Box")
		}
		if err != nil {
			return "", err
		}
		if result.Duplicate {
			return fmt.Sprintf("Duplicate %s-tier hero: +%d Crowns.", result.DuplicateTier, result.Crowns), nil
		}
		hero, ok := heroes.ByID(result.HeroID)
		if !ok {
			return "", fmt.Errorf("Hero Box returned unknown hero %q", result.HeroID)
		}
		evaluateAchievementEvent(p, achievements.EventHeroAcquired, "box")
		return "Hero Box unlocked " + hero.Name + ".", nil
	case "buy-hero":
		if err := heroes.BuyHeroWithCrowns(p, request.ID); err != nil {
			return "", err
		}
		evaluateAchievementEvent(p, achievements.EventHeroAcquired, "purchase")
		return "Purchased " + heroName(request.ID) + ".", nil
	case "redeem-pick":
		if err := catalog.RedeemChromaPick(p, request.ID); err != nil {
			return "", err
		}
		return "Claimed " + request.ID + ".", nil
	case "assign-chroma":
		if err := catalog.AssignChroma(p, request.Suit, request.ID); err != nil {
			return "", err
		}
		return "Assigned " + request.ID + " to suit " + request.Suit + ".", nil
	default:
		return "", errors.New("unknown shop action")
	}
}

func buyShopBox(p *profile.Profile, boxType string) (string, error) {
	switch boxType {
	case "wooden", "silver":
		if p.Marks < shopBoxMarksCost {
			return "", fmt.Errorf("need %d Marks; you have %d", shopBoxMarksCost, p.Marks)
		}
		if err := p.GrantBox(boxType, 1); err != nil {
			return "", err
		}
		p.Marks -= shopBoxMarksCost
	case "gold":
		if p.Crowns < shopGoldenBoxCrownsCost {
			return "", fmt.Errorf("need %d Crowns; you have %d", shopGoldenBoxCrownsCost, p.Crowns)
		}
		if err := p.GrantBox(boxType, 1); err != nil {
			return "", err
		}
		p.Crowns -= shopGoldenBoxCrownsCost
	case "hero":
		if p.Marks < shopBoxMarksCost {
			return "", fmt.Errorf("need %d Marks; you have %d", shopBoxMarksCost, p.Marks)
		}
		p.Marks -= shopBoxMarksCost
		p.HeroBoxesAvailable++
	default:
		return "", fmt.Errorf("unknown box type %q", boxType)
	}
	evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
	return "Purchased " + boxType + " box.", nil
}

func (s *webServer) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, preview, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	message := ""
	if r.Method == http.MethodPost {
		var request actionRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if request.Action != "unlock" {
			writeError(w, http.StatusBadRequest, errors.New("unknown skill-tree action"))
			return
		}
		switch request.Payment {
		case "crowns":
			err = skills.UnlockNode(p, request.ID)
		case "skill-point":
			err = skills.UnlockNodeWithSkillPoint(p, request.ID)
		default:
			err = errors.New("choose Crowns or a Skill Tree Point")
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := saveWebProfile(p, preview); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save skill-tree unlock: %w", err))
			return
		}
		message = "Unlocked " + request.ID + "."
	}
	page := s.skillsPage(p)
	page.Message = message
	writeJSON(w, http.StatusOK, page)
}

func (s *webServer) skillsPage(p *profile.Profile) webSkillsPage {
	page := webSkillsPage{Nodes: make([]webSkillNode, 0, len(skills.AllNodes())), Dashboard: dashboardFor(p)}
	for _, node := range skills.AllNodes() {
		canCrowns, crownReason := skills.CanUnlock(p, node.ID)
		canPoint, pointReason := skills.CanUnlockWithSkillPoint(p, node.ID)
		page.Nodes = append(page.Nodes, webSkillNode{
			Node: node, Unlocked: p.SkillNodes[node.ID], CanUnlockCrowns: canCrowns,
			CanUnlockSkillPoint: canPoint, CrownsReason: crownReason, SkillPointReason: pointReason,
		})
	}
	return page
}

func (s *webServer) handleHeroes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, preview, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	message := ""
	if r.Method == http.MethodPost {
		var request actionRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if request.Action != "select" {
			writeError(w, http.StatusBadRequest, errors.New("unknown hero action"))
			return
		}
		if _, ok := heroes.ByID(request.ID); !ok || !containsHeroID(p.UnlockedHeroes, request.ID) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("hero %q is not unlocked", request.ID))
			return
		}
		p.DefaultHeroID = request.ID
		if err := saveWebProfile(p, preview); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save default hero: %w", err))
			return
		}
		message = "Default hero set to " + heroName(request.ID) + "."
	}
	page := s.heroesPage(p)
	page.Message = message
	writeJSON(w, http.StatusOK, page)
}

func (s *webServer) heroesPage(p *profile.Profile) webHeroesPage {
	options := make([]webHeroOption, 0, len(heroes.All()))
	for _, hero := range heroes.All() {
		options = append(options, webHeroOption{
			ID: hero.ID, Name: hero.Name, Unlocked: containsHeroID(p.UnlockedHeroes, hero.ID),
		})
	}
	return webHeroesPage{Heroes: options, DefaultID: p.DefaultHeroID, Dashboard: dashboardFor(p)}
}

func (s *webServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, webCommanderPage{
		Dashboard: dashboardFor(p), PlayerID: p.PlayerID, CreatedAt: p.CreatedAt,
		TotalGames: p.TotalGames, TotalWins: p.TotalWins, TotalLosses: p.TotalLosses,
		TotalDraws: p.TotalDraws, CurrentStreak: p.CurrentStreak, BestStreak: p.BestStreak,
		TotalClashes: p.TotalClashes, MatchTies: p.TotalMatchTies,
		WinsAsSeatX: p.WinsAsSeatX, WinsAsSeatY: p.WinsAsSeatY,
	})
}

func (s *webServer) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	definitions := campaigns.All()
	page := webCampaignPage{
		Dashboard: dashboardFor(p),
		Campaigns: make([]webCampaignSummary, 0, len(definitions)),
	}
	for _, campaign := range definitions {
		progress := p.Campaigns[campaign.ID]
		completedMissions := make(map[int]bool)
		if progress != nil {
			for _, missionIndex := range progress.CompletedMissionIDs {
				completedMissions[missionIndex] = true
			}
		}
		summary := webCampaignSummary{
			ID: campaign.ID, Name: campaign.Name, Description: campaign.Description,
			Requires: campaign.Requires, Unlocked: campaigns.IsUnlocked(p, campaign.ID),
			Missions:      make([]webCampaignMission, 0, len(campaign.Missions)),
			TotalMissions: len(campaign.Missions),
		}
		if progress != nil {
			summary.Started = progress.Started
			summary.Completed = progress.Completed
		}
		for _, mission := range campaign.Missions {
			completed := completedMissions[mission.Index]
			if completed {
				summary.MissionsCompleted++
			}
			summary.Missions = append(summary.Missions, webCampaignMission{
				Index: mission.Index, Name: mission.Name, Description: mission.Hint,
				RewardMarks: mission.RewardMarks, Completed: completed,
			})
		}
		page.Campaigns = append(page.Campaigns, summary)
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *webServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.profileForRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	page := webHistoryPage{
		Dashboard: dashboardFor(p), TotalGames: p.TotalGames, TotalWins: p.TotalWins,
		TotalLosses: p.TotalLosses, TotalDraws: p.TotalDraws,
		Matches: make([]webHistoryMatch, 0, len(p.MatchHistory)),
	}
	for _, record := range p.MatchHistory {
		page.Matches = append(page.Matches, webHistoryMatch{
			Record: record, ModeName: webHistoryModeName(record.Mode),
		})
	}
	writeJSON(w, http.StatusOK, page)
}

func webHistoryModeName(modeID int) string {
	switch modeID {
	case economy.ModeIDCPUEasy:
		return "AI · Easy"
	case economy.ModeIDCPUNormal:
		return "AI · Normal"
	case economy.ModeIDCPUHard:
		return "AI · Hard"
	case economy.ModeIDRanked:
		return "Ranked"
	case economy.ModeIDFriendCasual:
		return "Friend · Casual"
	case economy.ModeIDFriendRanked:
		return "Friend · Ranked"
	default:
		return "Unknown mode"
	}
}

func (s *webServer) applyAction(request actionRequest) error {
	session := s.session
	switch request.Action {
	case "commit":
		return s.commitAction(request.Cards)
	case "ready":
		return s.readyAction()
	case "attack":
		if session.phase != "attack" || isAI(session.mode, session.attacker) {
			return errors.New("it is not your attack turn")
		}
		if err := s.selectAttack(request.Index); err != nil {
			return err
		}
	case "defend":
		if session.phase != "defend" || isAI(session.mode, session.actor) {
			return errors.New("it is not your defense turn")
		}
		if err := s.resolveBattle(request.Index); err != nil {
			return err
		}
	case "continue":
		if session.phase != "result" {
			return errors.New("there is no battle result to continue from")
		}
		if err := s.continueAfterBattle(); err != nil {
			return err
		}
	case "seat":
		if session.phase != "seat" {
			return errors.New("Round 3 seat selection is no longer active")
		}
		if isAI(session.mode, session.actor) {
			return errors.New("the AI chooses the Round 3 seat")
		}
		seat := game.SeatX
		if strings.EqualFold(request.Seat, "Y") {
			seat = game.SeatY
		} else if !strings.EqualFold(request.Seat, "X") {
			return errors.New("choose seat X or Y")
		}
		s.assignSeat(session.actor, seat)
		s.beginRoundThree()
	default:
		return errors.New("unknown action")
	}
	return nil
}

func (s *webServer) commitAction(cards []int) error {
	session := s.session
	if session.phase != "commit" || isAI(session.mode, session.actor) {
		return errors.New("it is not your commit turn")
	}
	if err := game.Commit(&session.state, session.actor, cards); err != nil {
		return err
	}
	if session.mode == "hvh" {
		if session.actor == 0 {
			session.actor = 1
			session.handoffTarget = 1
			session.handoffPhase = "commit"
			session.phase = "handoff"
			session.message = "Pass the screen to Player 2, then continue."
			return nil
		}
		s.beginBattles()
		return nil
	}
	session.actor = 1
	session.message = "Player 2 is committing."
	return nil
}

func (s *webServer) readyAction() error {
	session := s.session
	if session.phase != "handoff" {
		return errors.New("there is no handoff to acknowledge")
	}
	session.actor = session.handoffTarget
	session.phase = session.handoffPhase
	session.handoffPhase = ""
	if session.phase == "commit" {
		session.message = fmt.Sprintf("Player %d, choose three cards to commit.", session.actor+1)
	} else if session.phase == "attack" {
		session.message = fmt.Sprintf("Player %d attacks. Choose a card from their pile.", session.actor+1)
	} else {
		session.message = "Choose a response to the revealed attack."
	}
	return nil
}

func (s *webServer) advanceAI() error {
	session := s.session
	for session != nil && isAI(session.mode, session.actor) {
		switch session.phase {
		case "commit":
			indices, err := ai.ChooseCommit(session.state, session.actor, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			if err := game.Commit(&session.state, session.actor, indices); err != nil {
				return err
			}
			session.message = "The opponent has committed."
			if session.actor == 0 {
				session.actor = 1
			} else {
				s.beginBattles()
			}
		case "attack":
			index, err := ai.ChoosePoolIndex(session.state, session.attacker, true, nil, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			if err := s.selectAttack(index); err != nil {
				return err
			}
		case "defend":
			index, err := ai.ChoosePoolIndex(session.state, session.actor, false, &session.attackCard, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			if err := s.resolveBattle(index); err != nil {
				return err
			}
		case "seat":
			seat, err := ai.ChooseSeat(session.state, session.actor, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			s.assignSeat(session.actor, seat)
			s.beginRoundThree()
		default:
			return nil
		}
	}
	return nil
}

func (s *webServer) beginBattles() {
	session := s.session
	session.exchange = 0
	s.startBattle()
	s.handoffForAttacker()
}

func (s *webServer) startBattle() {
	session := s.session
	if session.exchange >= 3 {
		return
	}
	seat := game.SeatX
	if session.exchange == 1 {
		seat = game.SeatY
	}
	attacker, err := session.state.PlayerAtSeat(seat)
	if err != nil {
		session.message = err.Error()
		return
	}
	session.attacker = attacker
	session.actor = attacker
	attackerPlayer, _ := session.state.Player(attacker)
	defenderPlayer, _ := session.state.Player(1 - attacker)
	if len(attackerPlayer.Pool) == 0 || len(defenderPlayer.Pool) == 0 {
		session.message = "Battle skipped because a player has no remaining pile card."
		session.hasLastBattle = false
		session.phase = "result"
		return
	}
	if session.exchange == 2 {
		session.attackIndex = 0
		session.attackCard = attackerPlayer.Pool[0]
		session.phase = "defend"
		session.actor = 1 - attacker
		session.message = "The last card in each pile is played automatically."
		if len(defenderPlayer.Pool) == 1 {
			if err := s.resolveBattle(0); err != nil {
				session.message = err.Error()
			}
		}
		return
	}
	session.phase = "attack"
	session.message = fmt.Sprintf("Player %d attacks. Choose a card from their pile.", attacker+1)
}

func (s *webServer) handoffForAttacker() {
	session := s.session
	if session.mode == "hvh" && session.phase == "attack" {
		session.handoffTarget = session.attacker
		session.handoffPhase = "attack"
		session.phase = "handoff"
		session.message = fmt.Sprintf("Pass the screen to Player %d to attack.", session.attacker+1)
	}
}

func (s *webServer) selectAttack(index int) error {
	session := s.session
	attacker, _ := session.state.Player(session.attacker)
	if index < 0 || index >= len(attacker.Pool) {
		return game.ErrInvalidPoolCard
	}
	session.attackIndex = index
	session.attackCard = attacker.Pool[index]
	session.actor = 1 - session.attacker
	session.phase = "defend"
	session.message = fmt.Sprintf("Player %d revealed a card. The defender chooses a response.", session.attacker+1)
	if session.mode == "hvh" {
		session.handoffTarget = session.actor
		session.handoffPhase = "defend"
		session.phase = "handoff"
		session.message = fmt.Sprintf("Pass the screen to Player %d to defend. The attack is revealed.", session.actor+1)
		return nil
	}
	if isAI(session.mode, session.actor) {
		defendIndex, err := ai.ChoosePoolIndex(session.state, session.actor, false, &session.attackCard, webDifficulty(session.mode), s.rng)
		if err != nil {
			return err
		}
		return s.resolveBattle(defendIndex)
	}
	return nil
}

func (s *webServer) resolveBattle(defendIndex int) error {
	session := s.session
	defender := 1 - session.attacker
	defenderPlayer, _ := session.state.Player(defender)
	if defendIndex < 0 || defendIndex >= len(defenderPlayer.Pool) {
		return game.ErrInvalidPoolCard
	}
	defenseCard := defenderPlayer.Pool[defendIndex]
	attackerIndex := session.attacker
	attackCard := session.attackCard
	outcome, err := game.PlayExchange(&session.state, attackerIndex, session.attackIndex, defender, defendIndex)
	if err != nil {
		return err
	}
	session.lastAttack, session.lastDefense, session.lastOutcome = attackCard, defenseCard, outcome
	session.hasLastBattle = true
	slotIndex := (session.state.Round-1)*3 + session.exchange
	cell := jailCell{}
	switch outcome.Winner {
	case 0:
		cell.Card = cardPointer(defenseCard)
		cell.CapturedBy = s.colorName(attackerIndex)
	case 1:
		cell.Card = cardPointer(attackCard)
		cell.CapturedBy = s.colorName(defender)
	default:
		cell.Clash = true
	}
	if slotIndex >= 0 && slotIndex < len(session.slots) {
		session.slots[slotIndex] = cell
	}
	session.phase = "result"
	if outcome.Winner == -1 {
		session.message = "Clash! Both cards are discarded."
	} else {
		winner := attackerIndex
		if outcome.Winner == 1 {
			winner = defender
		}
		session.message = fmt.Sprintf("%s wins this battle and captures the opposing card.", s.colorName(winner))
	}
	return nil
}

func (s *webServer) continueAfterBattle() error {
	session := s.session
	if session.exchange < 2 {
		session.exchange++
		session.hasLastBattle = false
		s.startBattle()
		s.handoffForAttacker()
		return nil
	}
	game.DiscardPools(&session.state)
	if session.state.Round == 3 {
		s.finishGame()
		return nil
	}
	swap := session.state.Round == 1
	if err := game.EndRound(&session.state, swap, true); err != nil {
		return err
	}
	session.state.Round++
	if session.state.Round == 3 {
		s.chooseRoundThreeSeat()
		return nil
	}
	session.actor = 0
	session.phase = "commit"
	session.message = "New round: choose three cards from your hand to commit."
	if session.mode == "hvh" {
		session.handoffTarget = 0
		session.handoffPhase = "commit"
		session.phase = "handoff"
		session.message = "Pass the screen to Player 1 for the next round."
	}
	return nil
}

func (s *webServer) chooseRoundThreeSeat() {
	session := s.session
	score1 := game.ScorePlayer(session.state.P1, session.state.Bounty.Suit, session.state.Modifiers).Total
	score2 := game.ScorePlayer(session.state.P2, session.state.Bounty.Suit, session.state.Modifiers).Total
	chooser := 0
	if score2 < score1 || (score1 == score2 && s.rng.Intn(2) == 1) {
		chooser = 1
	}
	session.actor = chooser
	session.phase = "seat"
	session.message = fmt.Sprintf("Player %d chooses whether to take seat X or Y for Round 3.", chooser+1)
}

func (s *webServer) assignSeat(playerIndex int, seat game.Seat) {
	player, _ := s.session.state.Player(playerIndex)
	opponent, _ := s.session.state.Player(1 - playerIndex)
	player.Seat, opponent.Seat = seat, otherSeat(seat)
}

func (s *webServer) beginRoundThree() {
	s.session.actor = 0
	s.session.phase = "commit"
	s.session.message = "Round 3: choose three cards from your hand to commit."
}

func (s *webServer) finishGame() {
	session := s.session
	p1 := game.ScorePlayer(session.state.P1, session.state.Bounty.Suit, session.state.Modifiers)
	p2 := game.ScorePlayer(session.state.P2, session.state.Bounty.Suit, session.state.Modifiers)
	session.winner, session.tiebreak = finalWinner(p1, p2, s.rng)
	session.phase = "gameover"
	session.message = "The three rounds are complete."
	payout := awardWebMatchMarks(s.playerProfile, p1.Total, session.winner == 0)
	session.payout = &payout
	if s.playerProfile != nil {
		p := s.playerProfile
		won := session.winner == 0
		p.TotalGames++
		p.TotalClashes += session.state.Clashes
		if won {
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
		mode := map[string]int{"hvh": economy.ModeIDFriendCasual, "easy": economy.ModeIDCPUEasy, "normal": economy.ModeIDCPUNormal, "hard": economy.ModeIDCPUHard}[session.mode]
		suits := map[string]int{}
		for _, card := range session.state.P1.Jail {
			suits[achievements.SuitRoleID(card.Suit)]++
		}
		p.RecordMatch(profile.MatchRecord{At: time.Now().UTC(), Mode: mode, PlayerSeat: session.state.P1.Seat.String(), PlayerScore: p1.Total, OppScore: p2.Total, Won: won, BountySuit: achievements.SuitRoleID(session.state.Bounty.Suit), PlayerPrisoners: len(session.state.P1.Jail), OppPrisoners: len(session.state.P2.Jail), PlayerPrisonerSuits: suits, Clashes: session.state.Clashes, MarksEarned: payout.TotalMarks})
		if s.playerProfile.HeroPlayCounts == nil {
			s.playerProfile.HeroPlayCounts = make(map[string]int)
		}
		if s.playerProfile.HeroWinCounts == nil {
			s.playerProfile.HeroWinCounts = make(map[string]int)
		}
		s.playerProfile.HeroPlayCounts[session.playerHeroID]++
		if session.winner == 0 {
			s.playerProfile.HeroWinCounts[session.playerHeroID]++
		}
		if err := s.playerProfile.Save(); err != nil {
			session.profileError = "Profile save failed: " + err.Error()
		}
	}
}

func awardWebMatchMarks(p *profile.Profile, score int, won bool) webMatchPayout {
	if score < 0 {
		score = 0
	}
	payout := webMatchPayout{
		PlayerScore: score, PlayerWon: won,
		BaseMarks: int64(50 + score), WinMultiplier: 1,
	}
	if won {
		payout.WinMultiplier = 3
	}
	if p != nil {
		payout.SkillBonusPct = skills.AggregateBonuses(p).MarksBasePercent
	}
	multiplier := float64(100+payout.SkillBonusPct) / 100
	payout.TotalMarks = int64(math.Floor(float64(payout.BaseMarks) *
		float64(payout.WinMultiplier) * multiplier))
	if p != nil {
		p.Marks += payout.TotalMarks
	}
	return payout
}

func (s *webServer) view() webView {
	if s.session == nil {
		defaultHeroID := profile.DefaultHeroID
		if s.playerProfile != nil && s.playerProfile.DefaultHeroID != "" {
			defaultHeroID = s.playerProfile.DefaultHeroID
		}
		return webView{
			Started: false, Dashboard: s.dashboard(), Mode: s.defaultMode, Winner: -1, DefaultHeroID: defaultHeroID,
			OpponentHint: s.opponentLastHeroName(), Heroes: s.heroOptions(),
		}
	}
	session := s.session
	result := webView{
		Started:        true,
		Dashboard:      s.dashboard(),
		Mode:           session.mode,
		Phase:          session.phase,
		Round:          session.state.Round,
		Exchange:       session.exchange + 1,
		Actor:          session.actor,
		Attacker:       session.attacker,
		Bounty:         session.state.Bounty,
		JailSlots:      make([]jailCell, len(session.slots)),
		Message:        session.message,
		CanContinue:    session.phase == "result",
		Winner:         session.winner,
		Payout:         session.payout,
		Tiebreak:       session.tiebreak,
		Modifiers:      session.state.Modifiers,
		PlayerHero:     heroName(session.playerHeroID),
		OpponentHero:   heroName(session.opponentHeroID),
		PlayerHeroID:   session.playerHeroID,
		OpponentHeroID: session.opponentHeroID,
		Heroes:         s.heroOptions(),
		DefaultHeroID:  session.playerHeroID,
	}
	copy(result.JailSlots, session.slots[:])
	if session.phase == "defend" || session.phase == "result" && session.hasLastBattle {
		result.BattleAttack = cardPointer(session.attackCard)
	}
	if session.phase == "handoff" && session.handoffPhase == "defend" {
		result.BattleAttack = cardPointer(session.attackCard)
	}
	if session.phase == "result" && session.hasLastBattle {
		result.BattleDefend = cardPointer(session.lastDefense)
		result.BattleOutcome = &webBattleOutcome{
			AttackerValue: session.lastOutcome.AttackerValue,
			DefenderValue: session.lastOutcome.DefenderValue,
			Winner:        session.lastOutcome.Winner,
		}
	}
	scores := [2]game.ScoreBreakdown{
		game.ScorePlayer(session.state.P1, session.state.Bounty.Suit, session.state.Modifiers),
		game.ScorePlayer(session.state.P2, session.state.Bounty.Suit, session.state.Modifiers),
	}
	result.Players = make([]webPlayer, 2)
	for i, player := range session.state.Players() {
		result.Players[i] = s.playerView(i, player, scores[i].Total)
	}
	if session.phase == "gameover" && session.mode != "hvh" {
		if session.winner == 0 {
			result.Message = "You Won!"
		} else {
			result.Message = "You Lost."
		}
	}
	if session.phase == "seat" && session.actor == 0 {
		result.Message = "You choose a seat for Round 3."
	}
	if session.phase == "handoff" {
		result.Message = fmt.Sprintf("Pass the screen to Player %d.", session.handoffTarget+1)
	}
	if session.profileError != "" {
		result.Message += " " + session.profileError
	}
	result.ScoreLine = fmt.Sprintf("%s = %d  |  %s = %d",
		s.scoreLabel(session.whitePlayer), scores[session.whitePlayer].Total,
		s.scoreLabel(1-session.whitePlayer), scores[1-session.whitePlayer].Total)
	return result
}

func (s *webServer) playerView(index int, player game.Player, score int) webPlayer {
	session := s.session
	private := session.mode == "hvh" && index == session.actor && session.phase != "handoff" || session.mode != "hvh" && index == 0
	view := webPlayer{Color: s.colorName(index), You: session.mode != "hvh" && index == 0, Seat: player.Seat.String(), Score: score, Hand: []webCard{}, Pool: []webCard{}, HiddenHand: len(player.Hand), HiddenPool: len(player.Pool)}
	if (session.phase == "defend" || session.phase == "handoff" && session.handoffPhase == "defend") && index == session.attacker {
		view.HiddenPool--
	}
	if private {
		s.fillPrivateCards(&view, index, player)
	}
	return view
}

func (s *webServer) canSelectPool(index int) bool {
	return (s.session.phase == "attack" && index == s.session.attacker) || (s.session.phase == "defend" && index != s.session.attacker)
}

func (s *webServer) fillPrivateCards(view *webPlayer, index int, player game.Player) {
	session := s.session
	view.Hand = make([]webCard, len(player.Hand))
	for cardIndex, card := range player.Hand {
		view.Hand[cardIndex] = webCard{Card: card, Index: cardIndex, Selectable: session.phase == "commit"}
	}
	view.HiddenHand = 0
	view.Pool = make([]webCard, 0, len(player.Pool))
	for cardIndex, card := range player.Pool {
		if session.phase == "defend" && index == session.attacker && cardIndex == session.attackIndex {
			continue
		}
		view.Pool = append(view.Pool, webCard{Card: card, Index: cardIndex, Selectable: s.canSelectPool(index)})
	}
	view.HiddenPool = len(player.Pool) - len(view.Pool)
	view.CanSelectHand = session.phase == "commit"
	view.CanSelectPool = s.canSelectPool(index)
}

func (s *webServer) viewForRequest(r *http.Request) (webView, error) {
	view := s.view()
	if r.URL.Query().Get("preview") != "dashboard" && s.playerProfile == nil {
		return view, nil
	}
	p, _, err := s.profileForRequest(r)
	if err != nil {
		return webView{}, err
	}
	view.Dashboard = dashboardFor(p)
	view.DefaultHeroID = p.DefaultHeroID
	if s.session == nil {
		view.Heroes = s.heroOptionsFor(p)
	}
	return view, nil
}

func (s *webServer) dashboard() *webDashboard {
	p := s.playerProfile
	if p == nil {
		return nil
	}
	return dashboardFor(p)
}

func dashboardFor(p *profile.Profile) *webDashboard {
	if p == nil {
		return nil
	}
	now := time.Now()
	currentEnergy := energy.Current(p, now)
	adsLeft := energy.DailyAdLimit - p.AdsWatchedToday
	if adsLeft < 0 {
		adsLeft = 0
	}
	nextSeconds := 0
	if currentEnergy < p.MaxEnergy {
		if remaining := energy.TimeUntilNext(p, now); remaining > 0 {
			nextSeconds = int((remaining + time.Second - 1) / time.Second)
		}
	}
	commander := p.PlayerID
	if len(commander) > 8 {
		commander = commander[:8]
	}
	return &webDashboard{
		Commander:          commander,
		Title:              heroName(p.DefaultHeroID),
		Marks:              p.Marks,
		Crowns:             p.Crowns,
		Energy:             currentEnergy,
		MaxEnergy:          p.MaxEnergy,
		EnergyNextSeconds:  nextSeconds,
		AdsLeft:            adsLeft,
		DailyFeaturedItem:  dailyFeaturedShopItem(p, now),
		LoginStreakCurrent: p.LoginStreakCurrent,
		LoginStreakBest:    p.LoginStreakBest,
		WinsSinceCampDraw:  p.WinsSinceCampDraw,
		WinsPerCampDraw:    lootbox.RequiredWins(p),
		DrawsAvailable:     p.DrawsAvailable,
		PendingBoxes:       p.TotalPendingBoxes(),
		SkillPoints:        p.SkillPoints,
		UnseenAchievements: len(unseenAchievementIDs(p)),
	}
}

func dailyFeaturedShopItem(p *profile.Profile, now time.Time) *webDailyFeaturedItem {
	items := make([]catalog.Chroma, 0, len(catalog.AllChromas()))
	for _, chroma := range catalog.AllChromas() {
		if chroma.Currency == string(catalog.Marks) || chroma.Currency == string(catalog.Crowns) {
			items = append(items, chroma)
		}
	}
	if len(items) == 0 {
		return nil
	}
	localDate := now.In(time.Local)
	dayIndex := int64(localDate.Year())*366 + int64(localDate.YearDay())
	item := items[dayIndex%int64(len(items))]
	return &webDailyFeaturedItem{
		ID: item.ID, Name: item.Name, Color: item.Color, Rarity: item.Rarity,
		Currency: item.Currency, Cost: item.Cost, Date: localDate.Format("2006-01-02"),
		Owned: p != nil && p.OwnedChromas[item.ID],
	}
}

func (s *webServer) colorName(playerIndex int) string {
	if playerIndex == s.session.whitePlayer {
		return "White"
	}
	return "Black"
}

func (s *webServer) scoreLabel(playerIndex int) string {
	if s.session.mode != "hvh" && playerIndex == 0 {
		return "You"
	}
	return s.colorName(playerIndex)
}

func (s *webServer) unlockedHeroIDs() []string {
	if s.playerProfile != nil && len(s.playerProfile.UnlockedHeroes) > 0 {
		return append([]string(nil), s.playerProfile.UnlockedHeroes...)
	}
	return []string{"recruit"}
}

func (s *webServer) heroUnlocked(id string) bool {
	for _, unlockedID := range s.unlockedHeroIDs() {
		if unlockedID == id {
			return true
		}
	}
	return false
}

func (s *webServer) heroOptions() []webHeroOption {
	return s.heroOptionsFor(s.playerProfile)
}

func (s *webServer) heroOptionsFor(p *profile.Profile) []webHeroOption {
	all := heroes.All()
	options := make([]webHeroOption, 0, len(all))
	for _, hero := range all {
		options = append(options, webHeroOption{
			ID: hero.ID, Name: hero.Name, Unlocked: p != nil && containsHeroID(p.UnlockedHeroes, hero.ID),
		})
	}
	return options
}

func (s *webServer) opponentLastHeroName() string {
	if s.playerProfile == nil {
		return ""
	}
	if skills.AggregateBonuses(s.playerProfile).HeroHistory && len(s.playerProfile.LastOpponentHeroIDs) > 0 {
		names := make([]string, 0, len(s.playerProfile.LastOpponentHeroIDs))
		for _, id := range s.playerProfile.LastOpponentHeroIDs {
			if hero, ok := heroes.ByID(id); ok {
				names = append(names, hero.Name)
			}
		}
		if len(names) > 0 {
			return "Last two: " + strings.Join(names, ", ")
		}
	}
	if hero, ok := heroes.ByID(s.playerProfile.OpponentLastHeroID); ok {
		return hero.Name
	}
	return ""
}

func validWebMode(mode string) bool {
	return mode == "hvh" || mode == "easy" || mode == "normal" || mode == "hard"
}

func isAI(mode string, playerIndex int) bool {
	return mode != "hvh" && playerIndex == 1
}

func webDifficulty(mode string) ai.Difficulty {
	switch mode {
	case "easy":
		return ai.Easy
	case "normal":
		return ai.Normal
	default:
		return ai.Hard
	}
}

func decodeJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprintf(os.Stderr, "encode web response: %v\n", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func cardPointer(card game.Card) *game.Card { return &card }
