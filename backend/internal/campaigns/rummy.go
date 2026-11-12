package campaigns

import (
	"fmt"
	"strconv"
	"strings"

	"prisonersofwar/internal/game"
)

func rummyCampaign() Campaign {
	missions := []Mission{
		rummyMission(1, "First Steps", "Learn to build runs and sets before worrying about deadwood.", "recruit",
			rummyRules(rummyParams(6, 0, 0, 0, 6), "S3 S4 S5 R7 P7 S7 R2",
				"P1 P2 P3 R6 R8 R9 P4", "", "", "", 12, "score", 5, "random", 1)),
		rummyMission(2, "The Take", "The face-up S5 completes your run; take it instead of drawing two.", "recruit",
			rummyRules(rummyParams(6, 0, 0, 0, 6), "S3 S4 S6 R7 P7 S7 S2",
				"R1 R2 R3 P4 P6 P8 P9", "", "S5", "S5", 12, "win", 0, "passive", 1)),
		rummyMission(3, "Dig Deep", "The stock is stacked with run cards. Draw two when the discard does not help.", "recruit",
			rummyRules(rummyParams(6, 0, 0, 0, 6), "R1 R3 R5 R7 R9 P2 P4",
				"", "R2 R4 R6 R8", "", "", 12, "score", 3, "passive", 1)),
		rummyMission(4, "Deadweight", "Meld the run and set, then discard the high deadwood.", "recruit",
			rummyRules(rummyParams(6, 0, 0, 0, 6), "S1 S2 S3 R9 P9 S9 P8",
				"", "", "", "", 12, "score", 8, "standard", 1)),
		rummyMission(5, "The Race", "Eight total turns: take the sure run and do not chase the diamond pair.", "recruit",
			rummyRules(rummyParams(6, 0, 0, 0, 6), "S2 S3 S4 P5 P6 R9 R1",
				"", "", "", "", 8, "score", 2, "standard", 1)),
		rummyMission(6, "Favor", "Favor raises set value. Use the two ready-made sets to reach the target.", "warden",
			rummyRules(rummyParams(8, 0, 1, 0, 7), "R3 P3 S3 R6 P6 S6 P9",
				"", "", "", "", 12, "score", 20, "standard", 1)),
		rummyMission(7, "Shift", "Shift raises meld points and deadwood penalties; complete both melds.", "ascendant",
			rummyRules(rummyParams(6, 3, 0, -1, 5), "S7 S8 S9 R4 P4 S4",
				"", "", "", "", 12, "score", 15, "standard", 1)),
		rummyMission(8, "Bounty", "A high bounty makes the four-card run more valuable.", "herald",
			rummyRules(rummyParams(6, 0, 3, -1, 5), "S4 S5 S6 S7 P2",
				"", "", "", "", 12, "score", 12, "standard", 1)),
		rummyMission(9, "Draft", "Manage the larger hand: meld the run and set while shedding deadwood.", "gambler",
			rummyRules(rummyParams(6, 1, 2, 2, 8), "S2 S3 S4 R6 P6 S6 R8 R9",
				"", "", "", "", 12, "score", 15, "standard", 1)),
		rummyMission(10, "The Fork", "The face-up S3 is bait; draw two for cards that really build your hand.", "recruit",
			rummyRules(rummyParams(6, 0, 2, 0, 7), "S5 S6 S7 R2 R3 P9 P1",
				"P4 P5 P6 R8 P8 S8 S2", "", "S3", "S3", 12, "score", 5, "standard", 1)),
		rummyMission(11, "Pressure", "Avoid discarding the cards that complete the CPU's run or set.", "recruit",
			rummyRules(rummyParams(6, 1, 2, 1, 7), "R3 R4 R5 S8 P8 R8 P1",
				"S2 S6 R9 P9 S9 S4 S5", "", "", "", 12, "score", 5, "aggressive", 1)),
		rummyMission(12, "The Comeback", "Recover from a scattered hand against an opponent with two ready melds.", "recruit",
			rummyRules(rummyParams(6, 0, 0, 0, 7), "R1 R3 R5 R7 R9 P2 P4",
				"S2 S3 S4 R6 P6 S6 S8", "", "", "", 12, "win", 0, "standard", 1)),
		rummyMission(13, "The Set Trap", "The run is complete; do not hold a pair in hope of a third card.", "treasurer",
			rummyRules(rummyParams(7, 0, -2, 0, 7), "R5 S5 P6 P7 P8 S9 S1",
				"", "", "", "", 12, "score", 10, "standard", 1)),
		rummyMission(14, "The Long Game", "Plan across sixteen turns and force the stock to reshuffle when needed.", "recruit",
			rummyRules(rummyParams(6, 0, 0, 0, 6), "", "", "", "", "", 16, "score", 10, "standard", 1)),
		rummyMission(15, "The Final Test", "Choose a hero, adapt to a random opponent, and win the two-hand match.", "",
			rummyRules(game.RummyParameters{}, "", "", "", "", "", 12, "match", 0, "hard", 2)),
	}
	missions[14].AIHero = "random"
	return Campaign{
		ID: "rummy", Name: "Rummy", Description: "Learn melds, scoring, and strategy across 15 campaign missions.",
		Variant: "rummy", Missions: missions,
	}
}

func rummyMission(index int, name, hint, heroID string, rules *RummyCampaignRules) Mission {
	mission := Mission{
		Index: index, Name: name, Hint: hint, Variant: "rummy", AIHero: heroID,
		RewardMarks: AdvancedMissionMarks, Rummy: rules,
	}
	if heroID != "" {
		mission.PlayerHeroRestriction = []string{heroID}
	}
	return mission
}

func rummyParams(j, k, b, d, handSize int) game.RummyParameters {
	return game.RummyParameters{J: j, K: k, B: b, D: d, HandSize: handSize}
}

func rummyRules(params game.RummyParameters, playerHand, cpuHand, drawTop, faceUp, scriptedDiscard string,
	turns int, goal string, target int, policy string, matchHands int,
) *RummyCampaignRules {
	var scripted *game.Card
	if scriptedDiscard != "" {
		card := campaignRummyCards(scriptedDiscard)[0]
		scripted = &card
	}
	return &RummyCampaignRules{
		Parameters: params, PlayerHand: campaignRummyCards(playerHand),
		CPUHand: campaignRummyCards(cpuHand), DrawPileTop: campaignRummyCards(drawTop),
		InitialFaceUp: campaignRummyCards(faceUp), ScriptedDiscard: scripted,
		TurnLimit: turns, Goal: goal, Target: target, CPUPolicy: policy, MatchHands: matchHands,
	}
}

func campaignRummyCards(encoded string) []game.Card {
	if strings.TrimSpace(encoded) == "" {
		return nil
	}
	cards := make([]game.Card, 0, len(strings.Fields(encoded)))
	for _, item := range strings.Fields(encoded) {
		if len(item) < 2 {
			panic(fmt.Sprintf("invalid campaign Rummy card %q", item))
		}
		face, err := strconv.Atoi(item[1:])
		if err != nil {
			panic(fmt.Sprintf("invalid campaign Rummy card %q: %v", item, err))
		}
		var suit game.Suit
		switch item[0] {
		case 'R':
			suit = game.Shield
		case 'P':
			suit = game.Fang
		case 'S':
			suit = game.Blade
		default:
			panic(fmt.Sprintf("invalid campaign Rummy suit in %q", item))
		}
		card := game.Card{Suit: suit, Face: face}
		if !card.Valid() {
			panic(fmt.Sprintf("invalid campaign Rummy card %q", item))
		}
		cards = append(cards, card)
	}
	return cards
}
