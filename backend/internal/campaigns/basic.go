package campaigns

import "prisonersofwar/internal/game"

func basicCampaign() Campaign {
	recruit := []string{"recruit"}
	standardWeights := [3]int{1, 1, 1}
	return Campaign{
		ID:          "basic",
		Name:        "Basic Training",
		Description: "Learn the rules of Prisoners of War, one challenge at a time.",
		Missions: []Mission{
			{
				Index: 1, Name: "First Blood",
				Flavor: "Welcome to the camp, Recruit. Before we fight heroes, we fight like soldiers. Deal your cards, commit three, and see what happens. Simple.",
				Hint:   "Your training hand contains 6 Rock cards. Commit 3 to your round pile and play 3 exchanges. Higher face wins in this same-suit lesson. Capture cards to score.",
				Type:   SingleRound,
				PlayerHand: []game.Card{
					card(game.Shield, 5), card(game.Shield, 6), card(game.Shield, 7),
					card(game.Shield, 8), card(game.Shield, 9), card(game.Shield, 1),
				},
				CPUPool: []game.Card{card(game.Shield, 2), card(game.Shield, 3), card(game.Shield, 4)},
				CPULeads: []game.Card{
					card(game.Shield, 2), card(game.Shield, 3), card(game.Shield, 4),
				},
				CPUMirrorIndex: -1, BountyOverride: intValue(0),
				ReckoningOverride: weightsValue(standardWeights),
				AILevel:           AIEasy, AIHero: "recruit", PlayerHeroRestriction: recruit,
				RewardMarks: BasicMissionMarks,
			},
			{
				Index: 2, Name: "The Clash",
				Flavor: "Sometimes two cards are exactly equal. That's a clash - both destroyed, no capture. Rare, but you need to know it when it happens. Watch.",
				Hint:   "You have 3, 5, 7. The CPU has a 9 first. Save your 7. The middle exchange will tie - that's a clash. The last exchange is the decider.",
				Type:   SingleRound,
				PlayerHand: []game.Card{
					card(game.Shield, 3), card(game.Shield, 5), card(game.Shield, 7),
				},
				CPULeads: []game.Card{
					card(game.Shield, 9), {}, card(game.Shield, 6),
				},
				CPUPool: []game.Card{
					card(game.Shield, 9), card(game.Shield, 6), card(game.Fang, 8),
				},
				CPUAttacksAll: true, CPUMirrorIndex: 1,
				BountyOverride: intValue(0), ReckoningOverride: weightsValue(standardWeights),
				AILevel: AIEasy, AIHero: "recruit", PlayerHeroRestriction: recruit,
				RewardMarks: BasicMissionMarks,
			},
			{
				Index: 3, Name: "The Cycle",
				Flavor: "Rock beats Scissors. Scissors beats Paper. Paper beats Rock. Those aren't just words - they're the whole game. Watch carefully.",
				Hint:   "Your three cards are one of each suit. Match the right card against each CPU lead. The cycle is your guide.",
				Type:   SingleRound,
				PlayerHand: []game.Card{
					card(game.Shield, 5), card(game.Fang, 5), card(game.Blade, 5),
				},
				CPUPool: []game.Card{
					card(game.Blade, 9), card(game.Shield, 9), card(game.Fang, 9),
				},
				CPULeads: []game.Card{
					card(game.Blade, 9), card(game.Shield, 9), card(game.Fang, 9),
				},
				CPUMirrorIndex: -1, BountyOverride: intValue(0),
				ReckoningOverride: weightsValue(standardWeights),
				AILevel:           AIEasy, AIHero: "recruit", PlayerHeroRestriction: recruit,
				RewardMarks: BasicMissionMarks,
			},
			{
				Index: 4, Name: "The Bounty",
				Flavor: "This match, the Warden has put a bounty on Rock prisoners. Every Rock card you capture is worth more. Capture the bounty. Ignore the bait.",
				Hint:   "The CPU leads all three. Paper 4 counters the Rock 7. Rock 4 ties the Scissors 8. Your Scissors 7 loses to the Rock 8 - but that's the game. Capture what you can.",
				Type:   SingleRound,
				PlayerHand: []game.Card{
					card(game.Fang, 4), card(game.Shield, 4), card(game.Blade, 7),
				},
				CPUPool: []game.Card{
					card(game.Shield, 7), card(game.Blade, 8), card(game.Shield, 8),
				},
				CPULeads: []game.Card{
					card(game.Shield, 7), card(game.Blade, 8), card(game.Shield, 8),
				},
				CPUAttacksAll: true, CPUMirrorIndex: -1,
				BountyOverride: intValue(2), BountySuitOverride: stringValue("R"),
				ReckoningOverride: weightsValue(standardWeights),
				AILevel:           AIEasy, AIHero: "recruit", PlayerHeroRestriction: recruit,
				RewardMarks: BasicMissionMarks,
			},
			{
				Index: 5, Name: "Standard",
				Flavor: "No tricks, no bounties, no heroes. Just a straight match. Let's see if Basic Training stuck.",
				Hint:   "This is a standard 3-round match. Seat X receives 6 cards and Seat Y receives 5. Commit 3 per round; hands refill between rounds. Best score after 3 rounds wins.",
				Type:   ThreeRound, BountyOverride: intValue(0), ShiftOverride: intValue(0),
				FavoredOverride: intValue(0), ReckoningOverride: weightsValue(standardWeights),
				CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
				PlayerHeroRestriction: recruit, RewardMarks: BasicMissionMarks,
			},
			{
				Index: 6, Name: "The Reckoning",
				Flavor: "This match, the third round counts triple. Don't blow your best cards early. Save something for when it matters.",
				Hint:   "Round weights are 1 / 2 / 3. Round 3 captures count three times as much for scoring. Save your best cards for the final round.",
				Type:   ThreeRound, BountyOverride: intValue(0), ShiftOverride: intValue(0),
				FavoredOverride: intValue(0), ReckoningOverride: weightsValue([3]int{1, 2, 3}),
				CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
				PlayerHeroRestriction: recruit, RewardMarks: BasicMissionMarks,
			},
			{
				Index: 7, Name: "Negative Bounty",
				Flavor: "Sometimes the bounty works against you. This match, Rock prisoners are worth less. Let the CPU capture them. Don't fight for what's not worth having.",
				Hint:   "Bounty is Rock -2. Captured Rock cards score 2 less than their face. Sometimes letting the opponent capture a card is better than winning it yourself.",
				Type:   ThreeRound, BountySuitOverride: stringValue("R"),
				BountyOverride: intValue(-2), ShiftOverride: intValue(0),
				FavoredOverride: intValue(0), ReckoningOverride: weightsValue(standardWeights),
				CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
				PlayerHeroRestriction: recruit, RewardMarks: BasicMissionMarks,
			},
			{
				Index: 8, Name: "Favored",
				Flavor: "This match, every cycle-win gets +2. Doubling isn't just strong anymore - it's decisive. Watch your suit cycles.",
				Hint:   "Favored +2. Whenever your suit beats theirs, your doubled value gets +2. Seek cycle-wins.",
				Type:   ThreeRound, ShiftOverride: intValue(0), BountyOverride: intValue(0),
				FavoredOverride: intValue(2), ReckoningOverride: weightsValue(standardWeights),
				CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
				PlayerHeroRestriction: recruit, RewardMarks: BasicMissionMarks,
			},
			{
				Index: 9, Name: "Shift",
				Flavor: "This match, every card is effectively +2 face. All cards are stronger. Doubling on high cards is devastating.",
				Hint:   "Shift +2. Every card's effective face is +2. A face-7 is effectively a 9. High cards get very high, and doubling them is massive.",
				Type:   ThreeRound, ShiftOverride: intValue(2), BountyOverride: intValue(0),
				FavoredOverride: intValue(0), ReckoningOverride: weightsValue(standardWeights),
				CPUMirrorIndex: -1, AILevel: AIEasy, AIHero: "recruit",
				PlayerHeroRestriction: recruit, RewardMarks: BasicMissionMarks,
			},
			{
				Index: 10, Name: "Final Exam",
				Flavor: "This is it, Recruit. Bounty +2, favored -1, shift +1, round 3 counts double. Everything you've learned, all at once. Win this and you're a Warden.",
				Hint:   "Combined: B+2 (Rock +2), S+1 (all cards +1 face), F-1 (cycle-winners -1), Reckoning 1,1,2. Round 3 counts double.",
				Type:   ThreeRound, BountySuitOverride: stringValue("R"),
				BountyOverride: intValue(2), ShiftOverride: intValue(1),
				FavoredOverride: intValue(-1), ReckoningOverride: weightsValue([3]int{1, 1, 2}),
				CPUMirrorIndex: -1, AILevel: AINormal, AIHero: "recruit",
				PlayerHeroRestriction: recruit, RewardMarks: BasicMissionMarks,
			},
		},
		Completion: CampaignRewards{
			Marks: BasicCompleteMarks, Heroes: []string{"warden", "hunter", "anchor"},
			Badge: "badge_basic_training", UnlocksCampaign: "advanced",
		},
	}
}
