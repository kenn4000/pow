package campaigns

import (
	"fmt"
	"math/rand"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func PrepareMission(p *profile.Profile, mission Mission, heroID string, rng *rand.Rand) (game.GameState, string, error) {
	if p == nil || rng == nil {
		return game.GameState{}, "", fmt.Errorf("profile and random source are required")
	}
	if heroID == "" {
		heroID = p.DefaultHeroID
	}
	if !contains(p.UnlockedHeroes, heroID) || (len(mission.PlayerHeroRestriction) > 0 && !contains(mission.PlayerHeroRestriction, heroID)) {
		return game.GameState{}, "", fmt.Errorf("choose an unlocked hero permitted for this mission")
	}
	hero, ok := heroes.ByID(heroID)
	if !ok {
		return game.GameState{}, "", fmt.Errorf("unknown player hero")
	}
	cpuID := mission.AIHero
	if cpuID == "" {
		cpuID = "recruit"
	}
	cpu, ok := heroes.ByID(cpuID)
	if !ok {
		return game.GameState{}, "", fmt.Errorf("unknown mission opponent")
	}
	mods, err := missionModifiers(mission, heroes.Combine(hero, cpu), rng)
	if err != nil {
		return game.GameState{}, "", err
	}
	state, err := newMissionState(mission, mods, rng)
	return state, cpuID, err
}

func ChooseCommit(state game.GameState, mission Mission, rng *rand.Rand) ([]int, error) {
	return chooseCPUCommit(state, 1, mission.AILevel, rng)
}

func ChooseAttack(state game.GameState, mission Mission, exchange int, rng *rand.Rand) (int, error) {
	if mission.Type == SingleRound && mission.CPUMirrorIndex == exchange {
		return mirrorPhysicalCardIndex(state.P2.Pool, mission, exchange)
	}
	return chooseCPUAttack(state, mission, 1, exchange, rng)
}

func ChooseDefense(state game.GameState, mission Mission, exchange int, attack game.Card, rng *rand.Rand) (int, error) {
	if mission.Type == SingleRound && mission.CPUMirrorIndex == exchange {
		return mirrorPhysicalCardIndex(state.P2.Pool, mission, exchange)
	}
	return chooseCPUDefense(state, mission, exchange, 1, &attack, rng)
}

func PlayExchange(state *game.GameState, mission Mission, exchange, attacker, attackIndex, defenseIndex int) (game.Card, game.Card, game.Outcome, error) {
	first, err := state.Player(attacker)
	if err != nil {
		return game.Card{}, game.Card{}, game.Outcome{}, err
	}
	second, _ := state.Player(1 - attacker)
	if attackIndex < 0 || attackIndex >= len(first.Pool) || defenseIndex < 0 || defenseIndex >= len(second.Pool) {
		return game.Card{}, game.Card{}, game.Outcome{}, game.ErrInvalidPoolCard
	}
	attack, defense := first.Pool[attackIndex], second.Pool[defenseIndex]
	if mission.Type == SingleRound && mission.CPUMirrorIndex == exchange {
		if attacker == 1 {
			attack = defense
		} else {
			defense = attack
		}
		outcome := game.ResolveWithModifiers(attack, defense, state.Modifiers)
		return attack, defense, outcome, playMirrorClash(state, attacker, attackIndex, 1-attacker, defenseIndex)
	}
	outcome, err := game.PlayExchange(state, attacker, attackIndex, 1-attacker, defenseIndex)
	return attack, defense, outcome, err
}
