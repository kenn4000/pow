package campaigns

import "prisonersofwar/internal/game"

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
			Index: 3, Name: "The Gorilla",
			Flavor: "The Gorilla makes every card stronger. Every doubling is more devastating. High-stakes game.",
			Hint:   "The Gorilla adds +2 Shift. Your cards also get +2 face. Doubling becomes much more powerful.",
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
			Hint:   "The Hater flips your hero's Bounty contribution. Pick Warden (small contributions, low flip impact) or Hunter (your +2 becomes -2). The Turtle's negatives become positives - interesting but risky.",
			Type:   ThreeRound, AILevel: AIGreedy, AIHero: "hater",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks, CPUMirrorIndex: -1,
		},
		{
			Index: 6, Name: "No Draft Duel",
			Flavor: "No public pool, no draft. Five cards each and a fight to the finish. You will have to defend the opening attack.",
			Hint:   "Draft is skipped (mode 0). Both players receive five hole cards. The CPU attacks first in Round 1, so you begin on defense.",
			Type:   ThreeRound, DraftMode: 0, PlayerSeat: seatValue(game.SeatY),
			BountyOverride: intValue(0), ShiftOverride: intValue(0),
			FavoredOverride: intValue(0), ReckoningOverride: weightsValue([3]int{1, 1, 1}),
			CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks,
		},
		{
			Index: 7, Name: "Unlucky",
			Flavor: "The CPU has already drawn every nine in the deck. You will have to outplay those perfect hole cards.",
			Hint:   "Draft mode 2: three hole cards each and six public cards. All three nines are dealt to the CPU before the draft.",
			Type:   ThreeRound, DraftMode: 2, PlayerSeat: seatValue(game.SeatX),
			CPUHoleCards: []game.Card{
				card(game.Blade, 9), card(game.Shield, 9), card(game.Fang, 9),
			},
			BountyOverride: intValue(0), ShiftOverride: intValue(0),
			FavoredOverride: intValue(0), ReckoningOverride: weightsValue([3]int{1, 1, 1}),
			CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks,
		},
		{
			Index: 8, Name: "Harsh",
			Flavor: "The CPU drafts first, but you will attack first in the opening round. Make that first move count.",
			Hint:   "Draft mode 4: one hole card each and ten public cards. The CPU picks first; you attack first in Round 1.",
			Type:   ThreeRound, DraftMode: 4, PlayerSeat: seatValue(game.SeatX),
			BountyOverride: intValue(0), ShiftOverride: intValue(0),
			FavoredOverride: intValue(0), ReckoningOverride: weightsValue([3]int{1, 1, 1}),
			CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks,
		},
		{
			Index: 9, Name: "Blind Aggression",
			Flavor: "Take the first pick, then keep the initiative. You will attack first at the start of every round.",
			Hint:   "Draft mode 3: two hole cards each and eight public cards. You pick first and attack first in all three rounds.",
			Type:   ThreeRound, DraftMode: 3, PlayerSeat: seatValue(game.SeatX),
			PlayerDraftsFirst: true, PlayerAttacksFirstAll: true,
			BountyOverride: intValue(0), ShiftOverride: intValue(0),
			FavoredOverride: intValue(0), ReckoningOverride: weightsValue([3]int{1, 1, 1}),
			CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks,
		},
		{
			Index: 10, Name: "Blind Aggression 2",
			Flavor: "The CPU gets the first pick. You still take the initiative and attack first at the start of every round.",
			Hint:   "Draft mode 3: two hole cards each and eight public cards. The CPU picks first; you attack first in all three rounds.",
			Type:   ThreeRound, DraftMode: 3, PlayerSeat: seatValue(game.SeatX),
			PlayerAttacksFirstAll: true,
			BountyOverride:        intValue(0), ShiftOverride: intValue(0),
			FavoredOverride: intValue(0), ReckoningOverride: weightsValue([3]int{1, 1, 1}),
			CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
			PlayerHeroRestriction: heroes, RewardMarks: AdvancedMissionMarks,
		},
	}
	return Campaign{
		ID:          "advanced",
		Name:        "Advanced Training",
		Description: "Learn to fight with heroes. Every hero changes the game.",
		Missions:    missions,
		Requires:    "basic",
		Completion: CampaignRewards{
			Marks: AdvancedCompleteMarks, HeroBoxes: 1, SkillPoints: 1, Badge: "badge_advanced_training",
		},
	}
}
