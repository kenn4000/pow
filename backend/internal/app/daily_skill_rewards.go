package app

import (
	"math"
	"time"

	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

func applyCampaignWinSkillRewards(p *profile.Profile, campaignID string, marks int64, now time.Time) (int64, bool, error) {
	p.ResetDailyProgress(now)
	if p.DailyModeWins == nil {
		p.DailyModeWins = make(map[string]bool)
	}
	bonuses := skills.AggregateBonuses(p)
	p.DailyWins++
	markPercent := 0
	if p.DailyWins == 1 {
		markPercent = bonuses.FirstWinBonus
	} else if p.DailyWins == 2 {
		markPercent = bonuses.SecondWinBonus
	}
	extraMarks := int64(math.Floor(float64(marks) * float64(markPercent) / 100))
	modeKey := "campaign:" + campaignID
	if !p.DailyModeWins[modeKey] {
		p.DailyModeWins[modeKey] = true
		extraMarks += int64(bonuses.FirstModeWinMarks)
	}
	p.Marks += extraMarks

	p.DailyCampaignWins++
	if p.DailyCampaignWins == 1 && bonuses.RationsReadiness {
		p.RestartMissionTokens++
		p.UndoMoveTokens++
		if err := energy.Grant(p, 1, now); err != nil {
			return 0, false, err
		}
		return extraMarks, true, nil
	}
	return extraMarks, false, nil
}
