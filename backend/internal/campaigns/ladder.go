package campaigns

import "prisonersofwar/internal/game"

func ladderCampaignArc() Campaign {
	missions := []Mission{
		ladderMission(1, "First Chain", "Climb the chain with legal cards. You can play high or build upward.", "recruit", "recruit",
			ladderRules(5, game.Shield, "S2 S4 S6 S8 S9", "P3 P5 P7 P8 P9", 5, 5, [4]int{}, "random", false)),
		ladderMission(2, "The Doubling Rule", "A low Rock can beat a much higher Scissors card.", "recruit", "recruit",
			ladderRules(7, game.Fang, "R3 R4 R5 R6 R8 R9", "S4 S5 S6 S7 S8 S9", 6, 6, [4]int{}, "greedy", false)),
		ladderMission(3, "Wild Awakening", "Hold the wild until the chain strands you, or use its suit to dominate a reply.", "recruit", "recruit",
			ladderRules(4, game.Blade, "R2 R3 R4 R6 R8 R9", "S2 S3 S4 S6 S8 S9", 6, 6, [4]int{}, "greedy", false)),
		ladderMission(4, "Favor of the Suit", "Favor strengthens your winning suit. Use the Warden's bonus against Scissors.", "warden", "recruit",
			ladderRules(9, game.Shield, "R2 R3 R5 R6 R8 R9", "S2 S3 S5 S6 S8 S9", 6, 6, [4]int{0, 0, 0, 1}, "greedy", false)),
		ladderMission(5, "Burden of Favor", "Negative Favor weakens low-card suit wins; save your stronger Rocks.", "anchor", "anchor",
			ladderRules(9, game.Shield, "R2 R3 R5 R6 R8 R9", "S2 S3 S5 S6 S8 S9", 6, 6, [4]int{0, 0, 0, -2}, "greedy", false)),
		ladderMission(6, "Shifted Ground", "Shift raises every face. Even your 1 can meet the chain after the shift.", "ascendant", "ascendant",
			ladderRules(7, game.Fang, "R1 R2 R4 R6 R8 R9", "S3 S4 S5 S7 S8 S9", 6, 6, [4]int{0, 0, 2, 0}, "greedy", false)),
		ladderMission(7, "The Draft Table", "Draft two cards from each hand before play, then win with the improved hand.", "gambler", "gambler",
			ladderRules(6, game.Blade, "R1 R2 R3 R7 R8 R9 P9", "S1 S2 S3 S7 S8 S9 R4", 7, 7, [4]int{2, 2, 1, 0}, "greedy", false)),
		ladderMission(8, "Initiative", "The lowest card of the bounty suit determines who opens the chain.", "recruit", "recruit",
			ladderRules(8, game.Fang, "S2 P3 P5 P7 P8 P9 S9", "P1 P2 P4 P6 R8 R9 S8", 7, 7, [4]int{}, "greedy", false)),
		ladderMission(9, "The Pass", "When no legal play remains, pass and make the opponent open a fresh chain.", "recruit", "recruit",
			ladderRules(3, game.Shield, "S2 S3 S4 S5 S6 S9", "R1 R2 R4 R6 R7 R8", 6, 6, [4]int{}, "smart-pass", false)),
		ladderMission(10, "Denial", "The CPU avoids giving your Rocks a Scissors chain. Win with efficient raw values.", "recruit", "warden",
			ladderRules(9, game.Blade, "R2 R3 R4 R5 R6 R8 R9", "S2 S3 S4 S5 S6 S8 S9", 7, 7, [4]int{0, 0, 0, 1}, "denial", false)),
		ladderMission(11, "The Wild Finisher", "Save your wild until the late game; the CPU will hoard its own wild.", "recruit", "recruit",
			ladderRules(2, game.Shield, "R1 R2 R3 R4 R5 R6 R7 R8", "S1 S2 S3 S4 S5 S6 S7 S8", 8, 8, [4]int{}, "wild-finisher", false)),
		ladderMission(12, "Short Hand", "You have fewer cards than the CPU. Use your 9 Scissors to seize tempo.", "recruit", "recruit",
			ladderRules(5, game.Fang, "S2 S4 S6 S7 S8 S9 R9", "P1 P2 P3 P4 P5 P6 P7 P8 P9 R8", 7, 10, [4]int{}, "attrition", false)),
		ladderMission(13, "Long Grind", "Manage twelve cards and hold a high finisher for the late game.", "herald", "herald",
			ladderRules(9, game.Blade, "", "", 12, 12, [4]int{3, 0, 0, 0}, "attrition", false)),
		ladderMission(14, "The Mirror", "Same heroes and balanced hands: beat an open-hand CPU with careful play.", "warden", "warden",
			ladderRules(7, game.Shield, "R1 R2 R3 S4 S5 S6 P7 P8 P9", "R4 R5 R6 S1 S2 S3 P1 P2 P3", 9, 9, [4]int{1, 0, 0, 2}, "hard", true)),
		ladderMission(15, "The Gauntlet", "Choose your hero against a revealed random opponent and master every Ladder rule.", "", "random",
			ladderRules(0, game.Shield, "", "", 11, 11, [4]int{}, "hard", true)),
	}
	missions[12].Ladder.RandomHands = true
	missions[14].Ladder.RandomHands = true
	missions[14].Ladder.BountySuit = game.Suit(-1)
	missions[14].Ladder.SharedStats = nil
	return Campaign{
		ID: "ladder", Name: "Ladder Campaign", Description: "Learn the chain, wilds, suit pressure, and timing across 15 missions.",
		Variant: "ladder", Missions: missions,
	}
}

func ladderMission(index int, name, hint, playerHero, cpuHero string, rules *LadderCampaignRules) Mission {
	mission := Mission{
		Index: index, Name: name, Hint: hint, Variant: "ladder", AIHero: cpuHero,
		RewardMarks: AdvancedMissionMarks, Ladder: rules,
	}
	if playerHero != "" {
		mission.PlayerHeroRestriction = []string{playerHero}
	}
	return mission
}

func ladderRules(wildFace int, bounty game.Suit, playerHand, cpuHand string, playerSize, cpuSize int,
	stats [4]int, policy string, openHand bool,
) *LadderCampaignRules {
	return &LadderCampaignRules{
		WildFace: wildFace, BountySuit: bounty, PlayerHand: campaignRummyCards(playerHand),
		CPUHand: campaignRummyCards(cpuHand), PlayerSize: playerSize, CPUSize: cpuSize,
		SharedStats: &LadderSharedStats{Bounty: stats[0], Draft: stats[1], Shift: stats[2], Favor: stats[3]},
		CPUPolicy:   policy, OpenHand: openHand,
	}
}
