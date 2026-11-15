package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/ai"
	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/ladder"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/puzzlebattle"
	"prisonersofwar/internal/ranked"
	"prisonersofwar/internal/skills"
	"prisonersofwar/internal/skins"
	"prisonersofwar/internal/storage"
	"prisonersofwar/internal/tugwar"
)

var webFiles = skins.Files

type webServer struct {
	gameID              string
	db                  *sql.DB
	mu                  sync.Mutex
	rng                 *rand.Rand
	defaultMode         string
	onlineViewer        bool
	viewerPlayer        int
	session             *webSession
	playerProfile       *profile.Profile
	previewProfile      *profile.Profile
	campDrawSession     *lootbox.CampDrawSession
	campDrawPreview     bool
	lastCampDraw        *lootbox.CampDrawResult
	lastCampDrawPreview bool
	cpuPuzzleBattle     *puzzlebattle.Match
}

func (s *webServer) playerName(playerIndex int) string {
	if s.session.mode == "hvh" {
		return fmt.Sprintf("Player %d", playerIndex+1)
	}
	if playerIndex == 0 {
		return "You"
	}
	switch s.session.mode {
	case "easy":
		return "Easy CPU"
	case "normal":
		return "Medium CPU"
	case "hard":
		return "Hard CPU"
	default:
		return "Opponent"
	}
}

func (s *webServer) draftTurnMessage(playerIndex int) string {
	name := s.playerName(playerIndex)
	if name == "You" || s.session.mode == "hvh" {
		return "Choose one card from the public pool."
	}
	return fmt.Sprintf("%s is choosing a card from the public pool.", name)
}

type webSession struct {
	state               game.GameState
	restartSnapshot     *webSession
	undoSnapshot        *webSession
	mode                string
	variant             string
	puzzle              *puzzleSession
	abandon             *abandonSession
	abandonDuel         *abandonDuelGame
	rummy               *rummySession
	ladder              *ladder.LadderGame
	poker               *pokerSession
	pokerDuel           *pokerDuelGame
	tugWar              *tugwar.Game
	tugWarSeries        *tugWarSeriesProgress
	pokerDuelCPUReadyAt time.Time
	ladderReadyAt       time.Time
	whitePlayer         int
	phase               string
	actor               int
	handoffTarget       int
	handoffPhase        string
	exchange            int
	attacker            int
	attackIndex         int
	attackCard          game.Card
	lastAttack          game.Card
	lastDefense         game.Card
	lastOutcome         game.Outcome
	hasLastBattle       bool
	slots               [9]jailCell
	message             string
	winner              int
	tiebreak            string
	playerHeroID        string
	opponentHeroID      string
	conductorTunings    [2]heroes.Tuning
	profileError        string
	payout              *webMatchPayout
	campaign            *webCampaignContext
	battles             []webBattleRecord
	initialAchievements map[string]int
	achievementUpdates  []string
	rankedSeries        *ranked.Series
}

type webMatchPayout struct {
	PlayerScore         int     `json:"playerScore"`
	PlayerWon           bool    `json:"playerWon"`
	Draw                bool    `json:"draw"`
	BaseMarks           int64   `json:"baseMarks"`
	WinMultiplier       int     `json:"winMultiplier"`
	SkillBonusPct       int     `json:"skillBonusPercent"`
	AIMultiplier        float64 `json:"aiMultiplier"`
	TotalMarks          int64   `json:"totalMarks"`
	MatchCrowns         int64   `json:"matchCrowns"`
	DailyWinBonusCrowns int64   `json:"dailyWinBonusCrowns"`
	TotalCrowns         int64   `json:"totalCrowns"`
	FirstWinOfDay       bool    `json:"firstWinOfDay"`
}

type webBattleOutcome struct {
	AttackerValue int  `json:"attackerValue"`
	DefenderValue int  `json:"defenderValue"`
	Winner        int  `json:"winner"`
	HideValues    bool `json:"-"`
}

func (o webBattleOutcome) MarshalJSON() ([]byte, error) {
	if o.HideValues {
		return json.Marshal(struct {
			Winner int `json:"winner"`
		}{Winner: o.Winner})
	}
	type outcomeAlias webBattleOutcome
	return json.Marshal(outcomeAlias(o))
}

type jailCell struct {
	Card       *webFaceCard `json:"card,omitempty"`
	CapturedBy string       `json:"capturedBy,omitempty"`
	Clash      bool         `json:"clash,omitempty"`
}

type webCard struct {
	Card       game.Card `json:"card"`
	Index      int       `json:"index"`
	Selectable bool      `json:"selectable"`
	Placeable  bool      `json:"placeable"`
	FaceHidden bool      `json:"faceHidden,omitempty"`
	FaceRange  string    `json:"faceRange,omitempty"`
}

type webFaceCard struct {
	game.Card
	FaceHidden bool   `json:"faceHidden,omitempty"`
	FaceRange  string `json:"faceRange,omitempty"`
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
	GameID             string                 `json:"gameId,omitempty"`
	Started            bool                   `json:"started"`
	CanAct             bool                   `json:"canAct,omitempty"`
	Viewer             int                    `json:"viewer,omitempty"`
	Dashboard          *webDashboard          `json:"dashboard,omitempty"`
	Mode               string                 `json:"mode,omitempty"`
	Variant            string                 `json:"variant"`
	PuzzleOptions      []puzzleDefinition     `json:"puzzleOptions"`
	Puzzle             *webPuzzleState        `json:"puzzle,omitempty"`
	Abandon            *webAbandonState       `json:"abandon,omitempty"`
	AbandonDuel        *webAbandonDuelState   `json:"abandonDuel,omitempty"`
	Rummy              *webRummyState         `json:"rummy,omitempty"`
	Ladder             *webLadderState        `json:"ladder,omitempty"`
	Poker              *webPokerState         `json:"poker,omitempty"`
	PokerDuel          *webPokerDuelState     `json:"pokerDuel,omitempty"`
	TugWar             *webTugWarState        `json:"tugWar,omitempty"`
	Phase              string                 `json:"phase,omitempty"`
	Round              int                    `json:"round,omitempty"`
	Exchange           int                    `json:"exchange,omitempty"`
	Actor              int                    `json:"actor"`
	Attacker           int                    `json:"attacker"`
	Bounty             game.Card              `json:"bounty"`
	Players            []webPlayer            `json:"players,omitempty"`
	DraftPool          []webCard              `json:"draftPool,omitempty"`
	DraftMode          int                    `json:"draftMode"`
	DraftHand          []webFaceCard          `json:"draftHand,omitempty"`
	DraftPicked        []webFaceCard          `json:"draftPicked,omitempty"`
	DraftOpponentCount int                    `json:"draftOpponentCount,omitempty"`
	DraftPickNumber    int                    `json:"draftPickNumber,omitempty"`
	DraftPickTotal     int                    `json:"draftPickTotal,omitempty"`
	JailSlots          []jailCell             `json:"jailSlots,omitempty"`
	BattleAttack       *webFaceCard           `json:"battleAttack,omitempty"`
	BattleDefend       *webFaceCard           `json:"battleDefend,omitempty"`
	BattleOutcome      *webBattleOutcome      `json:"battleOutcome,omitempty"`
	Message            string                 `json:"message,omitempty"`
	ScoreLine          string                 `json:"scoreLine,omitempty"`
	CanContinue        bool                   `json:"canContinue"`
	Winner             int                    `json:"winner"`
	Payout             *webMatchPayout        `json:"payout,omitempty"`
	RankedSeries       *ranked.Series         `json:"rankedSeries,omitempty"`
	Tiebreak           string                 `json:"tiebreak,omitempty"`
	Modifiers          game.GameModifiers     `json:"modifiers"`
	PlayerHero         string                 `json:"playerHero,omitempty"`
	OpponentHero       string                 `json:"opponentHero,omitempty"`
	PlayerHeroID       string                 `json:"playerHeroId,omitempty"`
	OpponentHeroID     string                 `json:"opponentHeroId,omitempty"`
	DefaultHeroID      string                 `json:"defaultHeroId,omitempty"`
	OpponentHint       string                 `json:"opponentHint,omitempty"`
	Heroes             []webHeroOption        `json:"heroes,omitempty"`
	Scores             [2]game.ScoreBreakdown `json:"scores"`
	Campaign           *webCampaignContext    `json:"campaign,omitempty"`
	Battles            []webBattleRecord      `json:"battles"`
	AchievementUpdates []string               `json:"achievementUpdates"`
}

type webDashboard struct {
	DisplayName           string                    `json:"displayName"`
	EquippedChromas       map[string]catalog.Chroma `json:"equippedChromas"`
	OwnedChromas          []string                  `json:"ownedChromas"`
	OwnedChromaDetails    []catalog.Chroma          `json:"ownedChromaDetails"`
	Commander             string                    `json:"commander"`
	Title                 string                    `json:"title"`
	DefaultHeroID         string                    `json:"defaultHeroId"`
	Marks                 int64                     `json:"marks"`
	Crowns                int64                     `json:"crowns"`
	Energy                int                       `json:"energy"`
	MaxEnergy             int                       `json:"maxEnergy"`
	EnergyNextSeconds     int                       `json:"energyNextSeconds"`
	EnergyRechargeSeconds int                       `json:"energyRechargeSeconds"`
	AdsLeft               int                       `json:"adsLeft"`
	DailyFeaturedItem     *webDailyFeaturedItem     `json:"dailyFeaturedItem,omitempty"`
	LoginStreakCurrent    int                       `json:"loginStreakCurrent"`
	LoginStreakBest       int                       `json:"loginStreakBest"`
	WinsSinceCampDraw     int                       `json:"winsSinceCampDraw"`
	WinsPerCampDraw       int                       `json:"winsPerCampDraw"`
	DrawsAvailable        int                       `json:"drawsAvailable"`
	RestartTokens         int                       `json:"restartMissionTokens"`
	UndoTokens            int                       `json:"undoMoveTokens"`
	CampaignSkipTokens    int                       `json:"campaignSkipTokens"`
	ReplaceTokens         int                       `json:"replaceCardTokens"`
	MatchTokensAllowed    bool                      `json:"matchTokensAllowed"`
	CanSkipMission        bool                      `json:"canSkipMission"`
	ReplaceOptions        []webReplaceOption        `json:"replaceOptions,omitempty"`
	CanRestart            bool                      `json:"canRestart"`
	CanUndo               bool                      `json:"canUndo"`
	PendingBoxes          int                       `json:"pendingBoxes"`
	SkillPoints           int                       `json:"skillPoints"`
	ConductorLevel        int                       `json:"conductorLevel"`
	UnseenAchievements    int                       `json:"unseenAchievements"`
	GamesPlayed           int                       `json:"gamesPlayed"`
}

type webDailyFeaturedItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Skin     string `json:"skin"`
	Rarity   string `json:"rarity"`
	Currency string `json:"currency"`
	Cost     int64  `json:"cost"`
	Date     string `json:"date"`
	Owned    bool   `json:"owned"`
}

type webHeroOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Unlocked  bool   `json:"unlocked"`
	Flavor    string `json:"flavor"`
	Bounty    int    `json:"bounty"`
	Shift     int    `json:"shift"`
	Favored   int    `json:"favored"`
	Draft     int    `json:"draft"`
	DraftFlip bool   `json:"draftFlip,omitempty"`
	Flip      bool   `json:"flip"`
	Unlock    string `json:"unlock"`
}

type actionRequest struct {
	Action            string `json:"action"`
	Mode              string `json:"mode"`
	Variant           string `json:"variant"`
	Index             int    `json:"index"`
	Amount            int    `json:"amount"`
	Slot              int    `json:"slot"`
	Cards             []int  `json:"cards"`
	PlayerHeroID      string `json:"playerHeroId"`
	OpponentHeroID    string `json:"opponentHeroId"`
	ConductorModifier string `json:"conductorModifier"`
	ConductorAmount   int    `json:"conductorAmount"`
	ID                string `json:"id"`
	Payment           string `json:"payment"`
	BoxType           string `json:"boxType"`
	Suit              string `json:"suit"`
	SuitIndex         int    `json:"suitIndex"`
	Source            string `json:"source"`
	FaceUpIndex       int    `json:"faceUpIndex"`
	BuriedIndex       int    `json:"buriedIndex"`
	CampaignID        string `json:"campaignId"`
	MissionIndex      int    `json:"missionIndex"`
	Recipient         string `json:"recipient"`
}

type webAchievement struct {
	ProgressUnit string               `json:"progressUnit,omitempty"`
	Rewards      string               `json:"rewards"`
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Category     string               `json:"category"`
	Details      string               `json:"details"`
	Hidden       bool                 `json:"hidden,omitempty"`
	Unlocked     bool                 `json:"unlocked"`
	New          bool                 `json:"new"`
	Progress     int                  `json:"progress"`
	Goal         int                  `json:"goal"`
	Tiers        []webAchievementTier `json:"tiers,omitempty"`
}

type webAchievementTier struct {
	Rewards string `json:"rewards"`
	Name    string `json:"name"`
	Goal    int    `json:"goal"`
	Marks   int64  `json:"marks"`
	Crowns  int64  `json:"crowns"`
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

type webShopHero struct {
	Hero   webHeroOption `json:"hero"`
	Owned  bool          `json:"owned"`
	CanBuy bool          `json:"canBuy"`
	Price  int64         `json:"price"`
	Plays  int           `json:"plays"`
	Wins   int           `json:"wins"`
}

type webShopBoxOffer struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	MarksCost  int64  `json:"marksCost,omitempty"`
	CrownsCost int64  `json:"crownsCost,omitempty"`
	Available  int    `json:"available"`
}

const (
	shopWoodenBoxMarksCost  int64 = 300
	shopWoodenBoxCrownsCost int64 = 13
	shopSilverBoxMarksCost  int64 = 600
	shopSilverBoxCrownsCost int64 = 25
	shopGoldBoxMarksCost    int64 = 1200
	shopGoldBoxCrownsCost   int64 = 50
)

type webShopEnergyOffer struct {
	Amount     int   `json:"amount"`
	MarksCost  int64 `json:"marksCost"`
	CrownsCost int64 `json:"crownsCost"`
}

type webShopCrownOffer struct {
	Crowns       int64 `json:"crowns"`
	PriceDollars int   `json:"priceDollars"`
}

type webShopSkillPointOffer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	CrownsCost int64  `json:"crownsCost"`
}

type webShopPage struct {
	RewardTables       map[string][]webBoxReward `json:"rewardTables"`
	Chromas            []webShopChroma           `json:"chromas"`
	Heroes             []webShopHero             `json:"heroes"`
	BoxOffers          []webShopBoxOffer         `json:"boxOffers"`
	CrownOffers        []webShopCrownOffer       `json:"crownOffers"`
	EnergyOffers       []webShopEnergyOffer      `json:"energyOffers"`
	ConsumableOffers   []webShopConsumableOffer  `json:"consumableOffers"`
	SkillPointOffers   []webShopSkillPointOffer  `json:"skillPointOffers"`
	GiftRecipients     []webGiftRecipient        `json:"giftRecipients"`
	Dashboard          *webDashboard             `json:"dashboard"`
	HeroBoxesAvailable int                       `json:"heroBoxesAvailable"`
	HeroBoxCost        int64                     `json:"heroBoxCost"`
	ChromaPickTokens   int                       `json:"chromaPickTokens"`
	Message            string                    `json:"message,omitempty"`
	HeroBoxResult      *heroes.HeroBoxResult     `json:"heroBoxResult,omitempty"`
}

type webShopConsumableOffer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	Available  int    `json:"available"`
	MarksCost  int64  `json:"marksCost"`
	CrownsCost int64  `json:"crownsCost"`
	CanBuy     bool   `json:"canBuy"`
}

type webSkillNode struct {
	Node      skills.Node `json:"node"`
	Unlocked  bool        `json:"unlocked"`
	CanUnlock bool        `json:"canUnlock"`
	Reason    string      `json:"reason,omitempty"`
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
	DrawsAvailable int                       `json:"drawsAvailable"`
	PendingBoxes   map[string]int            `json:"pendingBoxes"`
	Dashboard      *webDashboard             `json:"dashboard"`
	Started        bool                      `json:"started"`
	StartingHand   []game.Card               `json:"startingHand,omitempty"`
	Prompt         *lootbox.CampDrawPrompt   `json:"prompt,omitempty"`
	Exchanges      []lootbox.ExchangeResult  `json:"exchanges,omitempty"`
	Result         *lootbox.CampDrawResult   `json:"result,omitempty"`
	BoxResult      *lootbox.BoxOpenResult    `json:"boxResult,omitempty"`
	RewardChroma   *catalog.Chroma           `json:"rewardChroma,omitempty"`
	RewardChromas  map[string]catalog.Chroma `json:"rewardChromas,omitempty"`
	BoxChroma      *catalog.Chroma           `json:"boxChroma,omitempty"`
	Message        string                    `json:"message,omitempty"`
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
	Dashboard  *webDashboard        `json:"dashboard"`
	Campaigns  []webCampaignSummary `json:"campaigns"`
	SkipTokens int                  `json:"skipTokens"`
	Message    string               `json:"message,omitempty"`
}

type webCampaignSummary struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	Variant           string               `json:"variant"`
	Status            string               `json:"status,omitempty"`
	Requires          string               `json:"requires,omitempty"`
	Unlocked          bool                 `json:"unlocked"`
	Started           bool                 `json:"started"`
	Completed         bool                 `json:"completed"`
	MissionsCompleted int                  `json:"missionsCompleted"`
	TotalMissions     int                  `json:"totalMissions"`
	Missions          []webCampaignMission `json:"missions"`
}

type webCampaignMission struct {
	Heroes            []webHeroOption `json:"heroes"`
	OpponentHeroes    []webHeroOption `json:"opponentHeroes,omitempty"`
	RecommendedHeroes []string        `json:"recommendedHeroes,omitempty"`
	Index             int             `json:"index"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	Variant           string          `json:"variant"`
	VariantID         string          `json:"variantId,omitempty"`
	Status            string          `json:"status,omitempty"`
	Enabled           bool            `json:"enabled"`
	Target            int             `json:"target,omitempty"`
	CardCount         int             `json:"cardCount,omitempty"`
	PairCount         int             `json:"pairCount,omitempty"`
	RewardMarks       int64           `json:"rewardMarks"`
	RewardCrowns      int64           `json:"rewardCrowns,omitempty"`
	Completed         bool            `json:"completed"`
	CanPlay           bool            `json:"canPlay"`
	EnergyCost        int             `json:"energyCost"`
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
	return serveUntilSignal(listener, mux)
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

func (s *webServer) handleBuildInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]string{"hash": "unknown", "date": "unknown"}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if len(setting.Value) > 12 {
					setting.Value = setting.Value[:12]
				}
				info["hash"] = setting.Value
			case "vcs.time":
				info["date"] = setting.Value
			}
		}
	}
	writeJSON(w, http.StatusOK, info)
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
	assetPath := strings.TrimPrefix(name, "skins/")
	var asset []byte
	var err error
	if strings.HasPrefix(assetPath, "icons/") {
		if isChestIconAsset(assetPath) {
			asset, _, err = chestIconSpriteFrame(assetPath)
		} else {
			asset, err = skins.IconFiles.ReadFile(assetPath)
		}
	} else if strings.HasPrefix(assetPath, "shop/") && strings.HasSuffix(assetPath, ".png") {
		asset, err = skinSpriteFrame(assetPath)
	} else {
		asset, err = webFiles.ReadFile(assetPath)
	}
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "skin asset unavailable", http.StatusInternalServerError)
		return
	}
	if thumb := r.URL.Query().Get("thumb"); (thumb == "160" || thumb == "320") && strings.HasSuffix(name, ".png") {
		width := 160
		if thumb == "320" {
			width = 320
		}
		asset, err = thumbnailArtwork(name, asset, width)
		if err != nil {
			http.Error(w, "thumbnail unavailable", http.StatusInternalServerError)
			return
		}
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

func (s *webServer) handleSkillIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := chi.URLParam(r, "*")
	if name == "" {
		name = strings.TrimPrefix(r.URL.Path, "/skills/icons/")
	}
	if !fs.ValidPath(name) || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	asset, err := skills.IconFiles.ReadFile("icons/" + name)
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "skill icon unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(asset); err != nil {
		fmt.Fprintf(os.Stderr, "write skill icon response: %v\n", err)
	}
}

func (s *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil && s.session.variant == "ladder" {
		if err := s.advanceLadderAI(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
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
	if request.CampaignID != "" {
		s.startWebMission(w, r, request)
		return
	}
	variant, err := specialVariant(request.Variant)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if variant == "puzzler" || variant == "abandon" || variant == "poker" {
		writeError(w, http.StatusBadRequest, errors.New("Poker, Puzzler, and Abandon are only available as campaign missions"))
		return
	}
	mode := requestedWebMode(request.Mode, s.defaultMode)
	if !validWebMode(mode) {
		writeError(w, http.StatusBadRequest, errors.New("mode must be easy, normal, or hard; use online battles to play another person"))
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
	tuning, err := webConductorTuning(s.playerProfile, request.ConductorModifier, request.ConductorAmount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	unlockedIDs := s.unlockedHeroIDs()
	opponentHero, chooseErr := heroes.ChooseForOpponent(unlockedIDs, playerHeroID, s.rng)
	if chooseErr != nil {
		writeError(w, http.StatusInternalServerError, chooseErr)
		return
	}
	opponentHeroID := opponentHero.ID
	if s.playerProfile != nil {
		if _, err := refreshEnergy(s.playerProfile, time.Now()); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if s.playerProfile.Energy < energy.SingleMatchCost {
			writeError(w, http.StatusConflict, errInsufficientEnergy)
			return
		}
	}
	if variant == "abandon-duel" {
		playerHero = applyWebHeroTuning(playerHero, tuning)
		duel, err := newAbandonDuelGame(playerHero, opponentHero, s.rng)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		duel.Setup.OpponentAlgo = abandonDuelPolicyForMode(mode)
		s.session = &webSession{
			mode: mode, variant: variant, abandonDuel: duel,
			phase: duel.phase(), actor: duel.ActivePlayer, winner: -1,
			playerHeroID: playerHeroID, opponentHeroID: opponentHeroID,
			message:             "Flip a face-down board card, then choose two cards to battle.",
			initialAchievements: achievementLevels(s.playerProfile),
		}
	} else if variant == "poker-duel" {
		playerHero = applyWebHeroTuning(playerHero, tuning)
		duel, err := newPokerDuelGame(playerHero, opponentHero, s.rng, time.Now())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.session = &webSession{
			mode: mode, variant: variant, pokerDuel: duel, phase: "poker-duel-play",
			actor: duel.ActivePlayer, handoffTarget: duel.ActivePlayer,
			handoffPhase: "poker-duel-play", winner: -1,
			playerHeroID: playerHeroID, opponentHeroID: opponentHeroID,
			message:             "Your turn. Place or redraw a card before the 15-second clock expires.",
			initialAchievements: achievementLevels(s.playerProfile),
		}
	} else if variant == "tug-war" {
		playerHero = applyWebHeroTuning(playerHero, tuning)
		if err := s.startWebTugWar(mode, playerHeroID, opponentHeroID, playerHero, opponentHero); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else if variant == "rummy" {
		playerHero = applyWebHeroTuning(playerHero, tuning)
		s.startWebRummy(mode, playerHeroID, opponentHeroID, playerHero, opponentHero)
	} else if variant == "ladder" {
		playerHero = applyWebHeroTuning(playerHero, tuning)
		if err := s.startWebLadder(mode, playerHeroID, opponentHeroID, playerHero, opponentHero); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		gameModifiers, err := heroes.GameModifiersForWithTuning(playerHero, opponentHero, tuning, nil, s.rng)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		state, err := game.NewDraftGameWithDraftMode(s.rng, gameModifiers, heroes.DraftMode(playerHero, opponentHero))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := s.startWebGameSession(state, mode, playerHeroID, opponentHeroID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.session.variant = variant
	}
	if s.playerProfile != nil {
		if err := energy.Spend(s.playerProfile, energy.SingleMatchCost, time.Now()); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
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
	if err := s.captureRestartSnapshot(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.advanceAI(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}

func (s *webServer) startWebGameSession(state game.GameState, mode, playerHeroID, opponentHeroID string) error {
	whitePlayer, err := state.PlayerAtSeat(game.SeatX)
	if err != nil {
		return err
	}
	draftActor := 0
	phase := "commit"
	message := "Choose three cards from your hand to commit for Round 1."
	if state.Draft != nil {
		draftSeat, _ := state.Draft.CurrentSeat()
		draftActor, err = state.PlayerAtSeat(draftSeat)
		if err != nil {
			return err
		}
		phase = "draft"
	}
	session := &webSession{
		initialAchievements: achievementLevels(s.playerProfile),
		state:               state,
		mode:                mode,
		variant:             "main",
		whitePlayer:         whitePlayer,
		phase:               phase,
		actor:               draftActor,
		winner:              -1,
		playerHeroID:        playerHeroID,
		opponentHeroID:      opponentHeroID,
	}
	s.session = session
	if state.Draft != nil {
		session.message = s.draftTurnMessage(draftActor)
	} else {
		session.message = message
	}
	if mode == "hvh" && state.Draft != nil {
		session.phase = "handoff"
		session.handoffTarget = draftActor
		session.handoffPhase = "draft"
		session.message = fmt.Sprintf("Pass the screen to Player %d for their private draft pick.", draftActor+1)
	}
	return nil
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
	recordUndoPoint := !isAutomaticWebAction(request.Action) && !isConsumableWebAction(request.Action)
	var previous *webSession
	if recordUndoPoint && canUseWebMatchTokens(s.session, s.onlineViewer) && s.session.phase != "gameover" {
		var err error
		previous, err = cloneWebSession(s.session)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save undo point: %w", err))
			return
		}
	}
	if err := s.applyAction(request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !isConsumableWebAction(request.Action) || request.Action == "use-restart-token" {
		if err := s.advanceAI(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if canUseWebMatchTokens(s.session, s.onlineViewer) && s.session.phase != "gameover" {
			if recordUndoPoint {
				s.session.undoSnapshot = previous
			}
		} else if s.session != nil {
			s.session.undoSnapshot = nil
		}
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

func requestedWebMode(mode, fallback string) string {
	if mode == "" {
		return fallback
	}
	return mode
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
	achievementMessage := ""
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
			result, err := s.campDrawSession.FinishWithRewardChoice(p, s.rng)
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
		achievementMessage = achievementUnlockSummary(recordBoxOpened(p))
		if err := saveWebProfile(p, preview); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save pending-box opening: %w", err))
			return
		}
	case "choose-reward":
		if s.lastCampDraw == nil || s.lastCampDrawPreview != preview || !s.lastCampDraw.AwaitingChoice {
			writeError(w, http.StatusConflict, errors.New("no Camp Draw reward choice is pending"))
			return
		}
		if err := lootbox.SelectCampDrawReward(p, s.lastCampDraw, request.Index); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := saveWebProfile(p, preview); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save selected Camp Draw reward: %w", err))
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errors.New("unknown Camp Draw action"))
		return
	}
	page := s.campDrawPage(p, preview, boxResult)
	page.Message = achievementMessage
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
		page.RewardChroma = chromaPointer(s.lastCampDraw.Reward.ChromaID)
		if len(s.lastCampDraw.RewardOptions) > 0 {
			page.RewardChromas = make(map[string]catalog.Chroma)
			for _, reward := range s.lastCampDraw.RewardOptions {
				if chroma, ok := catalog.FindChroma(reward.ChromaID); ok {
					page.RewardChromas[reward.ChromaID] = chroma
				}
			}
		}
	}
	if boxResult != nil {
		page.BoxChroma = chromaPointer(boxResult.ChromaID)
	}
	return page
}

// chromaPointer returns the catalog entry for a rewarded chroma so the page can
// show its artwork, or nil when the reward was not a chroma.
func chromaPointer(id string) *catalog.Chroma {
	if id == "" {
		return nil
	}
	if chroma, ok := catalog.FindChroma(id); ok {
		return &chroma
	}
	return nil
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
	evaluateAchievementEvent(p, achievements.EventProfileLoad, "browser")
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
			Rewards: achievementRewardDescription(
				definition.RewardMarks, definition.RewardCrowns, definition.RewardBadge,
				definition.RewardChromaPick, definition.RewardHeroBox, definition.RewardWoodenBoxes,
				definition.RewardSilverBoxes, definition.RewardGoldBoxes, definition.RewardSkillPoints,
				definition.RewardUndoTokens,
				definition.RewardRestartTokens, definition.RewardSkipTokens, definition.RewardSwapTokens),
			ID: definition.ID, Name: definition.Name, Category: categoryName(definition.Category),
			Details: definition.Description, Hidden: definition.Hidden && !unlocked,
			Unlocked: unlocked, New: discovered && !seen[definition.ID], Progress: progress,
			ProgressUnit: definition.ProgressUnit, Tiers: make([]webAchievementTier, 0, len(definition.Tiers)),
		}
		if item.Hidden {
			item.Name, item.Details = "???", "Hidden achievement"
			item.Progress = 0
		} else if len(definition.Tiers) == 0 {
			item.Goal = definition.Goal
			if item.Goal == 0 {
				item.Goal = 1
			}
			if definition.CounterID == "" && unlocked {
				item.Progress = item.Goal
			}
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
					Rewards: achievementRewardDescription(
						achievementTier.RewardMarks, achievementTier.RewardCrowns, achievementTier.RewardBadge,
						achievementTier.RewardChromaPick, achievementTier.RewardHeroBox,
						achievementTier.RewardWoodenBoxes, achievementTier.RewardSilverBoxes,
						achievementTier.RewardGoldBoxes, achievementTier.RewardSkillPoints,
						achievementTier.RewardUndoTokens, achievementTier.RewardRestartTokens,
						achievementTier.RewardSkipTokens, achievementTier.RewardSwapTokens),
					Name: achievementTier.Name, Goal: achievementTier.Goal,
					Marks: achievementTier.RewardMarks, Crowns: achievementTier.RewardCrowns,
				})
			}
		}
		if !item.Hidden && len(item.Tiers) == 0 {
			if count, goal, unit := achievementCounter(p, definition); goal > 0 {
				item.Progress, item.Goal, item.ProgressUnit = count, goal, unit
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
			ID: set.ID, Name: set.Name, Complete: completed == len(set.Members),
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
		page := s.shopPage(p)
		if user, ok := r.Context().Value(accountKey).(account); ok && s.db != nil {
			page.GiftRecipients, err = s.shopGiftRecipients(r.Context(), user.ID)
			if err != nil {
				databaseError(w, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, page)
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
	evaluateAchievementEvent(p, achievements.EventSkillUnlocked, request.ID)
	if err := saveWebProfile(p, preview); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("save shop purchase: %w", err))
		return
	}
	page := s.shopPage(p)
	if user, ok := r.Context().Value(accountKey).(account); ok && s.db != nil {
		page.GiftRecipients, err = s.shopGiftRecipients(r.Context(), user.ID)
		if err != nil {
			databaseError(w, err)
			return
		}
	}
	page.Message = message
	writeJSON(w, http.StatusOK, page)
}

func (s *webServer) shopPage(p *profile.Profile) webShopPage {
	page := webShopPage{
		RewardTables: boxRewardTables(p),
		Chromas:      make([]webShopChroma, 0, len(catalog.AllChromas())),
		Heroes:       make([]webShopHero, 0, len(heroes.All())),
		Dashboard:    dashboardFor(p), HeroBoxesAvailable: p.HeroBoxesAvailable,
		HeroBoxCost:      heroes.HeroBoxCost(p),
		ChromaPickTokens: p.ChromaPickTokens,
		CrownOffers: []webShopCrownOffer{
			{Crowns: 100, PriceDollars: 1},
			{Crowns: 550, PriceDollars: 5},
			{Crowns: 1150, PriceDollars: 10},
		},
		BoxOffers: []webShopBoxOffer{
			{Type: "wooden", Name: "Wooden Box", MarksCost: shopChestMarksCost(p, shopWoodenBoxMarksCost), CrownsCost: shopWoodenBoxCrownsCost, Available: p.PendingBoxes["wooden"]},
			{Type: "silver", Name: "Silver Box", MarksCost: shopChestMarksCost(p, shopSilverBoxMarksCost), CrownsCost: shopSilverBoxCrownsCost, Available: p.PendingBoxes["silver"]},
			{Type: "gold", Name: "Gold Box", MarksCost: shopChestMarksCost(p, shopGoldBoxMarksCost), CrownsCost: shopGoldBoxCrownsCost, Available: p.PendingBoxes["gold"]},
			{Type: "hero", Name: "Hero Box", MarksCost: heroes.HeroBoxCost(p), Available: p.HeroBoxesAvailable},
		},
		EnergyOffers: []webShopEnergyOffer{
			{Amount: energy.OneEnergyRecharge, MarksCost: energy.MarksPerEnergy, CrownsCost: energy.CrownsPerEnergy},
			{Amount: energy.TenEnergyRecharge, MarksCost: energy.MarksPerTenEnergy, CrownsCost: energy.CrownsPerTenEnergy},
		},
		ConsumableOffers: shopConsumableOffers(p),
		SkillPointOffers: shopSkillPointOffers(),
	}
	for _, chroma := range catalog.AllChromas() {
		page.Chromas = append(page.Chromas, webShopChroma{Chroma: chroma, Owned: p.OwnedChromas[chroma.ID]})
	}
	for _, hero := range heroes.All() {
		owned := containsHeroID(p.UnlockedHeroes, hero.ID)
		price, purchasable := profile.HeroCrownsCost(hero.ID)
		page.Heroes = append(page.Heroes, webShopHero{
			Hero:  heroOption(hero, owned),
			Owned: owned, CanBuy: !owned && purchasable && p.Crowns >= price, Price: price,
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
	case "buy-chroma-family":
		cost, currency, err := catalog.BuyChromaFamily(p, request.ID)
		if err != nil {
			return "", err
		}
		evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
		currencyName := "Marks"
		if currency == catalog.Crowns {
			currencyName = "Crowns"
		}
		return fmt.Sprintf("Purchased family colors for %d %s.", cost, currencyName), nil
	case "buy-box":
		return buyShopBox(p, request.BoxType, request.Payment)
	case "buy-energy":
		if err := energy.BuyEnergy(p, request.Amount, request.Payment, time.Now()); err != nil {
			return "", err
		}
		evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
		currencyName := "Marks"
		if request.Payment == "crowns" {
			currencyName = "Crowns"
		}
		return fmt.Sprintf("Purchased %d energy for %d %s.",
			request.Amount, energyPurchaseCost(request.Amount, request.Payment), currencyName), nil
	case "buy-consumable":
		message, err := buyShopConsumable(p, request.ID, request.Payment)
		if err != nil {
			return "", err
		}
		evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
		return message, nil
	case "buy-skill-points":
		message, err := buyShopSkillPoints(p, request.ID)
		if err != nil {
			return "", err
		}
		evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
		return message, nil
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
		boxEvents := recordBoxOpened(p)
		if result.Duplicate {
			hero, ok := heroes.ByID(result.HeroID)
			if !ok {
				return "", fmt.Errorf("Hero Box returned unknown duplicate hero %q", result.HeroID)
			}
			return fmt.Sprintf("Duplicate %s: +%d Crowns.%s", hero.Name, result.Crowns,
				achievementUnlockSummary(boxEvents)), nil
		}
		hero, ok := heroes.ByID(result.HeroID)
		if !ok {
			return "", fmt.Errorf("Hero Box returned unknown hero %q", result.HeroID)
		}
		evaluateAchievementEvent(p, achievements.EventHeroAcquired, "box")
		return "Hero Box unlocked " + hero.Name + "." + achievementUnlockSummary(boxEvents), nil
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
		return "Claimed " + chromaName(request.ID) + ".", nil
	case "assign-chroma":
		if err := catalog.AssignChroma(p, request.Suit, request.ID); err != nil {
			return "", err
		}
		return "Applied " + chromaName(request.ID) + " to " + suitDisplayName(request.Suit) + ".", nil
	default:
		return applySuitAppearance(p, request)
	}
}

func buyShopBox(p *profile.Profile, boxType, payment string) (string, error) {
	if p == nil {
		return "", errors.New("profile is required")
	}
	var cost int64
	switch boxType {
	case "wooden":
		if payment == "marks" {
			cost = shopChestMarksCost(p, shopWoodenBoxMarksCost)
		} else if payment == "crowns" {
			cost = shopWoodenBoxCrownsCost
		}
	case "silver":
		if payment == "marks" {
			cost = shopChestMarksCost(p, shopSilverBoxMarksCost)
		} else if payment == "crowns" {
			cost = shopSilverBoxCrownsCost
		}
	case "gold":
		if payment == "marks" {
			cost = shopChestMarksCost(p, shopGoldBoxMarksCost)
		} else if payment == "crowns" {
			cost = shopGoldBoxCrownsCost
		}
	case "hero":
		if payment == "marks" {
			cost = heroes.HeroBoxCost(p)
		}
	default:
		return "", fmt.Errorf("unknown box type %q", boxType)
	}
	if cost == 0 {
		return "", fmt.Errorf("unsupported payment %q for %s box", payment, boxType)
	}
	if payment == "marks" && p.Marks < cost {
		return "", fmt.Errorf("need %d Marks; you have %d", cost, p.Marks)
	}
	if payment == "crowns" && p.Crowns < cost {
		return "", fmt.Errorf("need %d Crowns; you have %d", cost, p.Crowns)
	}
	if boxType == "hero" {
		p.HeroBoxesAvailable++
	} else if err := p.GrantBox(boxType, 1); err != nil {
		return "", err
	}
	if payment == "marks" {
		p.Marks -= cost
	} else {
		p.Crowns -= cost
	}
	evaluateAchievementEvent(p, achievements.EventPurchase, "shop")
	return "Purchased " + boxType + " box.", nil
}

func energyPurchaseCost(amount int, currency string) int64 {
	switch amount {
	case energy.OneEnergyRecharge:
		if currency == "crowns" {
			return energy.CrownsPerEnergy
		}
		return energy.MarksPerEnergy
	case energy.TenEnergyRecharge:
		if currency == "crowns" {
			return energy.CrownsPerTenEnergy
		}
		return energy.MarksPerTenEnergy
	default:
		return 0
	}
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
		case "skill-point":
			err = skills.UnlockNodeWithSkillPoint(p, request.ID)
		default:
			err = errors.New("skill nodes can only be unlocked with Skill Points")
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		unlockEvents := evaluateAchievementEvent(p, achievements.EventSkillUnlocked, request.ID)
		if err := saveWebProfile(p, preview); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save skill-tree unlock: %w", err))
			return
		}
		message = "Unlocked " + skillName(request.ID) + "." + achievementUnlockSummary(unlockEvents)
	}
	page := s.skillsPage(p)
	page.Message = message
	writeJSON(w, http.StatusOK, page)
}

func (s *webServer) skillsPage(p *profile.Profile) webSkillsPage {
	page := webSkillsPage{Nodes: make([]webSkillNode, 0, len(skills.AllNodes())), Dashboard: dashboardFor(p)}
	for _, node := range skills.AllNodes() {
		canPoint, pointReason := skills.CanUnlockWithSkillPoint(p, node.ID)
		page.Nodes = append(page.Nodes, webSkillNode{
			Node: node, Unlocked: p.SkillNodes[node.ID], CanUnlock: canPoint, Reason: pointReason,
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
		options = append(options, heroOption(hero, containsHeroID(p.UnlockedHeroes, hero.ID)))
	}
	return webHeroesPage{Heroes: options, DefaultID: p.DefaultHeroID, Dashboard: dashboardFor(p)}
}

func (s *webServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
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
	if r.Method == http.MethodPost {
		var input struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		name := strings.TrimSpace(input.Name)
		if len([]rune(name)) < 1 || len([]rune(name)) > 40 || strings.ContainsAny(name, "\r\n\t") {
			writeError(w, http.StatusBadRequest, errors.New("use a display name between 1 and 40 characters on one line"))
			return
		}
		p.DisplayName = name
		if err := p.Save(); err != nil {
			databaseError(w, err)
			return
		}
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
		if request.Action != "skip-mission" {
			writeError(w, http.StatusBadRequest, errors.New("unknown campaign action"))
			return
		}
		message, err = skipCampaignMission(p, request.CampaignID, request.MissionIndex)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := saveWebProfile(p, preview); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("save skipped campaign mission: %w", err))
			return
		}
	}
	definitions := campaigns.All()
	page := webCampaignPage{
		Dashboard: dashboardFor(p), SkipTokens: p.CampaignSkipTokens, Message: message,
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
			Variant: campaignVariant(campaign), Status: campaign.Status,
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
			name, description, target, cardCount := campaignMissionMetadata(mission)
			opponentHeroes := []webHeroOption(nil)
			if mission.Variant == "ladder" && mission.Index == 15 && mission.AIHero == "random" {
				opponentHeroes = s.heroOptions()
			}
			summary.Missions = append(summary.Missions, webCampaignMission{
				Heroes: s.missionHeroOptions(mission), RecommendedHeroes: campaignRecommendedHeroes(mission),
				OpponentHeroes: opponentHeroes,
				Index:          mission.Index, Name: name, Description: description,
				Variant: missionVariant(mission), VariantID: mission.VariantID,
				Status: mission.Status, Enabled: !mission.Disabled,
				Target: target, CardCount: cardCount, PairCount: mission.AbandonPairs,
				RewardMarks: mission.RewardMarks, Completed: completed,
				RewardCrowns: mission.RewardCrowns,
				CanPlay:      campaigns.CanPlayMission(p, campaign.ID, mission.Index),
				EnergyCost:   campaignMissionEnergyCost(mission),
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
	if request.Action == "use-restart-token" {
		return s.useRestartMissionToken()
	}
	if request.Action == "use-undo-token" {
		return s.useUndoMoveToken()
	}
	if request.Action == "use-replace-token" {
		return s.useReplaceCardToken(request.Source, request.Index)
	}
	if request.Action == "use-campaign-skip-token" {
		return s.useCampaignSkipToken()
	}
	if request.Action == "quick-buy-token" {
		return s.quickBuyToken(request.ID)
	}
	if session.variant == "tug-war" {
		if request.Action == "next-tug-war-game" {
			return s.nextTugWarCampaignGame()
		}
		return s.applyTugWarAction(request)
	}
	if session.variant == "ladder" {
		if request.Action == "ready" {
			return s.readyAction()
		}
		return s.applyLadderAction(request)
	}
	if session.variant == "rummy" {
		if request.Action == "ready" {
			return s.readyAction()
		}
		return s.applyRummyAction(request)
	}
	if session.variant == "poker" {
		return s.applyPokerAction(request)
	}
	if session.variant == "poker-duel" {
		if request.Action == "ready" {
			return s.readyAction()
		}
		return s.applyPokerDuelAction(request)
	}
	if session.variant == "puzzler" {
		if request.Action != "puzzle-pick" {
			return errors.New("unknown puzzler action")
		}
		if session.phase != "puzzle-pick" || session.puzzle == nil {
			return errors.New("puzzle is not accepting picks")
		}
		if err := session.puzzle.pick(request.Index); err != nil {
			return err
		}
		if len(session.puzzle.Pairings) == len(session.puzzle.Definition.Cards)/2 {
			session.phase = "gameover"
			session.winner = 1
			if session.puzzle.Won {
				session.winner = 0
				session.message = "Puzzle solved."
			} else {
				session.message = "Puzzle complete."
			}
			s.finishSpecialCampaignMission(session.puzzle.Won)
		} else if session.puzzle.Selected >= 0 {
			session.message = "Choose a card to defend."
		} else {
			session.message = "Choose an attacker."
		}
		return nil
	}
	if session.variant == "abandon" {
		hero, ok := heroes.ByID(session.playerHeroID)
		if !ok {
			return errors.New("selected hero is unavailable")
		}
		if session.abandon == nil {
			return errors.New("Abandon game state is unavailable")
		}
		switch request.Action {
		case "abandon-flip":
			if session.phase != "abandon-reveal" {
				return errors.New("flip a board card at the start of each turn")
			}
			if err := session.abandon.flipBoardCard(request.Index); err != nil {
				return err
			}
			session.phase = "abandon-pair-first"
			session.message = "Choose the first card for the battle."
		case "abandon-reveal-pair-card":
			if session.phase != "abandon-pair-second" {
				return errors.New("choose the first card before revealing a second")
			}
			if err := session.abandon.revealPairCard(request.Source, request.Index); err != nil {
				return err
			}
		case "abandon-pair-card":
			if session.phase != "abandon-pair-first" && session.phase != "abandon-pair-second" {
				return errors.New("flip a board card before pairing")
			}
			if err := session.abandon.selectPairCard(request.Source, request.Index, hero); err != nil {
				return err
			}
			if session.abandon.SelectedSource != "" {
				session.phase = "abandon-pair-second"
				session.message = "Choose a second, different card."
			} else {
				s.updateAbandonPhase()
			}
		default:
			return errors.New("unknown abandon action")
		}
		return nil
	}
	if session.variant == "abandon-duel" {
		duel := session.abandonDuel
		if duel == nil {
			return errors.New("Abandon Duel state is unavailable")
		}
		if session.actor != duel.ActivePlayer || !s.onlineViewer && isAI(session.mode, duel.ActivePlayer) {
			return errors.New("it is not your Abandon Duel turn")
		}
		switch request.Action {
		case "abandon-duel-flip":
			if session.phase != "abandon-duel-reveal" {
				return errors.New("flip a board card before pairing")
			}
			if err := duel.flipBoardCard(request.Index); err != nil {
				return err
			}
			session.phase = "abandon-duel-pair-first"
			session.message = "Choose the first card for the battle."
		case "abandon-duel-reveal-pair-card":
			if session.phase != "abandon-duel-pair-second" {
				return errors.New("choose the first card before revealing a second")
			}
			if err := duel.revealPairCard(request.Source, request.Index); err != nil {
				return err
			}
		case "abandon-duel-pair-card":
			if session.phase != "abandon-duel-pair-first" && session.phase != "abandon-duel-pair-second" {
				return errors.New("flip a board card before pairing")
			}
			movingPlayer := duel.ActivePlayer
			previousRound := duel.Round
			completed, err := duel.selectPairCard(request.Source, request.Index, s.rng)
			if err != nil {
				return err
			}
			session.actor = duel.ActivePlayer
			session.phase = duel.phase()
			if duel.GameOver {
				session.winner = duel.Winner
				session.message = abandonDuelResultMessage(duel, s.playerName)
				s.finishAbandonDuel()
			} else if previousRound != duel.Round {
				session.message = "Round 1 complete. Seats swapped; Round 2 begins."
			} else if completed {
				session.message = fmt.Sprintf("%s completed a pair.", s.playerName(movingPlayer))
			} else {
				session.message = "Choose a second, different card."
			}
		default:
			return errors.New("unknown Abandon Duel action")
		}
		return nil
	}
	switch request.Action {
	case "commit":
		return s.commitAction(request.Cards)
	case "draft-pick":
		return s.draftPickAction(request.Index)
	case "draft-ai-pick":
		if session.phase != "draft" || !isAI(session.mode, session.actor) {
			return errors.New("there is no pending AI draft pick")
		}
		return s.chooseAIDraftPick()
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
	if mission, ok := s.campaignMission(); ok && mission.Type == campaigns.SingleRound {
		s.beginBattles()
		return nil
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
	if session.variant == "poker-duel" &&
		(session.pokerDuel == nil || session.handoffPhase != "poker-duel-play" ||
			session.handoffTarget != session.pokerDuel.ActivePlayer) {
		return errors.New("Poker duel turn is unavailable")
	}
	session.actor = session.handoffTarget
	session.phase = session.handoffPhase
	session.handoffPhase = ""
	if session.variant == "poker-duel" {
		session.pokerDuel.TurnStarted = time.Now()
		session.message = fmt.Sprintf("Player %d, make your move before the 15-second clock expires.", session.actor+1)
	} else if session.variant == "ladder" && session.phase == "ladder-opening-reveal" {
		session.ladderReadyAt = time.Now().Add(3 * time.Second)
		session.message = "Take a moment to review your full hand. The opening card will be played automatically."
	} else if session.variant == "ladder" && session.phase == "ladder-opening-choice" {
		session.message = "Choose a card to open the new ladder."
	} else if session.variant == "ladder" {
		session.message = "Choose a legal card to play."
	} else if session.phase == "rummy-turn" {
		session.message = fmt.Sprintf("Player %d, take the face-up card or draw two cards.", session.actor+1)
	} else if session.phase == "commit" {
		session.message = fmt.Sprintf("Player %d, choose three cards to commit.", session.actor+1)
	} else if session.phase == "attack" {
		session.message = fmt.Sprintf("Player %d attacks. Choose a card from their pile.", session.actor+1)
	} else if session.phase == "draft" {
		session.message = s.draftTurnMessage(session.actor)
	} else {
		session.message = "Choose a response to the revealed attack."
	}
	return nil
}

func (s *webServer) advanceAI() error {
	session := s.session
	if session != nil && session.variant == "tug-war" {
		return s.advanceTugWarAI()
	}
	if session != nil && session.variant == "ladder" {
		return s.advanceLadderAI()
	}
	if session != nil && session.variant == "rummy" {
		if isAI(session.mode, session.actor) {
			time.Sleep(500 * time.Millisecond)
		}
		return s.advanceRummyAI()
	}
	if session != nil && session.variant == "poker-duel" {
		return s.schedulePokerDuelCPU()
	}
	if session.variant == "abandon-duel" {
		return s.advanceAbandonDuelAI()
	}
	if session != nil && (session.variant == "puzzler" || session.variant == "abandon" || session.variant == "poker") {
		return nil
	}
	for session != nil && isAI(session.mode, session.actor) {
		switch session.phase {
		case "draft":
			return nil
		case "commit":
			indices, err := s.chooseWebCPUCommit()
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
			index, err := s.chooseWebCPUAttack()
			if err != nil {
				return err
			}
			if err := s.selectAttack(index); err != nil {
				return err
			}
		case "defend":
			index, err := s.chooseWebCPUDefense()
			if err != nil {
				return err
			}
			if err := s.resolveBattle(index); err != nil {
				return err
			}
		default:
			return nil
		}
	}
	return nil
}

func (s *webServer) updateAbandonPhase() {
	session := s.session
	if session.abandon == nil {
		session.phase = "gameover"
		return
	}
	if session.abandon.Finished {
		session.phase = "gameover"
		session.winner = 1
		session.message = "Abandon complete."
		if session.abandon.Won {
			session.winner = 0
		} else {
			session.message = "Abandon complete. Target not reached."
		}
		s.finishSpecialCampaignMission(session.abandon.Won)
		return
	}
	session.phase = "abandon-reveal"
	session.message = "Flip a face-down board card."
}

func (s *webServer) draftPickAction(index int) error {
	session := s.session
	if session.phase != "draft" || session.state.Draft == nil || isAI(session.mode, session.actor) {
		return errors.New("it is not your draft turn")
	}
	if _, _, err := session.state.Draft.Pick(index); err != nil {
		return err
	}
	return s.advanceDraftPicker(session.actor)
}

func (s *webServer) chooseAIDraftPick() error {
	session := s.session
	if session.state.Draft == nil {
		return errors.New("game has no active draft")
	}
	seat, ok := session.state.Draft.CurrentSeat()
	if !ok {
		return errors.New("draft has no pending AI pick")
	}
	playerIndex, err := session.state.PlayerAtSeat(seat)
	if err != nil {
		return err
	}
	if playerIndex != session.actor {
		return errors.New("draft actor does not match the current seat")
	}
	hand, drafted, _ := draftPrivateView(session.state.Draft, seat)
	aiHand := append(append([]game.Card(nil), hand...), drafted...)
	choice, err := ai.AIDraftPick(session.state.Draft.Pool,
		aiHand, session.state.Bounty.Suit, webDifficulty(session.mode), s.rng)
	if err != nil {
		return err
	}
	for index, card := range session.state.Draft.Pool {
		if card == choice {
			if _, _, err := session.state.Draft.Pick(index); err != nil {
				return err
			}
			return s.advanceDraftPicker(playerIndex)
		}
	}
	return errors.New("AI selected a card outside the draft pool")
}

func (s *webServer) advanceDraftPicker(previousActor int) error {
	session := s.session
	if session.state.Draft.CurrentPick == len(session.state.Draft.PickOrder) {
		if err := session.state.CompleteDraft(); err != nil {
			return err
		}
		session.actor = 0
		session.phase = "commit"
		session.message = "Draft complete. Choose three cards from your hand to commit for Round 1."
		if session.mode == "hvh" {
			session.phase = "handoff"
			session.handoffTarget = 0
			session.handoffPhase = "commit"
			session.message = "Pass the screen to Player 1 to choose their Round 1 pile."
		}
		if s.playerProfile != nil {
			s.playerProfile.DraftStats.TotalDraftsCompleted++
			if err := s.playerProfile.Save(); err != nil {
				return fmt.Errorf("save opening draft statistics: %w", err)
			}
		}
		return nil
	}
	seat, ok := session.state.Draft.CurrentSeat()
	if !ok {
		return errors.New("draft turn did not advance")
	}
	nextActor, err := session.state.PlayerAtSeat(seat)
	if err != nil {
		return err
	}
	session.actor = nextActor
	session.phase = "draft"
	session.message = s.draftTurnMessage(nextActor)
	if session.mode == "hvh" && previousActor != nextActor {
		session.phase = "handoff"
		session.handoffTarget = nextActor
		session.handoffPhase = "draft"
		session.message = fmt.Sprintf("Pass the screen to Player %d for their private draft pick.", nextActor+1)
	}
	return nil
}

func draftPrivateView(draft *game.DraftState, seat game.Seat) (hand, drafted []game.Card, opponentCount int) {
	if seat == game.SeatX {
		return draft.DealtX, draft.PickedX, len(draft.PickedY)
	}
	return draft.DealtY, draft.PickedY, len(draft.PickedX)
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
	if mission, ok := s.campaignMission(); ok && mission.Type == campaigns.SingleRound && mission.CPUAttacksAll {
		attacker = 1
	}
	if mission, ok := s.campaignMission(); ok && mission.Type == campaigns.ThreeRound && mission.PlayerAttacksFirstAll {
		attacker = 0
		if session.exchange == 1 {
			attacker = 1
		}
	}
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
		if _, ok := s.campaignMission(); ok && attacker == 1 {
			index, chooseErr := s.chooseWebCPUAttack()
			if chooseErr != nil {
				session.message = chooseErr.Error()
				return
			}
			session.attackIndex = index
		}
		session.attackCard = attackerPlayer.Pool[session.attackIndex]
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
		defendIndex, err := s.chooseWebCPUDefense()
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
	attackCard, defenseCard, outcome, err := s.playWebExchange(defendIndex)
	if err != nil {
		return err
	}
	session.lastAttack, session.lastDefense, session.lastOutcome = attackCard, defenseCard, outcome
	session.attackCard = attackCard
	session.battles = append(session.battles, webBattleRecord{Round: session.state.Round, Exchange: session.exchange + 1, Attacker: s.scoreLabel(attackerIndex), AttackerIndex: attackerIndex, Attack: webFaceCard{Card: attackCard}, Defense: webFaceCard{Card: defenseCard}, Outcome: webBattleOutcome{AttackerValue: outcome.AttackerValue, DefenderValue: outcome.DefenderValue, Winner: outcome.Winner}, Scores: [2]int{game.ScorePlayer(session.state.P1, session.state.Bounty.Suit, session.state.Modifiers).Total, game.ScorePlayer(session.state.P2, session.state.Bounty.Suit, session.state.Modifiers).Total}})
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
	if mission, ok := s.campaignMission(); ok && (mission.Type == campaigns.SingleRound || session.state.Round == 3) {
		s.finishWebMission()
		return nil
	}
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
		s.assignRoundThreeSeats()
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

func (s *webServer) assignRoundThreeSeats() {
	session := s.session
	assignRoundThreeSeats(&session.state)
	session.actor = 0
	s.session.phase = "commit"
	s.session.message = "Round 3: choose three cards from your hand to commit."
	if session.mode == "hvh" {
		session.phase = "handoff"
		session.handoffTarget = 0
		session.handoffPhase = "commit"
		session.message = "Pass the screen to Player 1 to choose their Round 3 pile."
	}
}

func (s *webServer) finishGame() {
	session := s.session
	p1 := game.ScorePlayer(session.state.P1, session.state.Bounty.Suit, session.state.Modifiers)
	p2 := game.ScorePlayer(session.state.P2, session.state.Bounty.Suit, session.state.Modifiers)
	session.winner, session.tiebreak = finalWinner(p1, p2, s.rng)
	session.phase = "gameover"
	session.message = "The three rounds are complete."
	payout, payoutErr := awardWebMultiplayerPayout(
		s.playerProfile, session.variant, p1.Total, session.winner == 0,
		session.winner == -1, session.winner == 0, session.mode,
	)
	if payoutErr != nil {
		session.profileError = "Match reward calculation failed: " + payoutErr.Error()
	} else {
		session.payout = &payout
	}
	if s.playerProfile != nil {
		p := s.playerProfile
		won := session.winner == 0
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
		if p.HeroPlayCounts == nil {
			p.HeroPlayCounts = make(map[string]int)
		}
		if p.HeroWinCounts == nil {
			p.HeroWinCounts = make(map[string]int)
		}
		p.HeroPlayCounts[session.playerHeroID]++
		if session.winner == 0 {
			p.HeroWinCounts[session.playerHeroID]++
		}
		s.creditWebMatchAchievements(p, s.webMatchResult(p1.Total, p2.Total, won, p1.Total == p2.Total))
		mode := map[string]int{"hvh": economy.ModeIDFriendCasual, "easy": economy.ModeIDCPUEasy, "normal": economy.ModeIDCPUNormal, "hard": economy.ModeIDCPUHard}[session.mode]
		suits := map[string]int{}
		for _, card := range session.state.P1.Jail {
			suits[achievements.SuitRoleID(card.Suit)]++
		}
		p.RecordMatch(profile.MatchRecord{At: time.Now().UTC(), Mode: mode, PlayerSeat: session.state.P1.Seat.String(), PlayerScore: p1.Total, OppScore: p2.Total, Won: won, BountySuit: achievements.SuitRoleID(session.state.Bounty.Suit), PlayerPrisoners: len(session.state.P1.Jail), OppPrisoners: len(session.state.P2.Jail), PlayerPrisonerSuits: suits, Clashes: session.state.Clashes, MarksEarned: payout.TotalMarks})
		if err := s.playerProfile.Save(); err != nil {
			session.profileError = "Profile save failed: " + err.Error()
		}
	}
	session.achievementUpdates = s.matchAchievementUpdates()
}

func awardWebMultiplayerPayout(
	p *profile.Profile,
	variant string,
	score int,
	won, draw, dailyEligible bool,
	mode string,
) (webMatchPayout, error) {
	difficulty := ""
	if mode == "easy" || mode == "normal" || mode == "hard" {
		difficulty = mode
	}
	result, err := economy.CalculateMultiplayerPayout(
		variant, score, won, draw, dailyEligible, difficulty, multiplayerEconomyMode(mode), p, time.Now(),
	)
	if err != nil {
		return webMatchPayout{}, err
	}
	return webMatchPayout{
		PlayerScore: result.PlayerScore, PlayerWon: won, Draw: draw,
		BaseMarks: result.BaseMarks, WinMultiplier: 1,
		SkillBonusPct: result.MarksSkillPercent, AIMultiplier: result.AIMultiplier,
		TotalMarks: result.TotalMarks, MatchCrowns: result.MatchCrowns,
		DailyWinBonusCrowns: result.DailyWinBonusCrowns, TotalCrowns: result.TotalCrowns,
		FirstWinOfDay: result.FirstWinOfDay,
	}, nil
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
			Variant: "main", PuzzleOptions: puzzleOptions(),
		}
	}
	session := s.session
	variant := session.variant
	if variant == "" {
		variant = "main"
	}
	if variant == "tug-war" {
		if session.tugWar == nil {
			return webView{
				GameID: s.gameID, Started: true, Variant: variant, Phase: "gameover",
				Winner: -1, Message: "Tug War state is unavailable.",
			}
		}
		tugWarView := tugWarWebView(s, session)
		message := session.message
		if s.onlineViewer {
			switch {
			case session.phase == "gameover" && session.winner < 0:
				message = "Tug War ends in a draw."
			case session.phase == "gameover" && session.winner == s.viewerPlayer:
				message = "You win Tug War."
			case session.phase == "gameover":
				message = "Opponent wins Tug War."
			case session.actor == s.viewerPlayer:
				message = "Your turn. Choose a card, flip the deck, or spend a push."
			default:
				message = "Opponent's turn."
			}
		}
		return webView{
			GameID: s.gameID, Started: true, CanAct: tugWarView.CanAct, Dashboard: s.dashboard(), Mode: session.mode,
			Variant: variant, Phase: session.phase, Actor: session.actor, Winner: session.winner,
			Message: message, PlayerHero: heroName(session.playerHeroID),
			OpponentHero: heroName(session.opponentHeroID), PlayerHeroID: session.playerHeroID,
			OpponentHeroID: session.opponentHeroID, DefaultHeroID: session.playerHeroID,
			Heroes: s.heroOptions(), TugWar: tugWarView, Payout: session.payout,
			Campaign: session.campaign,
		}
	}
	if variant == "rummy" {
		return webView{
			GameID: s.gameID, Started: true, Dashboard: s.dashboard(), Mode: session.mode,
			Variant: variant, Phase: session.phase, Actor: session.actor, Winner: session.winner,
			Message: session.message, PlayerHero: heroName(session.playerHeroID),
			OpponentHero: heroName(session.opponentHeroID), PlayerHeroID: session.playerHeroID,
			OpponentHeroID: session.opponentHeroID, DefaultHeroID: session.playerHeroID,
			Heroes: s.heroOptions(), Rummy: session.rummy.webView(session, s.viewerPlayer, s.onlineViewer),
			Payout: session.payout, Campaign: session.campaign,
		}
	}
	if variant == "ladder" {
		return webView{
			GameID: s.gameID, Started: true, Dashboard: s.dashboard(), Mode: session.mode,
			Variant: variant, Phase: session.phase, Actor: session.actor, Winner: session.winner,
			Message: session.message, PlayerHero: heroName(session.playerHeroID),
			OpponentHero: heroName(session.opponentHeroID), PlayerHeroID: session.playerHeroID,
			OpponentHeroID: session.opponentHeroID, DefaultHeroID: session.playerHeroID,
			Heroes: s.heroOptions(), Ladder: ladderWebView(session, s.viewerPlayer, s.onlineViewer),
			Payout: session.payout, Campaign: session.campaign,
		}
	}
	if variant == "abandon-duel" {
		if session.abandonDuel == nil {
			return webView{Started: true, Variant: variant, Phase: "gameover", Winner: -1, Message: "Abandon Duel state is unavailable."}
		}
		canAct := !session.abandonDuel.GameOver &&
			((s.onlineViewer && session.abandonDuel.ActivePlayer == s.viewerPlayer) ||
				(!s.onlineViewer && session.abandonDuel.ActivePlayer == 0 &&
					!isAI(session.mode, session.abandonDuel.ActivePlayer)))
		duelView := session.abandonDuel.webView(
			session.phase, session.abandonDuel.ActivePlayer,
			canAct,
		)
		return webView{
			GameID: s.gameID, Started: true, CanAct: canAct,
			Dashboard: s.dashboard(), Mode: session.mode, Variant: variant,
			Phase: session.phase, Actor: session.actor, Winner: session.abandonDuel.Winner,
			Message: session.message, PlayerHero: heroName(session.playerHeroID),
			OpponentHero: heroName(session.opponentHeroID), PlayerHeroID: session.playerHeroID,
			OpponentHeroID: session.opponentHeroID, DefaultHeroID: session.playerHeroID,
			Heroes: s.heroOptions(), AbandonDuel: duelView, Payout: session.payout,
			Campaign: session.campaign,
		}
	}
	if variant == "poker-duel" {
		if session.pokerDuel == nil {
			return webView{Started: true, Variant: variant, Phase: "gameover", Winner: -1, Message: "Poker duel state is unavailable."}
		}
		now := time.Now()
		duelView := session.pokerDuel.webView(
			session.phase, now, func() int {
				if s.onlineViewer {
					return 1 - s.viewerPlayer
				}
				return pokerDuelCPUPlayer(session.mode)
			}(),
		)
		if !session.pokerDuelCPUReadyAt.IsZero() {
			remaining := session.pokerDuelCPUReadyAt.Sub(now)
			if remaining > 0 {
				duelView.CPUThinkRemainingMS = (remaining + time.Millisecond - 1).Milliseconds()
			}
		}
		return webView{
			GameID: s.gameID, Started: true, Dashboard: s.dashboard(), Mode: session.mode,
			Variant: variant, Phase: session.phase, Actor: session.actor, Winner: session.pokerDuel.Winner,
			Message: session.message, PlayerHero: heroName(session.playerHeroID),
			OpponentHero: heroName(session.opponentHeroID), PlayerHeroID: session.playerHeroID,
			OpponentHeroID: session.opponentHeroID, DefaultHeroID: session.playerHeroID,
			Heroes: s.heroOptions(), PokerDuel: duelView, Payout: session.payout,
		}
	}
	if variant == "poker" {
		return webView{
			GameID: s.gameID, Started: true, Dashboard: s.dashboard(), Mode: session.mode,
			Variant: variant, Phase: session.phase, Actor: session.actor, Winner: session.winner,
			Message: session.message, PlayerHero: heroName(session.playerHeroID),
			PlayerHeroID: session.playerHeroID, DefaultHeroID: session.playerHeroID,
			Heroes: s.heroOptions(), Poker: session.poker.webView(session.phase), Campaign: session.campaign,
		}
	}
	if variant == "puzzler" || variant == "abandon" {
		result := webView{
			GameID: s.gameID, Started: true, Dashboard: s.dashboard(), Mode: session.mode,
			Variant: variant, PuzzleOptions: puzzleOptions(), Phase: session.phase,
			Actor: session.actor, Winner: session.winner, Message: session.message,
			PlayerHero: heroName(session.playerHeroID), PlayerHeroID: session.playerHeroID,
			DefaultHeroID: session.playerHeroID, Heroes: s.heroOptions(),
		}
		if session.puzzle != nil {
			result.Puzzle = session.puzzle.webView()
			result.Modifiers = session.puzzle.Modifiers
			result.Bounty = game.Card{Suit: session.puzzle.Definition.Bounty, Face: 1}
		}
		if session.abandon != nil {
			result.Abandon = session.abandon.webView()
			result.Bounty = session.abandon.Bounty
			if hero, ok := heroes.ByID(session.playerHeroID); ok {
				result.Modifiers = heroModifiers(hero)
			}
		}
		result.Campaign = session.campaign
		return result
	}
	result := webView{
		GameID:         s.gameID,
		Started:        true,
		Dashboard:      s.dashboard(),
		Mode:           session.mode,
		Variant:        variant,
		PuzzleOptions:  puzzleOptions(),
		Phase:          session.phase,
		Round:          session.state.Round,
		DraftMode:      session.state.DraftMode,
		Exchange:       session.exchange + 1,
		Actor:          session.actor,
		Attacker:       session.attacker,
		Bounty:         session.state.Bounty,
		JailSlots:      make([]jailCell, len(session.slots)),
		Message:        session.message,
		CanContinue:    session.phase == "result",
		Winner:         session.winner,
		Payout:         session.payout,
		RankedSeries:   session.rankedSeries,
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
	s.fillDraftView(&result)
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
	result.Scores = scores
	result.Campaign = session.campaign
	result.Battles = append([]webBattleRecord(nil), session.battles...)
	if session.phase == "gameover" {
		result.AchievementUpdates = session.achievementUpdates
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
	if session.phase == "handoff" {
		result.Message = fmt.Sprintf("Pass the screen to Player %d.", session.handoffTarget+1)
	}
	if session.profileError != "" {
		result.Message += " " + session.profileError
	}
	result.ScoreLine = fmt.Sprintf("%s = %d  |  %s = %d",
		s.scoreLabel(session.whitePlayer), scores[session.whitePlayer].Total,
		s.scoreLabel(1-session.whitePlayer), scores[1-session.whitePlayer].Total)
	if variant == "fog" {
		redactFogView(&result, session)
	}
	return result
}

func redactFogView(view *webView, session *webSession) {
	if session.phase != "draft" {
		for index := range view.DraftPool {
			view.DraftPool[index] = hideWebCardFace(view.DraftPool[index])
		}
	}
	if session.actor == 1 {
		view.DraftHand = hideCardFaces(view.DraftHand)
		view.DraftPicked = hideCardFaces(view.DraftPicked)
	}
	for playerIndex := range view.Players {
		if playerIndex != 1 {
			continue
		}
		for cardIndex := range view.Players[playerIndex].Hand {
			view.Players[playerIndex].Hand[cardIndex] = hideWebCardFace(view.Players[playerIndex].Hand[cardIndex])
		}
		for cardIndex := range view.Players[playerIndex].Pool {
			view.Players[playerIndex].Pool[cardIndex] = hideWebCardFace(view.Players[playerIndex].Pool[cardIndex])
		}
	}
	for index := range view.JailSlots {
		cell := &view.JailSlots[index]
		capturedByPlayer := cell.CapturedBy == "White" && session.whitePlayer == 0 ||
			cell.CapturedBy == "Black" && session.whitePlayer == 1
		if cell.Card != nil && capturedByPlayer {
			card := hideWebFaceCard(*cell.Card)
			cell.Card = &card
		}
	}
	view.Scores[1].HighestCard = hiddenFaceRepresentative(view.Scores[1].HighestCard)
	if session.phase != "result" {
		if view.BattleAttack != nil && session.attacker == 1 {
			originalFace := view.BattleAttack.Face
			card := hideWebFaceCard(*view.BattleAttack)
			view.BattleAttack = &card
			view.BattleAttack.FaceRange = faceRange(originalFace)
		}
		if view.BattleDefend != nil && session.attacker == 0 {
			originalFace := view.BattleDefend.Face
			card := hideWebFaceCard(*view.BattleDefend)
			view.BattleDefend = &card
			view.BattleDefend.FaceRange = faceRange(originalFace)
		}
		if view.BattleOutcome != nil {
			view.BattleOutcome.HideValues = true
		}
	}
	for index := range view.Battles {
		record := &view.Battles[index]
		if record.AttackerIndex == 1 {
			record.AttackFaceHidden = true
			record.AttackFaceRange = faceRange(record.Attack.Face)
			record.Attack = hideWebFaceCard(record.Attack)
		} else {
			record.DefenseFaceHidden = true
			record.DefenseFaceRange = faceRange(record.Defense.Face)
			record.Defense = hideWebFaceCard(record.Defense)
		}
		record.Outcome.HideValues = true
	}
}

func hideCardFaces(cards []webFaceCard) []webFaceCard {
	masked := append([]webFaceCard(nil), cards...)
	for index, card := range cards {
		masked[index] = hideWebFaceCard(card)
	}
	return masked
}

func hideWebCardFace(card webCard) webCard {
	face := card.Card.Face
	card.Card = hideCardFace(card.Card)
	card.FaceHidden = true
	card.FaceRange = faceRange(face)
	return card
}

func hideWebFaceCard(card webFaceCard) webFaceCard {
	face := card.Face
	card.Card = hideCardFace(card.Card)
	card.FaceHidden = true
	card.FaceRange = faceRange(face)
	return card
}

func hideCardFace(card game.Card) game.Card {
	switch {
	case card.Face <= 3:
		card.Face = 1
	case card.Face <= 6:
		card.Face = 4
	default:
		card.Face = 7
	}
	return card
}

func hiddenFaceRepresentative(face int) int {
	switch {
	case face <= 0:
		return 0
	case face <= 3:
		return 1
	case face <= 6:
		return 4
	default:
		return 7
	}
}

func faceRange(face int) string {
	switch {
	case face <= 3:
		return "1-3"
	case face <= 6:
		return "4-6"
	default:
		return "7-9"
	}
}

func (s *webServer) fillDraftView(result *webView) {
	session := s.session
	if session.phase != "draft" || session.state.Draft == nil {
		return
	}
	draft := session.state.Draft
	result.DraftPool = make([]webCard, len(draft.Pool))
	for index, card := range draft.Pool {
		selectable := !isAI(session.mode, session.actor) &&
			(!s.onlineViewer || s.viewerPlayer == session.actor)
		result.DraftPool[index] = webCard{Card: card, Index: index, Selectable: selectable}
	}
	if s.onlineViewer && s.viewerPlayer != session.actor {
		return
	}
	seat, ok := draft.CurrentSeat()
	if ok {
		hand, picked, opponentCount := draftPrivateView(draft, seat)
		result.DraftHand = webFaceCards(hand)
		result.DraftPicked = webFaceCards(picked)
		result.DraftOpponentCount = opponentCount
		result.DraftPickNumber = draft.CurrentPick + 1
		result.DraftPickTotal = len(draft.PickOrder)
	}
}

func (s *webServer) playerView(index int, player game.Player, score int) webPlayer {
	session := s.session
	private := session.mode == "hvh" && index == session.actor && session.phase != "handoff" &&
		(!s.onlineViewer || index == s.viewerPlayer) || session.mode != "hvh" && index == 0
	view := webPlayer{Color: s.colorName(index), You: session.mode != "hvh" && index == 0, Seat: player.Seat.String(), Score: score, Hand: []webCard{}, Pool: []webCard{}, HiddenHand: len(player.Hand), HiddenPool: len(player.Pool)}
	if session.state.Draft != nil {
		if player.Seat == game.SeatX {
			view.HiddenHand = len(session.state.Draft.DealtX) + len(session.state.Draft.PickedX)
		} else {
			view.HiddenHand = len(session.state.Draft.DealtY) + len(session.state.Draft.PickedY)
		}
	}
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
	view.Dashboard = s.dashboardFor(p)
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
	return s.dashboardFor(p)
}

func (s *webServer) dashboardFor(p *profile.Profile) *webDashboard {
	dashboard := dashboardFor(p)
	if dashboard == nil {
		return nil
	}
	if s.session != nil && canUseWebMatchTokens(s.session, s.onlineViewer) {
		dashboard.MatchTokensAllowed = true
		dashboard.CanRestart = s.session.phase != "gameover" && s.session.restartSnapshot != nil
		dashboard.CanUndo = s.session.phase != "gameover" && s.session.undoSnapshot != nil
		dashboard.ReplaceOptions = s.replaceCardOptions()
		dashboard.CanSkipMission = s.session.campaign != nil &&
			!s.session.campaign.Replay &&
			!missionIsComplete(p.Campaigns[s.session.campaign.ID], s.session.campaign.Index)
	}
	return dashboard
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
	equipped := map[string]catalog.Chroma{}
	ownedChromas := make([]string, 0, len(p.OwnedChromas))
	ownedChromaDetails := make([]catalog.Chroma, 0, len(p.OwnedChromas))
	for _, chroma := range catalog.AllChromas() {
		if p.OwnedChromas[chroma.ID] {
			ownedChromas = append(ownedChromas, chroma.ID)
			ownedChromaDetails = append(ownedChromaDetails, chroma)
		}
	}
	for _, suit := range []string{"B", "F", "S"} {
		if chroma, ok := catalog.FindChroma(p.AssignedChromas[suit]); ok && p.OwnedChromas[chroma.ID] {
			equipped[suit] = chroma
		} else {
			equipped[suit] = catalog.Chroma{Color: p.AssignedChromas[suit], Name: "Default suit color"}
		}
	}
	return &webDashboard{
		DisplayName:           p.DisplayName,
		EquippedChromas:       equipped,
		OwnedChromas:          ownedChromas,
		OwnedChromaDetails:    ownedChromaDetails,
		Commander:             commander,
		Title:                 heroName(p.DefaultHeroID),
		DefaultHeroID:         p.DefaultHeroID,
		Marks:                 p.Marks,
		Crowns:                p.Crowns,
		Energy:                currentEnergy,
		MaxEnergy:             profile.EnergyStorageCap(p),
		EnergyNextSeconds:     nextSeconds,
		EnergyRechargeSeconds: int((p.EnergyRechargeMs + 999) / 1000),
		AdsLeft:               adsLeft,
		DailyFeaturedItem:     dailyFeaturedShopItem(p, now),
		LoginStreakCurrent:    p.LoginStreakCurrent,
		LoginStreakBest:       p.LoginStreakBest,
		WinsSinceCampDraw:     p.WinsSinceCampDraw,
		WinsPerCampDraw:       lootbox.RequiredWins(p),
		DrawsAvailable:        p.DrawsAvailable,
		RestartTokens:         p.RestartMissionTokens,
		UndoTokens:            p.UndoMoveTokens,
		CampaignSkipTokens:    p.CampaignSkipTokens,
		ReplaceTokens:         p.ReplaceCardTokens,
		PendingBoxes:          p.TotalPendingBoxes(),
		SkillPoints:           p.SkillPoints,
		ConductorLevel:        skills.AggregateBonuses(p).ConductorLevel,
		UnseenAchievements:    len(unseenAchievementIDs(p)),
		GamesPlayed:           p.TotalGames,
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
		ID: item.ID, Name: item.Name, Color: item.Color, Skin: item.Skin, Rarity: item.Rarity,
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
		options = append(options, heroOption(hero, p != nil && containsHeroID(p.UnlockedHeroes, hero.ID)))
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
	return mode == "easy" || mode == "normal" || mode == "hard"
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

func cardPointer(card game.Card) *webFaceCard {
	return &webFaceCard{Card: card}
}

func webFaceCards(cards []game.Card) []webFaceCard {
	result := make([]webFaceCard, len(cards))
	for index, card := range cards {
		result[index] = webFaceCard{Card: card}
	}
	return result
}
