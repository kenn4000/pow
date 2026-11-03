package campaigns

import (
	"fmt"

	"prisonersofwar/internal/game"
)

func pokerCampaign() Campaign {
	missionConfigs := []struct {
		name  string
		hint  string
		rules game.PokerMissionRules
	}{
		{"First Hand", "Learn to build three poker hands.", game.PokerMissionRules{NineBase: 12, BountyBase: 1, Par: 160}},
		{"The Crown", "The 9-bonus is your friend.", game.PokerMissionRules{NineBase: 12, BountyBase: 1, Par: 175}},
		{"Find Your Suit", "Bounty starts paying.", game.PokerMissionRules{NineBase: 12, BountyBase: 2, Par: 185}},
		{"Nine Storm", "Bounty-heavy config.", game.PokerMissionRules{NineBase: 12, BountyBase: 3, Par: 195}},
		{"The Long Game", "First real push.", game.PokerMissionRules{NineBase: 12, BountyBase: 1, Par: 200}},
		{"Pure Suit", "Score without straights or flushes.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 1, Par: 175, Constraint: game.PokerNoStraightsOrFlushes,
		}},
		{"Gorilla's Game", "Every hand must contain a 9.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 2, Par: 195, Constraint: game.PokerNineInEveryHand,
		}},
		{"Silent Nines", "No 9s allowed in any completed hand.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 1, Par: 165, Constraint: game.PokerNoNines,
		}},
		{"The Bounty Hunter", "Every hand must contain your bounty suit.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 3, Par: 200, Constraint: game.PokerSameBountySuitEvery,
		}},
		{"Bodyb's Table", "Bounty-maxed. No constraint.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 3, Par: 205,
		}},
		{"Doubled Down", "Bounty is strong, but the 9-bonus is weaker.", game.PokerMissionRules{
			NineBase: 8, BountyBase: 3, Par: 200,
		}},
		{"The Marathon", "Score at least 180 in three consecutive games with the same hero.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 1, StreakCount: 3, StreakScore: 180,
		}},
		{"Hater's Crown", "Every hand must contain a 9 at a high par.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 3, Par: 210, Constraint: game.PokerNineInEveryHand,
		}},
		{"The Wall", "No straights or flushes, with a high par.", game.PokerMissionRules{
			NineBase: 12, BountyBase: 1, Par: 205, Constraint: game.PokerNoStraightsOrFlushes,
		}},
		{"The Crown", "The final test.", game.PokerMissionRules{NineBase: 12, BountyBase: 1, Par: 235}},
	}

	missions := make([]Mission, len(missionConfigs))
	for index, config := range missionConfigs {
		rules := config.rules
		missions[index] = Mission{
			Index: index + 1, Name: config.name, Hint: config.hint,
			Variant: "poker", VariantID: fmt.Sprintf("poker-%02d", index+1),
			RewardMarks: 400, PokerRules: &rules,
		}
	}
	return Campaign{
		ID: "poker", Name: "Poker Campaign",
		Description: "Build and score three poker hands with hero modifiers across 15 missions.",
		Variant:     "poker", Missions: missions,
	}
}
