package campaigns

import "strconv"

func abandonCampaign() Campaign {
	configs := []struct {
		pairs, boardCards, target int
	}{
		{4, 11, 35}, {4, 12, 36}, {4, 13, 37},
		{5, 11, 44}, {5, 12, 45}, {5, 13, 46},
		{6, 11, 53}, {6, 12, 54}, {6, 13, 55},
	}
	missions := make([]Mission, 15)
	for index, config := range configs {
		missions[index] = Mission{
			Index: index + 1, Name: "Mission " + strconv.Itoa(index+1),
			Hint:    "Make the required pairs and reach the target score.",
			Variant: "abandon", RewardMarks: 400, Target: config.target,
			AbandonPairs: config.pairs, AbandonBoardCards: config.boardCards,
		}
	}
	for index := len(configs); index < len(missions); index++ {
		missions[index] = Mission{
			Index: index + 1, Name: "Mission " + strconv.Itoa(index+1),
			Hint: "Coming Soon", Variant: "abandon", Status: "Coming Soon", Disabled: true,
		}
	}
	return Campaign{
		ID: "abandon", Name: "Abandon", Description: "Reveal cards, battle for prisoners, and avoid abandoning your personal pile.",
		Variant: "abandon", Missions: missions,
	}
}

func puzzlerCampaign() Campaign {
	puzzleIDs := []string{
		"gp6_balanced", "gp6_hunters_edge", "gp6_bounty_trap", "gp6_narrow_gate",
		"gp6_bodybuilders_wall", "gp8_balanced", "gp8_bounty_climb", "gp8_forgiving",
		"gp8_gamblers_puzzle", "gp8_tight", "gp10_balanced", "gp10_gorilla_threat",
		"gp10_turtle_lead", "gp10_low_score", "gp10_everyone_solves", "gp12_balanced",
		"gp12_dual_threat", "gp12_shift_dominant", "gp12_forgiving", "gp12_inverted",
	}
	missions := make([]Mission, len(puzzleIDs))
	for index, puzzleID := range puzzleIDs {
		missions[index] = Mission{
			Index: index + 1, Name: puzzleID, RewardMarks: 400,
			Variant: "puzzler", VariantID: puzzleID,
		}
	}
	return Campaign{
		ID: "puzzler", Name: "Puzzle Master", Description: "Solve the Puzzle Master catalog in order.",
		Variant: "puzzler", Missions: missions,
	}
}

func ladderCampaign() Campaign {
	return Campaign{
		ID: "ladder", Name: "Ladder Campaign", Description: "Coming Soon",
		Variant: "ladder", Status: "Coming Soon",
		Missions: []Mission{{
			Index: 1, Name: "Coming Soon", Hint: "Coming Soon",
			Status: "Coming Soon", Disabled: true,
		}},
	}
}

func heroCampaigns() []Campaign {
	heroes := []struct {
		id   string
		name string
	}{
		{"warden", "Warden"},
		{"hunter", "Hunter"},
		{"treasurer", "Treasurer"},
		{"ascendant", "Gorilla"},
		{"anchor", "Turtle"},
		{"gambler", "Gambler"},
		{"diplomat", "Diplomat"},
		{"ascetic", "Bureaucrat"},
		{"herald", "Bodybuilder"},
		{"hater", "Hater"},
	}
	campaigns := make([]Campaign, len(heroes))
	for index, hero := range heroes {
		campaigns[index] = Campaign{
			ID: hero.id, Name: hero.name + "'s Campaign",
			Description: "Coming Soon", Status: "Coming Soon",
			Missions: []Mission{{
				Index: 1, Name: "Coming Soon", Hint: "Coming Soon",
				Status: "Coming Soon", Disabled: true,
			}},
		}
	}
	return campaigns
}
