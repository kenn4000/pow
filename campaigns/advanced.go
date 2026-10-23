package campaigns

func advancedCampaign() Campaign {
	heroes := []string{"warden", "hunter", "anchor"}
	missions := []Mission{
		{
			Index: 1, Name: "The Hunter's Game",
			Flavor: "Meet the Hunter. Loves a good bounty, that one. Take him down, and you'll learn what a bounty hero does.",
			Hint:   "The Hunter adds +2 Bounty. Pick a hero that counters or complements. Warden is a safe pick here.",
			Type:   ThreeRound, AILevel: AIGreedy, AIHero: "hunter",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks, CPUMirrorIndex: -1,
		},
		{
			Index: 2, Name: "The Treasurer",
			Flavor: "The Treasurer doesn't care about bounties. Look what happens when you fight a suppressor.",
			Hint:   "The Treasurer subtracts 2 Bounty. If you also pick a bounty hero, the effects combine. Pick Warden for balance, or Hunter to see the Treasurer cancel you.",
			Type:   ThreeRound, AILevel: AIGreedy, AIHero: "treasurer",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks, CPUMirrorIndex: -1,
		},
		{
			Index: 3, Name: "The Ascendant",
			Flavor: "The Ascendant makes every card stronger. Every doubling is more devastating. High-stakes game.",
			Hint:   "The Ascendant adds +2 Shift. Your cards also get +2 face. Doubling becomes much more powerful.",
			Type:   ThreeRound, AILevel: AIGreedy, AIHero: "ascendant",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks, CPUMirrorIndex: -1,
		},
		{
			Index: 4, Name: "The Gambler",
			Flavor: "The Gambler bets on everything. Bounty and shift at once. He expects you to blink first.",
			Hint:   "The Gambler adds +2 Bounty and +1 Shift. Multi-axis hero. Pick Warden for a balanced game, or Hunter to lean into the bounty.",
			Type:   ThreeRound, AILevel: AIGreedy, AIHero: "gambler",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks, CPUMirrorIndex: -1,
		},
		{
			Index: 5, Name: "The Hater",
			Flavor: "The Hater is a problem. His flip makes your hero's strengths into weaknesses. Beat him and you've learned everything.",
			Hint:   "The Hater flips your hero's Bounty contribution. Pick Warden (small contributions, low flip impact) or Hunter (your +2 becomes -2). Anchor's negatives become positives - interesting but risky.",
			Type:   ThreeRound, AILevel: AIGreedy, AIHero: "hater",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks, CPUMirrorIndex: -1,
		},
	}
	return Campaign{
		ID:          "advanced",
		Name:        "Advanced Training",
		Description: "Learn to fight with heroes. Every hero changes the game.",
		Missions:    missions,
		Requires:    "basic",
		Completion: CampaignRewards{
			Marks: AdvancedCompleteMarks, HeroBoxes: 1, Badge: "badge_advanced_training",
		},
	}
}
