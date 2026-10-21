package profile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const SchemaVersion = 4

const (
	DefaultMaxEnergy      = 10
	MaxEnergyCap          = 20
	DefaultEnergyRecharge = 10 * time.Minute
)

type Profile struct {
	SchemaVersion             int               `json:"schema_version"`
	PlayerID                  string            `json:"player_id"`
	CreatedAt                 time.Time         `json:"created_at"`
	UpdatedAt                 time.Time         `json:"updated_at"`
	Marks                     int64             `json:"marks"`
	Crowns                    int64             `json:"crowns"`
	Energy                    int               `json:"energy"`
	MaxEnergy                 int               `json:"max_energy"`
	EnergyUpdatedAt           time.Time         `json:"energy_updated_at"`
	EnergyRechargeMs          int64             `json:"energy_recharge_ms"`
	DrawsAvailable            int               `json:"camp_draws_available"`
	WinsSinceCampDraw         int               `json:"camp_draw_wins"`
	PendingBoxes              map[string]int    `json:"pending_boxes"`
	TotalWins                 int               `json:"total_wins"`
	TotalLosses               int               `json:"total_losses"`
	TotalDraws                int               `json:"total_draws"`
	TotalGames                int               `json:"total_games"`
	CurrentStreak             int               `json:"current_streak"`
	BestStreak                int               `json:"best_streak"`
	DailyWins                 int               `json:"daily_wins"`
	DailyWinResetAt           time.Time         `json:"daily_win_reset_at"`
	AdsWatchedToday           int               `json:"ads_watched_today"`
	AdsResetAt                time.Time         `json:"ads_reset_at"`
	DailyFreeRefillUsed       bool              `json:"daily_free_refill_used"`
	FreeRefillResetAt         time.Time         `json:"free_refill_reset_at"`
	SkillNodes                map[string]bool   `json:"skill_nodes"`
	OwnedChromas              map[string]bool   `json:"owned_chromas"`
	AssignedChromas           map[string]string `json:"assigned_chromas"`
	PurchasedBundles          map[string]bool   `json:"purchased_bundles"`
	SeenChromas               map[string]bool   `json:"seen_chromas"`
	MatchHistory              []MatchRecord     `json:"match_history"`
	Achievements              map[string]bool   `json:"achievements"`
	AchievementTiers          map[string]int    `json:"achievement_tiers"`
	AchievementCounters       map[string]int    `json:"achievement_counters"`
	CompletedSets             map[string]bool   `json:"completed_sets"`
	SeenAchievementIDs        []string          `json:"seen_achievement_ids"`
	SkillPoints               int               `json:"skill_points"`
	ChromaPickTokens          int               `json:"chroma_pick_tokens"`
	Badges                    []string          `json:"badges"`
	ColorOn                   bool              `json:"color_on"`
	ConfirmQuit               bool              `json:"confirm_quit"`
	LoginStreakCurrent        int               `json:"login_streak_current"`
	LoginStreakBest           int               `json:"login_streak_best"`
	LastLoginDate             time.Time         `json:"last_login_date"`
	WinStreakByMode           map[int]int       `json:"win_streak_by_mode"`
	BestWinStreakByMode       map[int]int       `json:"best_win_streak_by_mode"`
	WinsAsSeatX               int               `json:"wins_as_seat_x"`
	WinsAsSeatY               int               `json:"wins_as_seat_y"`
	ComebackWins              int               `json:"comeback_wins"`
	TotalClashes              int               `json:"total_clashes"`
	MaxClashesInGame          int               `json:"max_clashes_in_game"`
	TotalMatchTies            int               `json:"total_match_ties"`
	TotalChromasPurchased     int               `json:"total_chromas_purchased"`
	RareChromasPurchased      int               `json:"rare_chromas_purchased"`
	SuperRareChromasPurchased int               `json:"super_rare_chromas_purchased"`
	HardCPUWins               int               `json:"hard_cpu_wins"`
	SuitDoublingWins          int               `json:"suit_doubling_wins"`
}

func New(now time.Time) (*Profile, error) {
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return nil, fmt.Errorf("generate player ID: %w", err)
	}
	now = now.UTC()
	return &Profile{
		SchemaVersion:       SchemaVersion,
		PlayerID:            hex.EncodeToString(idBytes[:]),
		CreatedAt:           now,
		UpdatedAt:           now,
		Energy:              DefaultMaxEnergy,
		MaxEnergy:           DefaultMaxEnergy,
		EnergyUpdatedAt:     now,
		EnergyRechargeMs:    DefaultEnergyRecharge.Milliseconds(),
		DailyWinResetAt:     nextLocalMidnight(now),
		AdsResetAt:          nextLocalMidnight(now),
		FreeRefillResetAt:   nextLocalMidnight(now),
		SkillNodes:          make(map[string]bool),
		OwnedChromas:        make(map[string]bool),
		AssignedChromas:     map[string]string{"B": "red", "F": "yellow", "S": "blue"},
		PurchasedBundles:    make(map[string]bool),
		SeenChromas:         make(map[string]bool),
		PendingBoxes:        make(map[string]int),
		MatchHistory:        make([]MatchRecord, 0),
		Achievements:        make(map[string]bool),
		AchievementTiers:    make(map[string]int),
		AchievementCounters: make(map[string]int),
		CompletedSets:       make(map[string]bool),
		SeenAchievementIDs:  make([]string, 0),
		ColorOn:             true,
		ConfirmQuit:         true,
		WinStreakByMode:     make(map[int]int),
		BestWinStreakByMode: make(map[int]int),
	}, nil
}

func ProfileFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".prisoners-of-war", "profile.json"), nil
}

func LoadProfile() (*Profile, error) {
	path, err := ProfileFilePath()
	if err != nil {
		return nil, err
	}
	return LoadProfileFrom(path)
}

func LoadProfileFrom(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		p, createErr := New(time.Now())
		if createErr != nil {
			return nil, createErr
		}
		if saveErr := p.SaveTo(path); saveErr != nil {
			return nil, saveErr
		}
		return p, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	if p.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("profile schema %d is newer than supported schema %d", p.SchemaVersion, SchemaVersion)
	}
	if p.SchemaVersion < 1 {
		return nil, fmt.Errorf("unsupported profile schema %d", p.SchemaVersion)
	}
	if p.PlayerID == "" {
		return nil, errors.New("profile is missing player_id")
	}
	previousSchema := p.SchemaVersion
	migrated := previousSchema < SchemaVersion
	normalize(&p)
	if previousSchema < 3 {
		p.ColorOn = true
		p.ConfirmQuit = true
	}
	p.ApplyEnergyTick(time.Now())
	if migrated {
		p.SchemaVersion = SchemaVersion
		if err := p.SaveTo(path); err != nil {
			return nil, fmt.Errorf("save migrated profile: %w", err)
		}
	}
	return &p, nil
}

func (p *Profile) Save() error {
	path, err := ProfileFilePath()
	if err != nil {
		return err
	}
	return p.SaveTo(path)
}

func (p *Profile) SaveTo(path string) error {
	if p == nil {
		return errors.New("cannot save a nil profile")
	}
	p.SchemaVersion = SchemaVersion
	p.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}
	file, err := os.CreateTemp(dir, ".profile-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary profile: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("secure temporary profile: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write profile: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync profile: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close profile: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace profile: %w", err)
	}
	return nil
}

func (p *Profile) ApplyEnergyTick(now time.Time) {
	if p == nil {
		return
	}
	if p.MaxEnergy <= 0 {
		p.MaxEnergy = DefaultMaxEnergy
	}
	if p.MaxEnergy > MaxEnergyCap {
		p.MaxEnergy = MaxEnergyCap
	}
	if p.EnergyRechargeMs <= 0 {
		p.EnergyRechargeMs = DefaultEnergyRecharge.Milliseconds()
	}
	if p.Energy < 0 {
		p.Energy = 0
	}
	if p.Energy > p.MaxEnergy {
		p.Energy = p.MaxEnergy
	}
	if p.EnergyUpdatedAt.IsZero() {
		p.EnergyUpdatedAt = now.UTC()
		return
	}
	elapsed := now.Sub(p.EnergyUpdatedAt)
	if elapsed <= 0 {
		return
	}
	recharge := time.Duration(p.EnergyRechargeMs) * time.Millisecond
	if recharge <= 0 {
		return
	}
	gained := int(elapsed / recharge)
	if gained > 0 {
		p.Energy += gained
		p.EnergyUpdatedAt = p.EnergyUpdatedAt.Add(time.Duration(gained) * recharge)
		if p.Energy >= p.MaxEnergy {
			p.Energy = p.MaxEnergy
			p.EnergyUpdatedAt = now.UTC()
		}
	}
}

func normalize(p *Profile) {
	if p.MaxEnergy <= 0 {
		p.MaxEnergy = DefaultMaxEnergy
	}
	if p.MaxEnergy > MaxEnergyCap {
		p.MaxEnergy = MaxEnergyCap
	}
	if p.EnergyRechargeMs <= 0 {
		p.EnergyRechargeMs = DefaultEnergyRecharge.Milliseconds()
	}
	if p.SkillNodes == nil {
		p.SkillNodes = make(map[string]bool)
	}
	if p.OwnedChromas == nil {
		p.OwnedChromas = make(map[string]bool)
	}
	if p.AssignedChromas == nil {
		p.AssignedChromas = make(map[string]string)
	}
	if p.PurchasedBundles == nil {
		p.PurchasedBundles = make(map[string]bool)
	}
	if p.SeenChromas == nil {
		p.SeenChromas = make(map[string]bool)
	}
	if p.MatchHistory == nil {
		p.MatchHistory = make([]MatchRecord, 0)
	}
	if p.Achievements == nil {
		p.Achievements = make(map[string]bool)
	}
	if p.AchievementTiers == nil {
		p.AchievementTiers = make(map[string]int)
	}
	if p.AchievementCounters == nil {
		p.AchievementCounters = make(map[string]int)
	}
	if p.CompletedSets == nil {
		p.CompletedSets = make(map[string]bool)
	}
	if p.SeenAchievementIDs == nil {
		p.SeenAchievementIDs = make([]string, 0)
	}
	if p.PendingBoxes == nil {
		p.PendingBoxes = make(map[string]int)
	}
	if p.WinStreakByMode == nil {
		p.WinStreakByMode = make(map[int]int)
	}
	if p.BestWinStreakByMode == nil {
		p.BestWinStreakByMode = make(map[int]int)
	}
	if p.Badges == nil {
		p.Badges = make([]string, 0)
	}
	for suit, color := range map[string]string{"B": "red", "F": "yellow", "S": "blue"} {
		if p.AssignedChromas[suit] == "" {
			p.AssignedChromas[suit] = color
		}
	}
}

func nextLocalMidnight(now time.Time) time.Time {
	local := now.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, time.Local)
}
