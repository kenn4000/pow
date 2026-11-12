package app

import (
	"math"
	"testing"
)

func TestDisplayedBoxOddsCoverAllFirstRollRewards(t *testing.T) {
	web := campaignTestServer(t)
	for _, upgraded := range []bool{false, true} {
		web.playerProfile.SkillNodes["fortune"] = upgraded
		for box, rewards := range boxRewardTables(web.playerProfile) {
			total := 0.0
			for _, reward := range rewards {
				if reward.Label == "" || reward.Chance <= 0 {
					t.Fatalf("invalid %s reward: %+v", box, reward)
				}
				total += reward.Chance
			}
			if math.Abs(total-100) > .000001 {
				t.Fatalf("%s first-roll odds sum to %f", box, total)
			}
		}
	}
}

func TestAchievementProgressUsesActualCountsAndUnits(t *testing.T) {
	web := campaignTestServer(t)
	web.playerProfile.TotalWins = 37
	page := buildAchievementsPage(web.playerProfile)
	for _, item := range page.Items {
		if item.ID == "long_march" {
			if item.Progress != 37 || item.Goal != 50 || item.ProgressUnit != "wins" {
				t.Fatalf("unexpected progress: %d/%d %s", item.Progress, item.Goal, item.ProgressUnit)
			}
			return
		}
	}
	t.Fatal("Long March achievement was not included in the gallery")
}
