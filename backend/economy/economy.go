package economy

import (
	"errors"
	"fmt"
	"math"
	"time"

	"prisonersofwar/lootbox"
	"prisonersofwar/profile"
	"prisonersofwar/skills"
)

type Mode string

const (
	ModeCPUEasy      Mode = "cpu_easy"
	ModeCPUNormal    Mode = "cpu_normal"
	ModeCPUHard      Mode = "cpu_hard"
	ModeRanked       Mode = "ranked"
	ModeFriendCasual Mode = "friend_casual"
	ModeFriendRanked Mode = "friend_ranked"
)

const (
	ModeIDCPUEasy = iota
	ModeIDCPUNormal
	ModeIDCPUHard
	ModeIDRanked
	ModeIDFriendCasual
	ModeIDFriendRanked
)

func ModeID(mode Mode) int {
	switch mode {
	case ModeCPUEasy:
		return ModeIDCPUEasy
	case ModeCPUNormal:
		return ModeIDCPUNormal
	case ModeCPUHard:
		return ModeIDCPUHard
	case ModeRanked:
		return ModeIDRanked
	case ModeFriendCasual:
		return ModeIDFriendCasual
	case ModeFriendRanked:
		return ModeIDFriendRanked
	default:
		return -1
	}
}

const (
	WinBaseMarks           int64 = 100
	LossBaseMarks          int64 = 40
	DrawBaseMarks          int64 = 60
	CrownsPerWin           int64 = 1
	RankedWinCrownsBonus   int64 = 1
	WinScoreBonusPerPoint  int64 = 2
	LossScoreBonusPerPoint int64 = 1
	DrawScoreBonusPerPoint int64 = 1
	DailyScoreBonusWinCap        = 10
	RankedMultiplier             = 1.5
	FriendRankedMultiplier       = 1.25
	NormalCPUMultiplier          = 1.05
	HardCPUMultiplier            = 1.1
)

type MatchResult struct {
	WinnerSeat    int `json:"winner_seat"` // SeatX is 0 and SeatY is 1; -1 represents a draw.
	PlayerSeat    int `json:"player_seat"`
	PlayerScore   int `json:"player_score"`
	OpponentScore int `json:"opponent_score"`
}

type Payout struct {
	PlayerWon         bool    `json:"player_won"`
	Draw              bool    `json:"draw"`
	BaseMarks         int64   `json:"base_marks"`
	ScoreBonusMarks   int64   `json:"score_bonus_marks"`
	ModeMultiplier    float64 `json:"mode_multiplier"`
	SkillBonusPercent int     `json:"skill_bonus_percent"`
	FirstWinBonus     bool    `json:"first_win_bonus"`
	TotalMarks        int64   `json:"total_marks"`
	CrownsAwarded     int64   `json:"crowns_awarded"`
	CampDrawAwarded   bool    `json:"camp_draw_awarded"`
}

func CalculatePayout(result MatchResult, mode Mode, p *profile.Profile, playerSeat int, now time.Time) (Payout, error) {
	if err := validatePayout(result, mode, p, playerSeat); err != nil {
		return Payout{}, err
	}
	return calculateValidatedPayout(result, mode, p, playerSeat, now), nil
}

func validatePayout(result MatchResult, mode Mode, p *profile.Profile, playerSeat int) error {
	if p == nil {
		return errors.New("profile is required")
	}
	if playerSeat != 0 && playerSeat != 1 {
		return fmt.Errorf("invalid player seat %d", playerSeat)
	}
	if result.WinnerSeat != -1 && result.WinnerSeat != 0 && result.WinnerSeat != 1 {
		return fmt.Errorf("invalid winner seat %d", result.WinnerSeat)
	}
	if result.PlayerSeat != 0 && result.PlayerSeat != 1 {
		return fmt.Errorf("invalid player seat %d", result.PlayerSeat)
	}
	if result.PlayerSeat != playerSeat {
		return fmt.Errorf("match result player seat %d does not match payout seat %d", result.PlayerSeat, playerSeat)
	}
	if result.PlayerScore < 0 || result.OpponentScore < 0 {
		return errors.New("scores cannot be negative")
	}
	if !validMode(mode) {
		return fmt.Errorf("invalid payout mode %q", mode)
	}
	return nil
}

func calculateValidatedPayout(result MatchResult, mode Mode, p *profile.Profile, playerSeat int, now time.Time) Payout {
	resetDailyWins(p, now)
	bonuses := skills.AggregateBonuses(p)
	payoutMultiplier := modeMultiplier(mode)
	if (mode == ModeRanked || mode == ModeFriendRanked) && bonuses.ModeMultiplier > 0 {
		payoutMultiplier += float64(bonuses.ModeMultiplier) / 100
	}
	payout := Payout{
		PlayerWon:      result.WinnerSeat >= 0 && result.WinnerSeat == playerSeat,
		Draw:           result.WinnerSeat == -1,
		ModeMultiplier: payoutMultiplier,
	}
	switch {
	case payout.PlayerWon:
		payout.BaseMarks = WinBaseMarks
		payout.CrownsAwarded = CrownsPerWin
		if mode == ModeRanked || mode == ModeFriendRanked {
			payout.CrownsAwarded += RankedWinCrownsBonus
		}
		p.DailyWins++
	case payout.Draw:
		payout.BaseMarks = DrawBaseMarks
	default:
		payout.BaseMarks = LossBaseMarks
	}
	payout.SkillBonusPercent = bonuses.MarksBasePercent
	if payout.PlayerWon && bonuses.FirstWinBonus > 0 && p.DailyWins == 1 {
		payout.FirstWinBonus = true
	}
	if payout.PlayerWon && scoreBonusMode(mode) && p.DailyWins > DailyScoreBonusWinCap {
		payout.ScoreBonusMarks = 0
	} else {
		multiplier := WinScoreBonusPerPoint
		if payout.Draw {
			multiplier = DrawScoreBonusPerPoint
		} else if !payout.PlayerWon {
			multiplier = LossScoreBonusPerPoint
		}
		payout.ScoreBonusMarks = int64(result.PlayerScore) * multiplier
	}
	base := float64(payout.BaseMarks) * (1 + float64(bonuses.MarksBasePercent)/100)
	if payout.FirstWinBonus {
		base *= 1 + float64(bonuses.FirstWinBonus)/100
	}
	payout.TotalMarks = int64(math.Floor((base + float64(payout.ScoreBonusMarks)) * payout.ModeMultiplier))
	p.Marks += payout.TotalMarks
	p.Crowns += payout.CrownsAwarded
	applyPayoutProgress(p, &payout)
	return payout
}

func applyPayoutProgress(p *profile.Profile, payout *Payout) {
	if payout.PlayerWon {
		p.TotalWins++
		p.CurrentStreak++
		if p.CurrentStreak > p.BestStreak {
			p.BestStreak = p.CurrentStreak
		}
		p.WinsSinceCampDraw++
		threshold := lootbox.RequiredWins(p)
		if p.WinsSinceCampDraw >= threshold {
			if p.DrawsAvailable < lootbox.MaxDrawsStored {
				p.DrawsAvailable++
				payout.CampDrawAwarded = true
			}
			p.WinsSinceCampDraw = 0
		}
	} else if payout.Draw {
		p.TotalDraws++
	} else {
		p.TotalLosses++
		p.CurrentStreak = 0
	}
}

func modeMultiplier(mode Mode) float64 {
	switch mode {
	case ModeRanked:
		return RankedMultiplier
	case ModeFriendRanked:
		return FriendRankedMultiplier
	case ModeCPUNormal:
		return NormalCPUMultiplier
	case ModeCPUHard:
		return HardCPUMultiplier
	default:
		return 1
	}
}

func validMode(mode Mode) bool {
	switch mode {
	case ModeCPUEasy, ModeCPUNormal, ModeCPUHard, ModeRanked, ModeFriendCasual, ModeFriendRanked:
		return true
	default:
		return false
	}
}

func scoreBonusMode(mode Mode) bool {
	switch mode {
	case ModeCPUEasy, ModeCPUNormal, ModeCPUHard, ModeRanked, ModeFriendCasual, ModeFriendRanked:
		return true
	default:
		return false
	}
}

func resetDailyWins(p *profile.Profile, now time.Time) {
	localNow := now.In(time.Local)
	if p.DailyWinResetAt.IsZero() || !localNow.Before(p.DailyWinResetAt.In(time.Local)) {
		p.DailyWins = 0
		p.DailyWinResetAt = time.Date(localNow.Year(), localNow.Month(), localNow.Day()+1, 0, 0, 0, 0, localNow.Location())
	}
}

func AuditConfig() map[string]any {
	return map[string]any{
		"base_marks":                  map[string]int64{"win": WinBaseMarks, "loss": LossBaseMarks, "draw": DrawBaseMarks},
		"crowns_per_win":              CrownsPerWin,
		"ranked_win_crowns_bonus":     RankedWinCrownsBonus,
		"score_bonus_marks_per_point": map[string]int64{"win": WinScoreBonusPerPoint, "loss": LossScoreBonusPerPoint, "draw": DrawScoreBonusPerPoint},
		"daily_score_bonus_win_cap":   DailyScoreBonusWinCap,
		"mode_multipliers": map[string]float64{
			string(ModeCPUEasy):      1,
			string(ModeCPUNormal):    NormalCPUMultiplier,
			string(ModeCPUHard):      HardCPUMultiplier,
			string(ModeRanked):       RankedMultiplier,
			string(ModeFriendCasual): 1,
			string(ModeFriendRanked): FriendRankedMultiplier,
		},
		"win_camp_draw_threshold": lootbox.WinsPerCampDraw,
		"max_camp_draws_stored":   lootbox.MaxDrawsStored,
	}
}
