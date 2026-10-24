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
	web.playerProfile.TotalGames = 37
	count, goal, unit := achievementCounter(web.playerProfile, "long_march")
	if count != 37 || goal != 100 || unit != "matches played" {
		t.Fatalf("unexpected progress: %d/%d %s", count, goal, unit)
	}
}
