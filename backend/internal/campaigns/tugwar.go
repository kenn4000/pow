package campaigns

import (
	"fmt"
	"strconv"
	"strings"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/tugwar"
)

func tugWarCampaign() Campaign {
	missions := []Mission{
		{
			Index: 1, Name: "First Steps", Hint: "Play the lowest card that wins a pile; spend your pushes to claim territory.",
			Variant: "tug-war", AIHero: "recruit", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("R1 R3 S5 P6 S8 R9", "P2 S2 R4 P5 S7 P8",
				[3]tugwar.Pile{twPile(0, "P1"), twPile(0, "R2"), twPile(0, "S3")},
				"", "S", [2]int{2, 0}, false, 0, tugwar.WinByScore, tugwar.CPUPolicyPassive),
		},
		{
			Index: 2, Name: "Push It", Hint: "Your hand cannot take these piles. Claim all three with pushes before round 3 ends.",
			Variant: "tug-war", AIHero: "recruit", RewardMarks: AdvancedMissionMarks,
			TugWar: func() *tugwar.CampaignMission {
				mission := fixedTugWarMission("R3 R4 R5 P2 S2 P4", "S8 P8 R7 S6 P6 R6",
					[3]tugwar.Pile{twPile(0, "R8"), twPile(0, "S9"), twPile(0, "P7")},
					"R1 S1 P1 R2 S3", "P", [2]int{3, 0}, false, 3,
					tugwar.WinByAllPiles, tugwar.CPUPolicyPushIt)
				mission.EarlyWin = true
				return mission
			}(),
		},
		{
			Index: 3, Name: "Denial", Hint: "Bounty cards can poison a pile. Watch what the CPU feeds and choose your territory carefully.",
			Variant: "tug-war", AIHero: "anchor", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("S4 S5 S7 R3 P2 R6", "S2 S3 S6 S9 R4 P5",
				[3]tugwar.Pile{twPile(0, "R5"), twPile(0, "P3"), twPile(0, "S8")},
				"R7 P6 S1 R8 P4 P7", "S", [2]int{2, 2}, true, 0,
				tugwar.WinByPositiveScorePile, tugwar.CPUPolicyDenial),
		},
		{
			Index: 4, Name: "The High Ground", Hint: "Pile A is the objective. Push it back while using the other piles to stall.",
			Variant: "tug-war", AIHero: "hunter", RewardMarks: AdvancedMissionMarks,
			TugWar: func() *tugwar.CampaignMission {
				mission := fixedTugWarMission("R6 R7 S5 P8 S9 P3", "P6 P5 R4 S4 P2 R3",
					[3]tugwar.Pile{twPile(1, "R5 S6 P4 P9"), twPile(0, "S3"), twPile(0, "R2")},
					"R9 S7 P7 S8 R8 P1 S1 R1", "R", [2]int{3, 0}, true, 4,
					tugwar.WinByPile, tugwar.CPUPolicyHighGround)
				mission.TargetPile = 0
				mission.EarlyWin = true
				return mission
			}(),
		},
		{
			Index: 5, Name: "The Siege", Hint: "Win all three piles by round 4. The CPU spreads its pressure across the board.",
			Variant: "tug-war", AIHero: "ascendant", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("R4 S7 P5 R7 S3 P9", "P7 R6 S8 P4 R2 S5",
				[3]tugwar.Pile{twPile(0, "R8"), twPile(0, "S6"), twPile(0, "P2")},
				"", "S", [2]int{2, 2}, false, 4, tugwar.WinByAllPiles, tugwar.CPUPolicySpreader),
		},
		{
			Index: 6, Name: "Fog of War", Hint: "Flip to scout hidden pile tops. The bounty suit stays hidden until the first flip.",
			// The roster has no Vulture entry; Treasurer provides the intended negative-bounty profile.
			Variant: "tug-war", AIHero: "treasurer", RewardMarks: AdvancedMissionMarks,
			TugWar: func() *tugwar.CampaignMission {
				mission := fixedTugWarMission("R5 S4 P6 R7 S8 P3", "P5 R4 S7 P2 R6 S3",
					[3]tugwar.Pile{twHiddenPile("R9"), twHiddenPile("S9"), twHiddenPile("P9")},
					"S2 P7 R3 S5 P1 R8 S6 P4", "P", [2]int{2, 2}, true, 0,
					tugwar.WinByScore, tugwar.CPUPolicyInfoGatherer)
				mission.HideBountyUntilFlip = true
				return mission
			}(),
		},
		{
			Index: 7, Name: "The Favor Economy", Hint: "Only Rock cards are played by the CPU. Use suit advantage and pushes to counter them.",
			Variant: "tug-war", AIHero: "warden", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("R2 R3 S9 P9 S4 P2", "R5 R6 R7 R4 R8 R9",
				[3]tugwar.Pile{twPile(0, "R1"), twPile(0, "S7"), twPile(0, "P6")},
				"", "R", [2]int{2, 2}, false, 0, tugwar.WinByScore, tugwar.CPUPolicyDominantSuit),
		},
		{
			Index: 8, Name: "Overdrive", Hint: "You are out-gunned. Find one pile the CPU is least likely to contest and survive to round 4.",
			Variant: "tug-war", AIHero: "ascendant", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("S2 P3 R4 S3 P2 R3", "R7 R8 S8 S9 R9 S7",
				[3]tugwar.Pile{twPile(0, "R6"), twPile(0, "S6"), twPile(0, "P9")},
				"R1 S1 P1", "S", [2]int{1, 3}, false, 4, tugwar.WinByAnyPile, tugwar.CPUPolicyBlitz),
		},
		{
			Index: 9, Name: "The Bodybuilder's Game", Hint: "The CPU commits every resource to pile B. Take piles A and C instead.",
			Variant: "tug-war", AIHero: "herald", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("R5 S6 S7 P4 R8 P7", "S3 S4 S5 S8 S9 R2",
				[3]tugwar.Pile{twPile(0, "R6"), twPile(0, "P5"), twPile(0, "S2")},
				"R1 P1 S1", "S", [2]int{3, 3}, false, 0, tugwar.WinByMostPiles, tugwar.CPUPolicyTunnelVision),
		},
		{
			Index: 10, Name: "No Pushes", Hint: "Pure card play: win by choosing suit advantage and spending your hand efficiently.",
			Variant: "tug-war", AIHero: "hater", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("R4 S6 P5 R7 S3 P8", "P6 R5 S7 P4 R6 S4",
				[3]tugwar.Pile{twPile(0, "R2"), twPile(0, "S5"), twPile(0, "P7")},
				"", "R", [2]int{0, 0}, false, 0, tugwar.WinByScore, tugwar.CPUPolicyEfficient),
		},
		{
			Index: 11, Name: "One-Way Pushes", Hint: "Your pushes only move toward you. Pile A cannot be recovered; win piles B and C.",
			Variant: "tug-war", AIHero: "diplomat", RewardMarks: AdvancedMissionMarks,
			TugWar: func() *tugwar.CampaignMission {
				mission := fixedTugWarMission("R3 R7 S6 P8 S8 P4", "S4 P5 R6 S5 P3 R8",
					[3]tugwar.Pile{twPile(1, "S2 P7 R4"), twPile(0, "S9"), twPile(0, "P6")},
					"R1 S1 P1 R2", "P", [2]int{2, 3}, false, 0,
					tugwar.WinByMostPiles, tugwar.CPUPolicyExploitative)
				mission.PlayerPushOneWay = true
				return mission
			}(),
		},
		{
			Index: 12, Name: "The Mirror", Hint: "You and the CPU use the same hero in a best-of-three. Mix your actions so the CPU cannot read you.",
			Variant: "tug-war", RewardMarks: AdvancedMissionMarks,
			TugWar: &tugwar.CampaignMission{
				RefillHands: true, CPUPolicy: tugwar.CPUPolicyAdaptive,
				SeriesGames: 3, SeriesWinsNeeded: 2, MirrorHero: true,
				SeriesPolicies: []tugwar.CPUPolicy{tugwar.CPUPolicyAdaptive},
			},
		},
		{
			Index: 13, Name: "The Gambler's Gambit", Hint: "The CPU may bait you toward pile B. Win by a margin of more than 5 points.",
			Variant: "tug-war", AIHero: "gambler", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("R7 S9 P5 R4 S3 P8", "P9 R8 S5 P4 R6 S7",
				[3]tugwar.Pile{twPile(1, "R5 S4 P6"), twPile(0, "S8"), twPile(0, "R3")},
				"S1 R1 P1 S2 R2", "R", [2]int{2, 2}, false, 0, tugwar.WinByScoreMargin, tugwar.CPUPolicyBluffer),
		},
		{
			Index: 14, Name: "The Gauntlet", Hint: "Keep your hero through three fair games against three CPU personalities. Win two.",
			Variant: "tug-war", RewardMarks: AdvancedMissionMarks,
			TugWar: &tugwar.CampaignMission{
				RefillHands: true, SeriesGames: 3, SeriesWinsNeeded: 2,
				SeriesCPUHeroes: []string{"recruit", "warden", "ascendant"},
				SeriesPolicies: []tugwar.CPUPolicy{
					tugwar.CPUPolicyPassive, tugwar.CPUPolicyPushIt, tugwar.CPUPolicySpreader,
				},
			},
		},
		{
			Index: 15, Name: "Tug of War", Hint: "Final exam: manage your bounty, defend your piles, and overcome the boss's extra push.",
			Variant: "tug-war", AIHero: "hater", RewardMarks: AdvancedMissionMarks,
			TugWar: fixedTugWarMission("R8 S7 P6 R5 R6 P7", "S9 S8 R9 P4 S5 R4",
				[3]tugwar.Pile{twPile(0, "R7"), twPile(0, "P8"), twPile(0, "S6")},
				"R1 S1 P1 R2 S2 P2 R3 S3", "S", [2]int{2, 3}, true, 0, tugwar.WinByScore, tugwar.CPUPolicyEverything),
		},
	}
	return Campaign{
		ID: "tug-war", Name: "Tug War", Description: "Master card play, board control, and CPU tactics across 15 campaign missions.",
		Variant: "tug-war", Missions: missions,
	}
}

func fixedTugWarMission(playerHand, cpuHand string, piles [tugwar.NumPiles]tugwar.Pile,
	deck, bounty string, pushes [2]int, refill bool, maxRounds int,
	win tugwar.MissionWinCondition, policy tugwar.CPUPolicy,
) *tugwar.CampaignMission {
	return &tugwar.CampaignMission{
		PlayerHand: twCards(playerHand), CPUHand: twCards(cpuHand), Piles: piles, Deck: twCards(deck),
		BountySuit: twSuit(bounty), Pushes: pushes, RefillHands: refill, MaxRounds: maxRounds,
		WinCondition: win, CPUPolicy: policy,
	}
}

func twPile(zone int, cards string) tugwar.Pile {
	return tugwar.Pile{Zone: zone, Cards: twCards(cards)}
}

func twHiddenPile(card string) tugwar.Pile {
	pile := twPile(0, card)
	pile.Hidden = true
	return pile
}

func twCards(encoded string) []game.Card {
	if strings.TrimSpace(encoded) == "" {
		return nil
	}
	cards := strings.Fields(encoded)
	result := make([]game.Card, 0, len(cards))
	for _, value := range cards {
		if len(value) < 2 {
			panic(fmt.Sprintf("invalid Tug War campaign card %q", value))
		}
		var suit game.Suit
		switch value[0] {
		case 'S':
			suit = game.Blade
		case 'P':
			suit = game.Fang
		case 'R':
			suit = game.Shield
		default:
			panic(fmt.Sprintf("invalid Tug War campaign suit %q", value))
		}
		face, err := strconv.Atoi(value[1:])
		card := game.Card{Suit: suit, Face: face}
		if err != nil || !card.Valid() {
			panic(fmt.Sprintf("invalid Tug War campaign card %q", value))
		}
		result = append(result, card)
	}
	return result
}

func twSuit(code string) game.Suit {
	switch code {
	case "S":
		return game.Blade
	case "P":
		return game.Fang
	case "R":
		return game.Shield
	default:
		panic(fmt.Sprintf("invalid Tug War campaign suit %q", code))
	}
}
