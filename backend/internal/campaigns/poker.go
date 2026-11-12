package campaigns

import (
	"fmt"

	"prisonersofwar/internal/game"
)

func pokerCampaign() Campaign {
	rock, paper, scissors := game.Shield, game.Fang, game.Blade
	missionConfigs := []struct {
		name  string
		hint  string
		rules game.PokerMissionRules
	}{
		{
			name: "The First Hand", hint: "Learn poker hand scoring and the 9-bonus.",
			rules: pokerRules(12, 1, 160, &rock,
				[]game.Card{card(rock, 9), card(paper, 9), card(scissors, 3), card(rock, 4), card(paper, 5), card(scissors, 6)},
				"", "Welcome to the poker mission. You'll place cards into three hands (A, B, C). Each hand scores as a poker hand: pair = 20, flush = 40, straight = 50, three of a kind = 60, straight flush = 100. Fill all three hands to maximize your score. You have a pair of 9s in your starting hand — place them together for a strong hand.",
				[]string{"hunter", "warden"}),
		},
		{
			name: "Finding the Nine", hint: "A 9 can add a major bonus to its hand.",
			rules: pokerRules(12, 1, 175, &paper,
				[]game.Card{card(rock, 9), card(paper, 2), card(scissors, 3), card(rock, 4), card(paper, 5), card(scissors, 6)},
				"", "A hand that contains at least one 9 gets a big bonus: +12 + 2×Shift. For most heroes, that's +12 to +16 per 9-hand. Chasing 9s is the most direct scoring path. You have one 9 — place it where it can anchor a strong hand.",
				[]string{"ascendant", "hater"}),
		},
		{
			name: "Your Suit", hint: "Stack bounty-suit cards to earn bounty bonuses.",
			rules: pokerRules(12, 1, 185, &scissors,
				[]game.Card{card(scissors, 2), card(scissors, 5), card(scissors, 8), card(rock, 3), card(paper, 4), card(rock, 7)},
				"", "When a hand contains enough cards of your bounty suit, it scores bonus points. The base payout is 1 + your hero's Bounty stat. Your hero's Favor stat changes how many bounty cards you need: Favor +1 needs only 1, Favor 0 needs 2, Favor −1 needs 3. You have three Scissors cards — try to trigger bounty in at least one hand.",
				[]string{"herald", "hunter"}),
		},
		{
			name: "The Even Table", hint: "Keep all three hands within one card of each other.",
			rules: pokerRules(12, 1, 185, &rock,
				[]game.Card{card(rock, 2), card(paper, 4), card(scissors, 6), card(paper, 3), card(rock, 5), card(scissors, 7)},
				"", "All three hands must be within 1 card of each other at all times. You can't fill one hand while ignoring the others. This means every card placement matters — you're building three hands simultaneously, not one strong hand and two dumps.",
				[]string{"recruit", "warden"}),
		},
		{
			name: "The Bonus Table", hint: "Make Hand A your strongest and Hand C your weakest.",
			rules: pokerRules(12, 1, 195, &rock,
				[]game.Card{card(rock, 9), card(paper, 3), card(scissors, 3), card(rock, 2), card(paper, 4), card(scissors, 6)},
				"", "If Hand A scores strictly higher than B and C (using the full hand total, including 9-bonus and bounty), you get +15. If Hand C scores strictly lower than A and B, you get +5. Both bonuses can fire together if A > B > C. Make A your best hand and C your worst.",
				[]string{"gambler", "hater"}),
		},
		{
			name: "Chase the Scissors", hint: "The ghost chases bounty; use your 9s to beat its score.",
			rules: pokerRules(12, 1, 200, &scissors,
				[]game.Card{card(rock, 9), card(scissors, 9), card(paper, 2), card(rock, 4), card(scissors, 6), card(paper, 7)},
				"", "The CPU is demonstrating a bounty-chasing strategy. It will stack Scissors cards into one hand to trigger bounty repeatedly. Watch how it plays, then beat it by chasing 9s instead — you have two 9s in your starting hand.",
				[]string{"ascendant", "hater", "gambler"}),
		},
		{
			name: "Chase the Paper", hint: "The ghost chases 9s; use Paper bounty to beat its score.",
			rules: pokerRules(12, 1, 200, &paper,
				[]game.Card{card(paper, 2), card(paper, 5), card(paper, 8), card(rock, 3), card(scissors, 4), card(rock, 6)},
				"", "The CPU is demonstrating a 9-chasing strategy. It will place its 9 into Hand A immediately and build everything else around that hand. Beat it by chasing Paper bounty instead — you have three Paper cards ready.",
				[]string{"herald", "hunter", "gambler"}),
		},
		{
			name: "Silent Nines", hint: "9s cannot be placed in any hand in this mission.",
			rules: pokerRules(12, 1, 175, &rock,
				[]game.Card{card(rock, 2), card(paper, 3), card(scissors, 4), card(rock, 5), card(paper, 6), card(scissors, 7)},
				game.PokerNoNines, "This mission bans 9s. You cannot place a 9 into any hand, and any hand that contains a 9 fails the mission. Your scoring options are limited to pairs (20), straights (50), flushes (40), and bounty. Build hands without relying on the 9-bonus.",
				[]string{"herald", "hunter", "warden"}),
		},
		{
			name: "The Wall", hint: "Straights and flushes score no base points.",
			rules: pokerRules(12, 1, 195, &paper,
				[]game.Card{card(rock, 9), card(paper, 9), card(scissors, 3), card(rock, 3), card(paper, 4), card(scissors, 5)},
				game.PokerNoStraightsOrFlushes, "No straights or flushes. Any hand that scores as a straight (50) or flush (40) instead scores 0 for its base. Your best paths are pairs (20), three of a kind (60), 9-bonus, and bounty. You have a pair of 9s and a pair of 3s — place them carefully.",
				[]string{"hunter", "warden"}),
		},
		{
			name: "Free Play", hint: "Play a default game and score at least 205.",
			rules: pokerRules(12, 1, 205, nil, nil, "",
				"No scripts, no forced suits, no constraints. This is the default game. Score 205 to advance. Everything you've learned so far is fair game.",
				[]string{"gambler", "hater", "hunter"}),
		},
		{
			name: "High Bar", hint: "Choose a high-scoring hero to reach the 210 par.",
			rules: pokerRules(12, 1, 210, &rock, nil, "",
				"Par 210 is high. The average game scores about 193; only heroes with strong scoring modifiers clear 210 consistently. Hater (mean 198), Gambler (mean 198), and Hunter (mean 196) are the best picks. Recruit (mean 189) can clear 210 but needs a strong draw.",
				[]string{"hater", "gambler", "hunter"}),
		},
		{
			name: "Consistency", hint: "Score at least 175 in three consecutive games; changing heroes resets the streak.",
			rules: func() game.PokerMissionRules {
				rules := pokerRules(12, 1, 175, nil, nil, "",
					"Three consecutive games ≥ 175. Pick a hero whose scores are stable, not spiky. Gambler (IQR 50, mean 198) and Hunter (IQR 47, mean 196) are the most consistent top-tier heroes. Turtle (IQR 52) is stable but low-scoring. Recruit (IQR 50) is a safe pick if you can hit 175.",
					[]string{"gambler", "hunter", "recruit"})
				rules.StreakCount, rules.StreakScore = 3, 175
				return rules
			}(),
		},
		{
			name: "Doubled Bounty", hint: "Bounty triggers are stronger and the 9-bonus is weaker.",
			rules: pokerRules(8, 3, 210, &scissors, nil, "",
				"This mission triples the bounty payout. A bounty trigger pays max(0, 3 + Bounty) instead of max(0, 1 + Bounty). Bodyb's Bounty +3 makes him S-tier: 6 points per trigger × about 1.4 triggers = about 8 bonus points from bounty alone. Meanwhile, the 9-bonus is only 8 + 2×Shift (vs. 12 + 2×Shift normally), so Shift-heavy heroes are weaker.",
				[]string{"herald", "hunter", "gambler"}),
		},
		{
			name: "Nine Storm", hint: "The 9-bonus is stronger; choose a Shift-heavy hero.",
			rules: pokerRules(16, 1, 215, &paper, nil, "",
				"This mission raises the 9-bonus to 16 + 2×Shift. Gorilla's Shift 2 gives him 20 points per 9-hand (vs. 16 normally). Three 9-hands = 60 points from the 9-bonus alone. Meanwhile, bounty is unchanged, so bounty-focused heroes are weaker relative to the 9-chasers.",
				[]string{"ascendant", "hater", "gambler"}),
		},
		{
			name:  "The Crown",
			rules: pokerRules(12, 1, 235, nil, nil, "", "", nil),
		},
	}

	missions := make([]Mission, len(missionConfigs))
	for index, config := range missionConfigs {
		rules := config.rules
		if index == 5 {
			rules.Ghost = &game.PokerGhostRules{
				Strategy: game.PokerGhostBounty,
				StartingHand: []game.Card{
					card(scissors, 2), card(scissors, 4), card(scissors, 6),
					card(rock, 3), card(paper, 5), card(rock, 7),
				},
				TeachingNote: "Watch the CPU's plays. It's chasing Scissors — every Scissors card goes into its best hand. By turn 5, you'll see the pattern. Your job: outscore it. Chase 9s instead of Scissors to exploit the difference.",
			}
		} else if index == 6 {
			rules.Ghost = &game.PokerGhostRules{
				Strategy: game.PokerGhostNine,
				StartingHand: []game.Card{
					card(rock, 9), card(paper, 2), card(scissors, 4),
					card(rock, 5), card(scissors, 6), card(paper, 8),
				},
				TeachingNote: "The CPU is chasing 9s. You have three Paper cards and a forced Paper bounty suit. Chase Paper bounty instead of 9s — the two strategies are equally strong, but only one of you is playing each.",
			}
		}
		missions[index] = Mission{
			Index: index + 1, Name: config.name, Hint: config.hint,
			Variant: "poker", VariantID: fmt.Sprintf("poker-%02d", index+1),
			RewardMarks: AdvancedMissionMarks, PokerRules: &rules,
		}
	}
	return Campaign{
		ID: "poker", Name: "Poker Campaign",
		Description: "Build and score three poker hands across 15 missions, including two strategy ghosts.",
		Variant:     "poker", Missions: missions,
	}
}

func pokerRules(nineBase, bountyBase, par int, bountySuit *game.Suit, startingHand []game.Card, constraint game.PokerMissionConstraint, lesson string, recommendedHeroes []string) game.PokerMissionRules {
	return game.PokerMissionRules{
		NineBase: nineBase, BountyBase: bountyBase, Par: par,
		ForcedBountySuit: bountySuit, StartingHand: startingHand,
		Constraint: constraint, Lesson: lesson,
		RecommendedHeroes: recommendedHeroes,
	}
}
