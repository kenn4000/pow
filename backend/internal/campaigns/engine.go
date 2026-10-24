package campaigns

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/internal/ai"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

type MissionUI interface {
	ShowBriefing(mission Mission)
	ShowStartingHand(playerCards []game.Card)
	ChooseHeroForMission(allowed []string) (string, error)
	ChooseCommit(hand []game.Card, count int) ([]int, error)
	ChoosePoolCard(exchange int, attacking bool, revealed *game.Card, pool []game.Card) (int, error)
	ShowMissionSetup(state game.GameState)
	ShowBattleResult(exchange int, attacker, defender game.Card, outcome game.Outcome, attackerIsPlayer bool)
	ChooseRoundThreeSeat(scores [2]int) (game.Seat, error)
	ShowMissionResult(result MissionResult)
	ChooseMissionTuning() (heroes.Tuning, error)
}

type MissionResult struct {
	Won               bool
	PlayerScore       int
	CPUScore          int
	MissionsCompleted bool
	PlayerHeroID      string
	CPUHeroID         string
	State             game.GameState
	MatchResult       game.MatchResult
	Progression       MissionOutcome
}

func RunMission(p *profile.Profile, mission Mission, rng *rand.Rand, ui MissionUI) (MissionResult, error) {
	if err := validateMissionInputs(p, mission, rng, ui); err != nil {
		return MissionResult{}, err
	}

	ui.ShowBriefing(mission)
	playerHeroID, err := choosePlayerHero(p, mission, ui)
	if err != nil {
		return MissionResult{}, err
	}
	playerHero, ok := heroes.ByID(playerHeroID)
	if !ok {
		return MissionResult{}, fmt.Errorf("mission references unknown player hero %q", playerHeroID)
	}
	cpuHeroID := mission.AIHero
	if cpuHeroID == "" {
		cpuHeroID = "recruit"
	}
	cpuHero, ok := heroes.ByID(cpuHeroID)
	if !ok {
		return MissionResult{}, fmt.Errorf("mission references unknown AI hero %q", cpuHeroID)
	}
	tuning, err := ui.ChooseMissionTuning()
	if err != nil {
		return MissionResult{}, err
	}
	combined, err := heroes.CombineWithTuning(playerHero, cpuHero, tuning, nil)
	if err != nil {
		return MissionResult{}, fmt.Errorf("combine mission hero modifiers: %w", err)
	}
	gameModifiers, err := missionModifiers(mission, combined, rng)
	if err != nil {
		return MissionResult{}, err
	}
	state, err := newMissionState(mission, gameModifiers, rng)
	if err != nil {
		return MissionResult{}, err
	}
	ui.ShowMissionSetup(state)

	playerIndex, cpuIndex := 0, 1
	var scoreAfterRoundTwo [2]int
	var roundWins [3][2]int
	suitDoublingWin := false
	if mission.Type == SingleRound {
		ui.ShowStartingHand(append([]game.Card(nil), state.P1.Hand...))
		indices, err := ui.ChooseCommit(append([]game.Card(nil), state.P1.Hand...), min(3, len(state.P1.Hand)))
		if err != nil {
			return MissionResult{}, err
		}
		if err := game.Commit(&state, playerIndex, indices); err != nil {
			return MissionResult{}, fmt.Errorf("commit player cards: %w", err)
		}
		if err := playMissionRound(&state, mission, ui, rng, 0, playerIndex, cpuIndex, &roundWins, &suitDoublingWin); err != nil {
			return MissionResult{}, err
		}
	} else {
		scoreAfterRoundTwo, err = playThreeRoundMission(&state, mission, ui, rng, &roundWins, &suitDoublingWin)
		if err != nil {
			return MissionResult{}, err
		}
	}

	playerScore := game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total
	cpuScore := game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total
	won := playerScore > cpuScore
	summary := game.MatchGame{
		Won: won, PlayerSeat: state.P1.Seat, PlayerScore: playerScore, OpponentScore: cpuScore,
		PlayerJail:              append([]game.Card(nil), state.P1.Jail...),
		OpponentJail:            append([]game.Card(nil), state.P2.Jail...),
		PlayerCapturedByRound:   state.P1.CapturedByRound,
		OpponentCapturedByRound: state.P2.CapturedByRound,
		Modifiers:               state.Modifiers, PlayerHeroID: playerHeroID, OpponentHeroID: cpuHeroID,
		BountySuit: state.Bounty.Suit, Clashes: state.Clashes,
		ScoreAfterR2: scoreAfterRoundTwo[0], OpponentAfterR2: scoreAfterRoundTwo[1],
		SuitDoublingWin: suitDoublingWin,
	}
	for _, wins := range roundWins {
		if wins[playerIndex] == 3 || wins[cpuIndex] == 3 {
			summary.RoundSweeps++
		}
	}
	matchResult := game.MatchResult{
		Won: won, Tied: playerScore == cpuScore, PlayerSeat: state.P1.Seat,
		PlayerScore: playerScore, OpponentScore: cpuScore, Games: []game.MatchGame{summary},
	}
	result := MissionResult{
		Won: won, PlayerScore: playerScore, CPUScore: cpuScore,
		MissionsCompleted: won, PlayerHeroID: playerHeroID, CPUHeroID: cpuHeroID,
		State: state, MatchResult: matchResult,
	}
	ui.ShowMissionResult(result)
	return result, nil
}

func validateMissionInputs(p *profile.Profile, mission Mission, rng *rand.Rand, ui MissionUI) error {
	if p == nil {
		return errors.New("profile is required")
	}
	if rng == nil {
		return errors.New("random source is required")
	}
	if ui == nil {
		return errors.New("mission UI is required")
	}
	if mission.Type != SingleRound && mission.Type != ThreeRound {
		return fmt.Errorf("mission %d has invalid type %d", mission.Index, mission.Type)
	}
	return nil
}

func playThreeRoundMission(state *game.GameState, mission Mission, ui MissionUI, rng *rand.Rand, roundWins *[3][2]int, suitDoublingWin *bool) ([2]int, error) {
	var scores [2]int
	for state.Round <= 3 {
		if state.Round == 3 {
			if err := chooseMissionSeat(state, ui, rng); err != nil {
				return scores, err
			}
		}
		ui.ShowStartingHand(append([]game.Card(nil), state.P1.Hand...))
		if err := commitMissionRound(state, mission, ui, rng); err != nil {
			return scores, err
		}
		if err := playMissionRound(state, mission, ui, rng, state.Round-1, 0, 1, roundWins, suitDoublingWin); err != nil {
			return scores, err
		}
		if state.Round == 2 {
			scores = [2]int{game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total, game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total}
		}
		if state.Round == 3 {
			break
		}
		if err := game.EndRound(state, state.Round == 1, true); err != nil {
			return scores, err
		}
		state.Round++
	}
	return scores, nil
}

func chooseMissionSeat(state *game.GameState, ui MissionUI, rng *rand.Rand) error {
	scores := [2]int{game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total, game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total}
	chooser := 0
	if scores[1] < scores[0] || (scores[0] == scores[1] && rng.Intn(2) == 1) {
		chooser = 1
	}
	var seat game.Seat
	var err error
	if chooser == 0 {
		seat, err = ui.ChooseRoundThreeSeat(scores)
	} else {
		seat, err = ai.ChooseSeat(*state, chooser, ai.Hard, rng)
	}
	if err != nil {
		return err
	}
	choosingPlayer, _ := state.Player(chooser)
	otherPlayer, _ := state.Player(1 - chooser)
	choosingPlayer.Seat, otherPlayer.Seat = seat, otherSeat(seat)
	return nil
}

func choosePlayerHero(p *profile.Profile, mission Mission, ui MissionUI) (string, error) {
	allowed := make([]string, 0, len(mission.PlayerHeroRestriction))
	if len(mission.PlayerHeroRestriction) == 0 {
		allowed = append(allowed, p.DefaultHeroID)
	} else {
		for _, heroID := range mission.PlayerHeroRestriction {
			if contains(p.UnlockedHeroes, heroID) {
				if _, ok := heroes.ByID(heroID); ok {
					allowed = append(allowed, heroID)
				}
			}
		}
	}
	if len(allowed) == 0 {
		return "", errors.New("no permitted campaign hero is unlocked")
	}
	selected := allowed[0]
	if len(allowed) > 1 {
		selected, err := ui.ChooseHeroForMission(append([]string(nil), allowed...))
		if err != nil {
			return "", err
		}
		if !contains(allowed, selected) {
			return "", fmt.Errorf("mission UI selected unavailable hero %q", selected)
		}
	}
	return selected, nil
}

func missionModifiers(mission Mission, combined heroes.CombinedModifiers, rng *rand.Rand) (game.GameModifiers, error) {
	reckoning, err := heroes.RollReckoning(rng)
	if err != nil {
		return game.GameModifiers{}, err
	}
	result := game.GameModifiers{
		Bounty: combined.Bounty, Shift: combined.Shift, Favored: combined.Favored,
		Reckoning: reckoning.Weights,
	}
	if mission.BountyOverride != nil {
		result.Bounty = *mission.BountyOverride
	}
	if mission.ShiftOverride != nil {
		result.Shift = *mission.ShiftOverride
	}
	if mission.FavoredOverride != nil {
		result.Favored = *mission.FavoredOverride
	}
	if mission.ReckoningOverride != nil {
		result.Reckoning = *mission.ReckoningOverride
	}
	if err := result.Validate(); err != nil {
		return game.GameModifiers{}, fmt.Errorf("mission %q has invalid modifiers: %w", mission.Name, err)
	}
	return result, nil
}

func newMissionState(mission Mission, gameModifiers game.GameModifiers, rng *rand.Rand) (game.GameState, error) {
	state, err := game.NewGameWithModifiers(rng, gameModifiers)
	if err != nil {
		return game.GameState{}, err
	}
	if mission.Type == ThreeRound {
		if len(mission.PlayerHand) > 0 {
			return newFixedThreeRoundState(state, mission, rng)
		}
		if err := applyBountySuitOverride(&state, mission, rng); err != nil {
			return game.GameState{}, err
		}
		return state, nil
	}
	return newScriptedSingleRoundState(state, mission, rng)
}

func newFixedThreeRoundState(state game.GameState, mission Mission, rng *rand.Rand) (game.GameState, error) {
	fullDeck := append([]game.Card(nil), state.Deck...)
	fullDeck = append(fullDeck, state.P1.Hand...)
	fullDeck = append(fullDeck, state.P2.Hand...)
	fullDeck = append(fullDeck, state.Bounty)
	game.Shuffle(fullDeck, rng)
	playerHand := append([]game.Card(nil), mission.PlayerHand...)
	for index, card := range playerHand {
		if !card.Valid() {
			return game.GameState{}, fmt.Errorf("mission %q has invalid player card at index %d", mission.Name, index)
		}
		if err := removeCard(&fullDeck, card); err != nil {
			return game.GameState{}, fmt.Errorf("mission %q player hand: %w", mission.Name, err)
		}
	}
	if mission.BountySuitOverride != nil {
		suit, err := parseSuitCode(*mission.BountySuitOverride)
		if err != nil {
			return game.GameState{}, err
		}
		bountyIndex := randomCardOfSuit(fullDeck, suit, rng)
		if bountyIndex < 0 {
			return game.GameState{}, fmt.Errorf("mission %q cannot deal a bounty card of suit %s", mission.Name, suit)
		}
		state.Bounty = takeCard(&fullDeck, bountyIndex)
	} else {
		state.Bounty = takeCard(&fullDeck, rng.Intn(len(fullDeck)))
	}
	cpuHandSize := campaignHandSize(state.P2.Seat)
	cpuHand := make([]game.Card, 0, cpuHandSize)
	for len(cpuHand) < cpuHandSize {
		cpuHand = append(cpuHand, takeCard(&fullDeck, rng.Intn(len(fullDeck))))
	}
	state.Deck = fullDeck
	state.P1.Hand, state.P1.Pool, state.P1.Jail = playerHand, nil, nil
	state.P2.Hand, state.P2.Pool, state.P2.Jail = cpuHand, nil, nil
	state.P1.CapturedByRound, state.P2.CapturedByRound = [3][3]int{}, [3][3]int{}
	state.Round, state.Clashes = 1, 0
	if err := state.Validate(); err != nil {
		return game.GameState{}, fmt.Errorf("initialize mission %q: %w", mission.Name, err)
	}
	return state, nil
}

func newScriptedSingleRoundState(state game.GameState, mission Mission, rng *rand.Rand) (game.GameState, error) {
	fullDeck := append([]game.Card(nil), state.Deck...)
	fullDeck = append(fullDeck, state.P1.Hand...)
	fullDeck = append(fullDeck, state.P2.Hand...)
	fullDeck = append(fullDeck, state.Bounty)
	game.Shuffle(fullDeck, rng)

	cpuPool := append([]game.Card(nil), mission.CPUPool...)
	for index, lead := range mission.CPULeads {
		if index == mission.CPUMirrorIndex {
			continue
		}
		if !lead.Valid() {
			return game.GameState{}, fmt.Errorf("mission %q has invalid CPU lead at exchange %d", mission.Name, index+1)
		}
		if !containsCard(cpuPool, lead) {
			cpuPool = append(cpuPool, lead)
		}
	}
	if len(cpuPool) > 3 {
		return game.GameState{}, fmt.Errorf("mission %q defines more than three CPU pool cards", mission.Name)
	}

	playerHand := append([]game.Card(nil), mission.PlayerHand...)
	for index, card := range playerHand {
		if !card.Valid() {
			return game.GameState{}, fmt.Errorf("mission %q has invalid player card at index %d", mission.Name, index)
		}
		if err := removeCard(&fullDeck, card); err != nil {
			return game.GameState{}, fmt.Errorf("mission %q player hand: %w", mission.Name, err)
		}
	}
	for index, card := range cpuPool {
		if !card.Valid() {
			return game.GameState{}, fmt.Errorf("mission %q has invalid CPU pool card at index %d", mission.Name, index)
		}
		if err := removeCard(&fullDeck, card); err != nil {
			return game.GameState{}, fmt.Errorf("mission %q CPU pool: %w", mission.Name, err)
		}
	}
	for len(cpuPool) < 3 {
		index := rng.Intn(len(fullDeck))
		cpuPool = append(cpuPool, takeCard(&fullDeck, index))
	}
	if mission.BountySuitOverride != nil {
		suit, err := parseSuitCode(*mission.BountySuitOverride)
		if err != nil {
			return game.GameState{}, err
		}
		bountyIndex := randomCardOfSuit(fullDeck, suit, rng)
		if bountyIndex < 0 {
			return game.GameState{}, fmt.Errorf("mission %q cannot deal a bounty card of suit %s", mission.Name, suit)
		}
		state.Bounty = takeCard(&fullDeck, bountyIndex)
	} else {
		state.Bounty = takeCard(&fullDeck, rng.Intn(len(fullDeck)))
	}
	if len(playerHand) == 0 {
		for len(playerHand) < 6 {
			playerHand = append(playerHand, takeCard(&fullDeck, rng.Intn(len(fullDeck))))
		}
	}
	state.Deck = fullDeck
	state.P1.Hand, state.P1.Pool, state.P1.Jail = playerHand, nil, nil
	state.P2.Hand, state.P2.Pool, state.P2.Jail = nil, cpuPool, nil
	state.P1.CapturedByRound = [3][3]int{}
	state.P2.CapturedByRound = [3][3]int{}
	state.P1.Seat, state.P2.Seat = game.SeatY, game.SeatX
	state.Round, state.Clashes = 1, 0
	if err := state.Validate(); err != nil {
		return game.GameState{}, fmt.Errorf("initialize mission %q: %w", mission.Name, err)
	}
	return state, nil
}

func forceBountySuit(state *game.GameState, suit game.Suit, rng *rand.Rand) error {
	if state.Bounty.Suit == suit {
		return nil
	}
	index := randomCardOfSuit(state.Deck, suit, rng)
	if index >= 0 {
		state.Bounty, state.Deck[index] = state.Deck[index], state.Bounty
		return state.Validate()
	}
	for _, player := range []*game.Player{&state.P1, &state.P2} {
		index = randomCardOfSuit(player.Hand, suit, rng)
		if index >= 0 {
			state.Bounty, player.Hand[index] = player.Hand[index], state.Bounty
			return state.Validate()
		}
	}
	return fmt.Errorf("cannot deal a bounty card of suit %s", suit)
}

func applyBountySuitOverride(state *game.GameState, mission Mission, rng *rand.Rand) error {
	if mission.BountySuitOverride == nil {
		return nil
	}
	suit, err := parseSuitCode(*mission.BountySuitOverride)
	if err != nil {
		return err
	}
	return forceBountySuit(state, suit, rng)
}

func parseSuitCode(code string) (game.Suit, error) {
	switch code {
	case "R":
		return game.Shield, nil
	case "P":
		return game.Fang, nil
	case "S":
		return game.Blade, nil
	default:
		return 0, fmt.Errorf("invalid bounty suit code %q", code)
	}
}

func randomCardOfSuit(cards []game.Card, suit game.Suit, rng *rand.Rand) int {
	indices := make([]int, 0, 9)
	for index, card := range cards {
		if card.Suit == suit {
			indices = append(indices, index)
		}
	}
	if len(indices) == 0 {
		return -1
	}
	return indices[rng.Intn(len(indices))]
}

func removeCard(cards *[]game.Card, target game.Card) error {
	for index, card := range *cards {
		if card == target {
			*cards = append((*cards)[:index], (*cards)[index+1:]...)
			return nil
		}
	}
	return fmt.Errorf("card %s is duplicated or not in the deck", target)
}

func takeCard(cards *[]game.Card, index int) game.Card {
	card := (*cards)[index]
	*cards = append((*cards)[:index], (*cards)[index+1:]...)
	return card
}

func containsCard(cards []game.Card, target game.Card) bool {
	for _, card := range cards {
		if card == target {
			return true
		}
	}
	return false
}

func commitMissionRound(state *game.GameState, mission Mission, ui MissionUI, rng *rand.Rand) error {
	indices, err := ui.ChooseCommit(append([]game.Card(nil), state.P1.Hand...), min(3, len(state.P1.Hand)))
	if err != nil {
		return err
	}
	if err := game.Commit(state, 0, indices); err != nil {
		return fmt.Errorf("commit player cards: %w", err)
	}
	cpuIndices, err := chooseCPUCommit(*state, 1, mission.AILevel, rng)
	if err != nil {
		return fmt.Errorf("choose CPU commit: %w", err)
	}
	if err := game.Commit(state, 1, cpuIndices); err != nil {
		return fmt.Errorf("commit CPU cards: %w", err)
	}
	return nil
}

func playMissionRound(
	state *game.GameState,
	mission Mission,
	ui MissionUI,
	rng *rand.Rand,
	roundIndex int,
	playerIndex int,
	cpuIndex int,
	roundWins *[3][2]int,
	suitDoublingWin *bool,
) error {
	for exchange := 0; exchange < 3; exchange++ {
		attackerIndex, err := state.PlayerAtSeat(game.SeatX)
		if exchange == 1 {
			attackerIndex, err = state.PlayerAtSeat(game.SeatY)
		}
		if err != nil {
			return err
		}
		if mission.Type == SingleRound && mission.CPUAttacksAll {
			attackerIndex = cpuIndex
		}
		defenderIndex := 1 - attackerIndex
		attacker, _ := state.Player(attackerIndex)
		defender, _ := state.Player(defenderIndex)
		if len(attacker.Pool) == 0 || len(defender.Pool) == 0 {
			return fmt.Errorf("mission %q exchange %d has no card available", mission.Name, exchange+1)
		}

		cpuAttacks := attackerIndex == cpuIndex
		mirror := mission.Type == SingleRound && mission.CPUMirrorIndex == exchange
		var attackIndex, defendIndex int
		var attackCard, defendCard game.Card
		if mirror {
			attackCard, defendCard, err = playMissionMirror(state, mission, ui, exchange, attackerIndex, cpuAttacks)
			if err != nil {
				return err
			}
		} else if cpuAttacks {
			attackIndex, err = chooseCPUAttack(*state, mission, attackerIndex, exchange, rng)
			if err != nil {
				return err
			}
			attackCard = attacker.Pool[attackIndex]
			defendIndex, err = choosePlayerPoolCard(ui, exchange, false, &attackCard, defender.Pool)
			if err != nil {
				return err
			}
			defendCard = defender.Pool[defendIndex]
			outcome, err := game.PlayExchange(state, attackerIndex, attackIndex, defenderIndex, defendIndex)
			if err != nil {
				return err
			}
			if err := reportOutcome(ui, state, roundIndex, playerIndex, attackerIndex, exchange, attackCard, defendCard, outcome, roundWins, suitDoublingWin); err != nil {
				return err
			}
			continue
		} else {
			attackIndex, err = choosePlayerPoolCard(ui, exchange, true, nil, attacker.Pool)
			if err != nil {
				return err
			}
			attackCard = attacker.Pool[attackIndex]
			defendIndex, err = chooseCPUDefense(*state, mission, exchange, defenderIndex, &attackCard, rng)
			if err != nil {
				return err
			}
			defendCard = defender.Pool[defendIndex]
			outcome, err := game.PlayExchange(state, attackerIndex, attackIndex, defenderIndex, defendIndex)
			if err != nil {
				return err
			}
			if err := reportOutcome(ui, state, roundIndex, playerIndex, attackerIndex, exchange, attackCard, defendCard, outcome, roundWins, suitDoublingWin); err != nil {
				return err
			}
			continue
		}
		outcome := game.ResolveWithModifiers(attackCard, defendCard, state.Modifiers)
		ui.ShowBattleResult(exchange+1, attackCard, defendCard, outcome, attackerIndex == playerIndex)
		recordMissionOutcome(roundIndex, attackerIndex, outcome, attackCard, defendCard, roundWins, suitDoublingWin)
	}
	return nil
}

func playMissionMirror(state *game.GameState, mission Mission, ui MissionUI, exchange, attackerIndex int, cpuAttacks bool) (game.Card, game.Card, error) {
	attacker, _ := state.Player(attackerIndex)
	defenderIndex := 1 - attackerIndex
	defender, _ := state.Player(defenderIndex)
	var attackIndex, defendIndex int
	var card game.Card
	var err error
	if cpuAttacks {
		defendIndex, err = choosePlayerPoolCard(ui, exchange, false, nil, defender.Pool)
		if err != nil {
			return card, card, err
		}
		card = defender.Pool[defendIndex]
		attackIndex, err = mirrorPhysicalCardIndex(attacker.Pool, mission, exchange)
	} else {
		attackIndex, err = choosePlayerPoolCard(ui, exchange, true, nil, attacker.Pool)
		if err != nil {
			return card, card, err
		}
		card = attacker.Pool[attackIndex]
		defendIndex, err = mirrorPhysicalCardIndex(defender.Pool, mission, exchange)
	}
	if err != nil {
		return card, card, err
	}
	return card, card, playMirrorClash(state, attackerIndex, attackIndex, defenderIndex, defendIndex)
}

func choosePlayerPoolCard(ui MissionUI, exchange int, attacking bool, revealed *game.Card, pool []game.Card) (int, error) {
	index, err := ui.ChoosePoolCard(exchange+1, attacking, revealed, append([]game.Card(nil), pool...))
	if err != nil {
		return 0, err
	}
	if index < 0 || index >= len(pool) {
		return 0, fmt.Errorf("mission UI selected unavailable pool index %d", index)
	}
	return index, nil
}

func chooseCPUCommit(state game.GameState, playerIndex int, level AILevel, rng *rand.Rand) ([]int, error) {
	difficulty, err := aiDifficulty(level, rng)
	if err != nil {
		return nil, err
	}
	return ai.ChooseCommit(state, playerIndex, difficulty, rng)
}

func chooseCPUAttack(state game.GameState, mission Mission, playerIndex, exchange int, rng *rand.Rand) (int, error) {
	player, _ := state.Player(playerIndex)
	if exchange < len(mission.CPULeads) && exchange != mission.CPUMirrorIndex && mission.CPULeads[exchange].Valid() {
		for index, card := range player.Pool {
			if card == mission.CPULeads[exchange] {
				return index, nil
			}
		}
		return 0, fmt.Errorf("scripted CPU lead %s is not in the pool", mission.CPULeads[exchange])
	}
	difficulty, err := aiDifficulty(levelForAttack(mission.AILevel), rng)
	if err != nil {
		return 0, err
	}
	return ai.ChoosePoolIndex(state, playerIndex, true, nil, difficulty, rng)
}

func chooseCPUDefense(state game.GameState, mission Mission, exchange, playerIndex int, attack *game.Card, rng *rand.Rand) (int, error) {
	cpu, _ := state.Player(playerIndex)
	available := make([]int, 0, len(cpu.Pool))
	for index, card := range cpu.Pool {
		neededForLaterLead := false
		for future := exchange + 1; future < len(mission.CPULeads); future++ {
			if future != mission.CPUMirrorIndex && mission.CPULeads[future] == card {
				neededForLaterLead = true
				break
			}
		}
		if !neededForLaterLead {
			available = append(available, index)
		}
	}
	if len(available) == 0 {
		return 0, errors.New("CPU has no card available to defend without using a future scripted lead")
	}
	if len(available) != len(cpu.Pool) {
		filtered := state
		filteredCPU, _ := filtered.Player(playerIndex)
		filteredCPU.Pool = make([]game.Card, len(available))
		for i, index := range available {
			filteredCPU.Pool[i] = cpu.Pool[index]
		}
		difficulty, err := aiDifficulty(levelForDefense(mission.AILevel), rng)
		if err != nil {
			return 0, err
		}
		filteredIndex, err := ai.ChoosePoolIndex(filtered, playerIndex, false, attack, difficulty, rng)
		if err != nil {
			return 0, err
		}
		return available[filteredIndex], nil
	}
	difficulty, err := aiDifficulty(levelForDefense(mission.AILevel), rng)
	if err != nil {
		return 0, err
	}
	return ai.ChoosePoolIndex(state, playerIndex, false, attack, difficulty, rng)
}

func levelForAttack(level AILevel) AILevel {
	if level == AINormal {
		return AIEasy
	}
	return level
}

func levelForDefense(level AILevel) AILevel {
	if level == AINormal {
		return AIGreedy
	}
	return level
}

func aiDifficulty(level AILevel, rng *rand.Rand) (ai.Difficulty, error) {
	switch level {
	case AIEasy:
		if rng.Intn(100) < 30 {
			return ai.Greedy, nil
		}
		return ai.Easy, nil
	case AINormal:
		return ai.Normal, nil
	case AIGreedy:
		return ai.Greedy, nil
	default:
		return 0, fmt.Errorf("unknown AI level %d", level)
	}
}

func mirrorPhysicalCardIndex(cpuPool []game.Card, mission Mission, exchange int) (int, error) {
	for index, physical := range cpuPool {
		futureLead := false
		for future := exchange + 1; future < len(mission.CPULeads); future++ {
			if future != mission.CPUMirrorIndex && mission.CPULeads[future] == physical {
				futureLead = true
				break
			}
		}
		if !futureLead {
			return index, nil
		}
	}
	return 0, errors.New("no physical CPU card is available for the scripted mirror clash")
}

func playMirrorClash(state *game.GameState, attackerIndex, attackIndex, defenderIndex, defendIndex int) error {
	attacker, _ := state.Player(attackerIndex)
	defender, _ := state.Player(defenderIndex)
	attackCard := takeCard(&attacker.Pool, attackIndex)
	defendCard := takeCard(&defender.Pool, defendIndex)
	state.Discard = append(state.Discard, attackCard, defendCard)
	state.Clashes++
	return state.Validate()
}

func reportOutcome(
	ui MissionUI,
	state *game.GameState,
	roundIndex int,
	playerIndex int,
	attackerIndex int,
	exchange int,
	attackCard game.Card,
	defendCard game.Card,
	outcome game.Outcome,
	roundWins *[3][2]int,
	suitDoublingWin *bool,
) error {
	ui.ShowBattleResult(exchange+1, attackCard, defendCard, outcome, attackerIndex == playerIndex)
	recordMissionOutcome(roundIndex, attackerIndex, outcome, attackCard, defendCard, roundWins, suitDoublingWin)
	return state.Validate()
}

func recordMissionOutcome(roundIndex, attackerIndex int, outcome game.Outcome, attackCard, defendCard game.Card, roundWins *[3][2]int, suitDoublingWin *bool) {
	switch outcome.Winner {
	case 0:
		roundWins[roundIndex][attackerIndex]++
		if attackCard.Suit.Beats(defendCard.Suit) {
			*suitDoublingWin = true
		}
	case 1:
		roundWins[roundIndex][1-attackerIndex]++
		if defendCard.Suit.Beats(attackCard.Suit) {
			*suitDoublingWin = true
		}
	}
}

func otherSeat(seat game.Seat) game.Seat {
	if seat == game.SeatX {
		return game.SeatY
	}
	return game.SeatX
}

func campaignHandSize(seat game.Seat) int {
	if seat == game.SeatX {
		return 6
	}
	return 5
}
