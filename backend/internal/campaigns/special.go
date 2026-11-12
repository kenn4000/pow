package campaigns

import (
	"prisonersofwar/internal/game"
)

func abandonCampaign() Campaign {
	solo := func(name, hint string, board, hand []game.Card, bounty game.Suit, pairs, target int, heroLock bool) Mission {
		restriction := []string(nil)
		if heroLock {
			restriction = []string{"recruit"}
		}
		return Mission{
			Name: name, Hint: hint, Variant: "abandon", RewardMarks: AdvancedMissionMarks, Target: target,
			AbandonPairs: pairs, AbandonBoardCards: len(board), PlayerHeroRestriction: restriction,
			Abandon: &AbandonCampaignRules{
				Board: board, PlayerHand: hand, BoardSize: len(board), HandSize: len(hand),
				PairCount: pairs, Rounds: 1, BountySuits: [2]int{int(bounty), int(bounty)},
				Goal: "score", Target: target, MinCaptured: 1,
			},
		}
	}
	duel := func(name, hint string, board, hand, cpuHand []game.Card, boardSize, pairs, rounds, bounty int, snake, mirror bool, algo, goal string, target int) Mission {
		rules := &AbandonCampaignRules{
			Board: board, PlayerHand: hand, CPUHand: cpuHand,
			BoardSize: boardSize, HandSize: 5, PairCount: pairs, Rounds: rounds,
			BountySuits: [2]int{bounty, -1}, MirrorHands: mirror, Snake: snake,
			OpponentAlgo: algo, Goal: goal, Target: target,
		}
		if len(hand) > 0 {
			rules.HandSize = len(hand)
		}
		return Mission{
			Name: name, Hint: hint, Variant: "abandon", RewardMarks: AdvancedMissionMarks,
			Target: target, AbandonPairs: pairs, AbandonBoardCards: boardSize, Abandon: rules,
		}
	}
	r := game.Shield
	p := game.Fang
	s := game.Blade
	missions := []Mission{
		solo("First Blood", "Match dominant suits and capture at least one card.", []game.Card{
			card(r, 5), card(r, 3), card(p, 7), card(s, 4), card(s, 8), card(p, 2),
		}, []game.Card{card(r, 6), card(p, 9), card(s, 2)}, r, 1, 1, true),
		solo("Mirror", "Same-suit battles capture the lower face; find a winning matchup.", []game.Card{
			card(r, 3), card(r, 7), card(p, 5), card(p, 9), card(s, 1), card(s, 6),
		}, []game.Card{card(r, 2), card(p, 4), card(s, 8)}, p, 1, 3, true),
		solo("The Tie", "Equal battle values capture nothing; avoid the tempting tie.", []game.Card{
			card(r, 5), card(s, 5), card(p, 3), card(p, 8), card(r, 1), card(s, 9), card(s, 4),
		}, []game.Card{card(r, 4), card(p, 2), card(s, 7), card(r, 2)}, s, 1, 1, true),
		solo("Count and Peak", "Jail score multiplies each suit's count by its highest face.", []game.Card{
			card(r, 2), card(r, 3), card(r, 5), card(p, 9), card(p, 7), card(s, 4), card(s, 6), card(s, 2),
		}, []game.Card{card(r, 4), card(r, 6), card(p, 3), card(s, 1)}, p, 2, 12, true),
		solo("The Bounty", "Bounty-suit captures score extra; choose any hero.", []game.Card{
			card(r, 5), card(r, 8), card(p, 9), card(p, 3), card(s, 7), card(s, 4), card(s, 2), card(p, 6),
		}, []game.Card{card(r, 6), card(r, 7), card(s, 3), card(s, 5)}, r, 2, 15, false),
		solo("Don't Abandon", "Use high-value private cards carefully to limit abandon penalties.", []game.Card{
			card(r, 3), card(r, 7), card(r, 1), card(p, 4), card(p, 8), card(p, 2), card(s, 5), card(s, 9), card(s, 3), card(p, 6),
		}, []game.Card{card(r, 9), card(r, 2), card(p, 5), card(s, 7), card(s, 4)}, s, 2, 20, true),
		solo("Flip and Find", "Use the free flip to find the hidden high bounty card.", []game.Card{
			card(r, 5), card(r, 9), card(p, 3), card(p, 7), card(s, 2), card(s, 8), card(p, 4), card(s, 6),
		}, []game.Card{card(r, 4), card(s, 3), card(p, 6), card(s, 1)}, r, 2, 18, true),
		solo("Blind Gamble", "When visible captures are weak, consider gambling on hidden cards.", []game.Card{
			card(r, 7), card(r, 2), card(r, 4), card(p, 5), card(p, 8), card(p, 1), card(s, 3), card(s, 9), card(s, 5), card(p, 3),
		}, []game.Card{card(r, 9), card(p, 2), card(s, 4), card(r, 1)}, s, 2, 25, true),
		duel("The Mirror Match", "Beat a greedy opponent in an alternating two-player match.", nil, nil, nil, 12, 2, 1, -1, false, true, "GREEDY", "win", 0),
		duel("Snake Eyes", "Use snake-order initiative to win and capture at least two bounty cards.", []game.Card{
			card(r, 9), card(r, 8), card(r, 7), card(r, 6), card(p, 4), card(p, 2), card(p, 5), card(s, 3), card(s, 1), card(s, 6), card(p, 3), card(s, 4),
		}, []game.Card{card(p, 8), card(p, 7), card(s, 5), card(s, 8), card(r, 2)}, nil, 12, 3, 1, int(r), true, true, "BOUNTY_FOCUSED", "win_bounty", 2),
		duel("Two Rounds", "Play a two-round seat-swap match and win on cumulative score.", nil, nil, nil, 14, 3, 2, int(r), false, true, "GREEDY", "match_win", 0),
		duel("All-In", "Score at least 40 against an opponent that avoids suit concentration.", []game.Card{
			card(r, 9), card(r, 8), card(r, 7), card(r, 6), card(r, 5), card(r, 4), card(p, 3), card(p, 5), card(p, 1), card(p, 7), card(s, 2), card(s, 4), card(s, 8), card(s, 6),
		}, []game.Card{card(p, 9), card(p, 8), card(s, 9), card(s, 7), card(r, 3)}, nil, 14, 3, 1, int(r), true, false, "ANTI_CONCENTRATE", "score", 40),
		duel("Spread", "Score 30 or more and capture cards from all three suits.", []game.Card{
			card(r, 7), card(r, 2), card(p, 8), card(p, 3), card(p, 5), card(s, 9), card(s, 4), card(s, 6), card(r, 5), card(p, 1), card(s, 2), card(r, 4), card(p, 6), card(s, 3),
		}, []game.Card{card(r, 9), card(r, 6), card(p, 9), card(s, 7), card(s, 8)}, nil, 14, 3, 1, int(p), true, false, "ANTI_SPREAD", "score_all_suits", 30),
		duel("Tempo", "Secure strong visible captures early and adapt as the board changes.", []game.Card{
			card(r, 9), card(r, 8), card(p, 7), card(p, 6), card(s, 5), card(s, 4), card(p, 3), card(p, 2),
			card(r, 3), card(r, 2), card(p, 5), card(p, 4), card(s, 9), card(s, 8), card(s, 7), card(s, 6),
		}, []game.Card{card(p, 9), card(p, 8), card(s, 3), card(s, 2), card(r, 4)}, nil, 16, 4, 1, int(r), true, false, "SMART", "score", 35),
		duel("Abandon", "Win a two-round seat-swap match against the stronger, future-focused opponent.", nil, nil, nil, 16, 6, 2, -1, true, true, "SMART_HIGH", "match_win", 0),
	}
	for index := range missions {
		missions[index].Index = index + 1
	}
	targetCard := card(s, 1)
	missions[1].Abandon.Goal = "capture_or_score"
	missions[1].Abandon.TargetCard = &targetCard
	missions[5].Abandon.MaxAbandoned = 1
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
			Index: index + 1, Name: puzzleID, RewardMarks: AdvancedMissionMarks,
			Variant: "puzzler", VariantID: puzzleID,
		}
	}
	return Campaign{
		ID: "puzzler", Name: "Puzzle Master", Description: "Solve the Puzzle Master catalog in order.",
		Variant: "puzzler", Missions: missions,
	}
}

func ladderCampaign() Campaign {
	return ladderCampaignArc()
}
