package economy

import (
	"errors"
	"fmt"
	"math"
	"time"

	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
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
	PlayerWon               bool    `json:"player_won"`
	Draw                    bool    `json:"draw"`
	BaseMarks               int64   `json:"base_marks"`
	ScoreBonusMarks         int64   `json:"score_bonus_marks"`
	ModeMultiplier          float64 `json:"mode_multiplier"`
	SkillBonusPercent       int     `json:"skill_bonus_percent"`
	LossSkillPercent        int     `json:"loss_skill_percent"`
	FirstWinBonus           bool    `json:"first_win_bonus"`
	SecondWinBonus          bool    `json:"second_win_bonus"`
	DailyModeBonusMarks     int64   `json:"daily_mode_bonus_marks"`
	TotalMarks              int64   `json:"total_marks"`
	CrownsAwarded           int64   `json:"crowns_awarded"`
	RankedStreakBonusCrowns int64   `json:"ranked_streak_bonus_crowns"`
	CampDrawAwarded         bool    `json:"camp_draw_awarded"`
}

type MultiplayerPayout struct {
	Variant             string  `json:"variant"`
	PlayerScore         int     `json:"playerScore"`
	Won                 bool    `json:"won"`
	Draw                bool    `json:"draw"`
	BaseMarks           int64   `json:"baseMarks"`
	MarksSkillPercent   int     `json:"marksSkillPercent"`
	LossSkillPercent    int     `json:"lossSkillPercent"`
	AIMultiplier        float64 `json:"aiMultiplier"`
	TotalMarks          int64   `json:"totalMarks"`
	MatchCrowns         int64   `json:"matchCrowns"`
	DailyWinBonusCrowns int64   `json:"dailyWinBonusCrowns"`
	RankedStreakCrowns  int64   `json:"rankedStreakCrowns"`
	DailyModeBonusMarks int64   `json:"dailyModeBonusMarks"`
	TotalCrowns         int64   `json:"totalCrowns"`
	FirstWinOfDay       bool    `json:"firstWinOfDay"`
}

func CalculateMultiplayerPayout(
	variant string,
	score int,
	won, draw, dailyEligible bool,
	aiDifficulty string,
	mode Mode,
	p *profile.Profile,
	now time.Time,
) (MultiplayerPayout, error) {
	if p == nil {
		return MultiplayerPayout{}, errors.New("profile is required")
	}
	if score < 0 {
		score = 0
	}
	payout := MultiplayerPayout{Variant: variant, PlayerScore: score, Won: won, Draw: draw, AIMultiplier: 1}
	switch variant {
	case "main", "fog", "rummy":
		if won || draw {
			payout.BaseMarks, payout.MatchCrowns = int64(300+3*score), 4
		} else {
			payout.BaseMarks = int64(100 + score)
		}
	case "puzzle-battle":
		if won || draw {
			payout.BaseMarks, payout.MatchCrowns = int64(75+3*score), 2
		} else {
			payout.BaseMarks = int64(25 + score)
		}
	case "poker-duel":
		if won || draw {
			payout.BaseMarks, payout.MatchCrowns = int64(150+score), 3
		} else {
			payout.BaseMarks = int64(50 + score/3)
		}
	case "ladder", "tug-war", "abandon-duel":
		if won || draw {
			payout.BaseMarks, payout.MatchCrowns = 300, 3
		} else {
			payout.BaseMarks = 100
		}
	default:
		return MultiplayerPayout{}, fmt.Errorf("unknown multiplayer variant %q", variant)
	}
	switch aiDifficulty {
	case "":
	case "easy":
		payout.AIMultiplier = 0.5
	case "normal":
		payout.AIMultiplier = 0.75
	case "hard":
		payout.AIMultiplier = 0.9
	default:
		return MultiplayerPayout{}, fmt.Errorf("unknown AI difficulty %q", aiDifficulty)
	}
	crownsMultiplier := payout.AIMultiplier
	resetDailyWins(p, now)
	bonuses := skills.AggregateBonuses(p)
	if mode == "" {
		switch aiDifficulty {
		case "easy":
			mode = ModeCPUEasy
		case "normal":
			mode = ModeCPUNormal
		case "hard":
			mode = ModeCPUHard
		default:
			mode = ModeFriendCasual
		}
	}
	if aiDifficulty == "" {
		payout.AIMultiplier = modeMultiplier(mode)
	}
	payout.MarksSkillPercent = bonuses.MarksBasePercent
	payout.LossSkillPercent = bonuses.LossMarksPercent
	if (mode == ModeRanked || mode == ModeFriendRanked) && bonuses.RankedPayoutPercent > 0 {
		payout.AIMultiplier += float64(bonuses.RankedPayoutPercent) / 100
	}
	marks := float64(payout.BaseMarks) * (1 + float64(bonuses.MarksBasePercent)/100)
	payout.MatchCrowns = int64(math.Floor(float64(payout.MatchCrowns) * crownsMultiplier))
	if dailyEligible && won {
		p.DailyWins++
		if p.DailyWins == 1 && bonuses.FirstWinBonus > 0 {
			marks *= 1 + float64(bonuses.FirstWinBonus)/100
		} else if p.DailyWins == 2 && bonuses.SecondWinBonus > 0 {
			marks *= 1 + float64(bonuses.SecondWinBonus)/100
		}
		p.DailyMultiplayerWins++
		if p.DailyMultiplayerWins == 1 {
			payout.FirstWinOfDay = true
			payout.DailyWinBonusCrowns = int64(15 + bonuses.DailyWinCrowns)
		}
	}
	if won {
		marks *= 1 + float64(bonuses.MultiplayerMarksPercent)/100
	} else if !draw {
		marks *= 1 + float64(bonuses.LossMarksPercent)/100
	}
	payout.TotalMarks = int64(math.Floor(marks * payout.AIMultiplier))
	if dailyEligible && won {
		payout.DailyModeBonusMarks = dailyModeWinBonus(p, "multiplayer:"+variant, bonuses.FirstModeWinMarks)
		payout.TotalMarks += payout.DailyModeBonusMarks
		if (mode == ModeRanked || mode == ModeFriendRanked) && bonuses.RankedBestOfThreeMarks > 0 {
			payout.TotalMarks += int64(bonuses.RankedBestOfThreeMarks)
		}
	}
	payout.RankedStreakCrowns = updateRankedWinStreak(
		p, mode == ModeRanked || mode == ModeFriendRanked, won, draw, bonuses.RankedStreakCrowns,
	)
	payout.TotalCrowns = payout.MatchCrowns + payout.DailyWinBonusCrowns
	payout.TotalCrowns += payout.RankedStreakCrowns
	p.Marks += payout.TotalMarks
	p.Crowns += payout.TotalCrowns
	return payout, nil
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
	if (mode == ModeRanked || mode == ModeFriendRanked) && bonuses.RankedPayoutPercent > 0 {
		payoutMultiplier += float64(bonuses.RankedPayoutPercent) / 100
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
	if payout.PlayerWon && bonuses.SecondWinBonus > 0 && p.DailyWins == 2 {
		payout.SecondWinBonus = true
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
	total := base + float64(payout.ScoreBonusMarks)
	if payout.FirstWinBonus {
		total *= 1 + float64(bonuses.FirstWinBonus)/100
	}
	if payout.SecondWinBonus {
		total *= 1 + float64(bonuses.SecondWinBonus)/100
	}
	if !payout.PlayerWon && !payout.Draw && bonuses.LossMarksPercent > 0 {
		payout.LossSkillPercent = bonuses.LossMarksPercent
		total *= 1 + float64(bonuses.LossMarksPercent)/100
	}
	payout.TotalMarks = int64(math.Floor(total * payout.ModeMultiplier))
	if payout.PlayerWon {
		payout.DailyModeBonusMarks = dailyModeWinBonus(p, string(mode), bonuses.FirstModeWinMarks)
		payout.TotalMarks += payout.DailyModeBonusMarks
	}
	if mode == ModeRanked || mode == ModeFriendRanked {
		payout.RankedStreakBonusCrowns = updateRankedWinStreak(
			p, true, payout.PlayerWon, payout.Draw, bonuses.RankedStreakCrowns,
		)
		if payout.PlayerWon && bonuses.RankedBestOfThreeMarks > 0 {
			payout.TotalMarks += int64(bonuses.RankedBestOfThreeMarks)
		}
	}
	payout.CrownsAwarded += payout.RankedStreakBonusCrowns
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
	p.ResetDailyProgress(now)
}

func dailyModeWinBonus(p *profile.Profile, mode string, bonus int) int64 {
	if p.DailyModeWins == nil {
		p.DailyModeWins = make(map[string]bool)
	}
	if p.DailyModeWins[mode] {
		return 0
	}
	p.DailyModeWins[mode] = true
	return int64(bonus)
}

func updateRankedWinStreak(p *profile.Profile, ranked, won, draw bool, bonus int) int64 {
	if !ranked {
		return 0
	}
	if won {
		p.RankedWinStreak++
		if bonus > 0 && p.RankedWinStreak%3 == 0 {
			return int64(bonus)
		}
	} else if !draw {
		p.RankedWinStreak = 0
	}
	return 0
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
