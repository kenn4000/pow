package heroes

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"prisonersofwar/internal/modifiers"
)

type Hero struct {
	ID      string
	Name    string
	Flavor  string
	Bounty  int
	Shift   int
	Favored int
	Flip    bool
	Unlock  string
	SkinIDs []string
}

type CombinedModifiers struct {
	Bounty  int
	Shift   int
	Favored int
}

var roster = []Hero{
	{ID: "recruit", Name: "The Recruit", Flavor: "Fresh off the boat. Nothing special. Yet.", Unlock: "default"},
	{ID: "warden", Name: "The Warden", Flavor: "Order is the first wall against chaos.", Bounty: 1, Favored: 1, Unlock: "campaign:basic"},
	{ID: "hunter", Name: "The Hunter", Flavor: "Every weakness leaves a trail.", Bounty: 2, Unlock: "campaign:basic"},
	{ID: "treasurer", Name: "The Treasurer", Flavor: "A careful count can turn any battle.", Bounty: -2, Favored: 1, Unlock: "hero_lootbox"},
	{ID: "ascendant", Name: "The Ascendant", Flavor: "Rise above the limits of the hand.", Shift: 2, Unlock: "hero_lootbox"},
	{ID: "anchor", Name: "The Anchor", Flavor: "Hold fast when the tide turns.", Bounty: -1, Favored: -1, Unlock: "campaign:basic"},
	{ID: "gambler", Name: "The Gambler", Flavor: "Fortune favors the bold wager.", Bounty: 2, Shift: 1, Unlock: "hero_lootbox"},
	{ID: "diplomat", Name: "The Diplomat", Flavor: "A quiet bargain changes the odds.", Bounty: -2, Shift: 1, Favored: 1, Unlock: "hero_lootbox"},
	{ID: "ascetic", Name: "The Ascetic", Flavor: "Want less, and endure more.", Bounty: -1, Shift: 1, Favored: -1, Unlock: "hero_lootbox"},
	{ID: "herald", Name: "The Herald", Flavor: "A new reckoning is always at hand.", Bounty: 3, Favored: -1, Unlock: "hero_lootbox"},
	{ID: "hater", Name: "The Hater", Flavor: "Your advantage belongs to me now.", Bounty: 1, Favored: 1, Flip: true, Unlock: "hero_lootbox"},
}

func All() []Hero {
	result := make([]Hero, len(roster))
	for i, hero := range roster {
		result[i] = hero
		result[i].SkinIDs = append([]string(nil), hero.SkinIDs...)
	}
	return result
}

func ByID(id string) (Hero, bool) {
	for _, hero := range roster {
		if hero.ID == id {
			hero.SkinIDs = append([]string(nil), hero.SkinIDs...)
			return hero, true
		}
	}
	return Hero{}, false
}

func Combine(h1, h2 Hero) CombinedModifiers {
	result, _ := CombineWithTuning(h1, h2, nil, nil)
	return result
}

type Tuning map[string]int

func CombineWithTuning(h1, h2 Hero, tuning1, tuning2 Tuning) (CombinedModifiers, error) {
	if err := ValidateTuning(tuning1, 2); err != nil {
		return CombinedModifiers{}, err
	}
	if err := ValidateTuning(tuning2, 2); err != nil {
		return CombinedModifiers{}, err
	}
	tuningBounty, err := tuningValue(tuning1, "bounty")
	if err != nil {
		return CombinedModifiers{}, err
	}
	value, err := tuningValue(tuning2, "bounty")
	if err != nil {
		return CombinedModifiers{}, err
	}
	tuningBounty += value
	tuningShift, err := tuningValue(tuning1, "shift")
	if err != nil {
		return CombinedModifiers{}, err
	}
	value, err = tuningValue(tuning2, "shift")
	if err != nil {
		return CombinedModifiers{}, err
	}
	tuningShift += value
	tuningFavored, err := tuningValue(tuning1, "favored")
	if err != nil {
		return CombinedModifiers{}, err
	}
	value, err = tuningValue(tuning2, "favored")
	if err != nil {
		return CombinedModifiers{}, err
	}
	tuningFavored += value
	firstBounty, secondBounty := h1.Bounty, h2.Bounty
	if h1.Flip && h2.Flip {
		firstBounty, secondBounty = -firstBounty, -secondBounty
	} else if h1.Flip {
		secondBounty = -secondBounty
	} else if h2.Flip {
		firstBounty = -firstBounty
	}
	bounty := firstBounty + secondBounty + tuningBounty
	if h1.ID == "recruit" && h2.ID == "recruit" {
		// The starter mirror match uses a positive default bounty.
		bounty++
	}
	return CombinedModifiers{
		Bounty:  clamp(bounty, -3, 3),
		Shift:   clamp(h1.Shift+h2.Shift+tuningShift, 0, 3),
		Favored: clamp(h1.Favored+h2.Favored+tuningFavored, -1, 2),
	}, nil
}

func tuningValue(tuning Tuning, modifier string) (int, error) {
	for key, value := range tuning {
		if key != "bounty" && key != "shift" && key != "favored" {
			return 0, fmt.Errorf("unsupported tuning modifier %q", key)
		}
		if value < -1 || value > 1 {
			return 0, fmt.Errorf("tuning for %s must be -1, 0, or +1", key)
		}
	}
	return tuning[modifier], nil
}

func ValidateTuning(tuning Tuning, maxModifiers int) error {
	if maxModifiers < 0 {
		return fmt.Errorf("maximum tuned modifier count cannot be negative")
	}
	used := 0
	for key, value := range tuning {
		if key != "bounty" && key != "shift" && key != "favored" {
			return fmt.Errorf("unsupported tuning modifier %q", key)
		}
		if value < -1 || value > 1 {
			return fmt.Errorf("tuning for %s must be -1, 0, or +1", key)
		}
		if value != 0 {
			used++
		}
	}
	if used > maxModifiers {
		return fmt.Errorf("cannot tune more than %d different modifiers", maxModifiers)
	}
	return nil
}

func ChooseForOpponent(unlockedIDs []string, opponentHint string, rng *rand.Rand) (Hero, error) {
	if rng == nil {
		return Hero{}, fmt.Errorf("random source is required")
	}
	unlocked := make([]Hero, 0, len(unlockedIDs))
	for _, id := range unlockedIDs {
		hero, ok := ByID(id)
		if ok {
			unlocked = append(unlocked, hero)
		}
	}
	if len(unlocked) == 0 {
		return Hero{}, fmt.Errorf("no unlocked heroes are available")
	}

	opponent, hasHint := ByID(opponentHint)
	if !hasHint {
		return unlocked[rng.Intn(len(unlocked))], nil
	}
	preferred := make([]string, 0, 2)
	switch opponent.ID {
	case "hunter", "herald":
		preferred = []string{"treasurer", "hater"}
	case "ascendant", "gambler":
		preferred = []string{"anchor", "ascendant"}
	case "treasurer", "diplomat":
		preferred = []string{"hunter", "hater"}
	case "warden", "anchor":
		preferred = []string{"warden", "herald"}
	default:
		return unlocked[rng.Intn(len(unlocked))], nil
	}
	candidates := make([]Hero, 0, len(preferred))
	for _, candidate := range unlocked {
		for _, id := range preferred {
			if candidate.ID == id {
				candidates = append(candidates, candidate)
				break
			}
		}
	}
	if len(candidates) == 0 {
		return unlocked[rng.Intn(len(unlocked))], nil
	}
	return candidates[rng.Intn(len(candidates))], nil
}

func GameModifiersFor(first, second Hero, rng *rand.Rand) (modifiers.GameModifiers, error) {
	combined := Combine(first, second)
	return gameModifiersFromCombined(combined, rng)
}

func GameModifiersForWithTuning(first, second Hero, tuning1, tuning2 Tuning, rng *rand.Rand) (modifiers.GameModifiers, error) {
	combined, err := CombineWithTuning(first, second, tuning1, tuning2)
	if err != nil {
		return modifiers.GameModifiers{}, err
	}
	return gameModifiersFromCombined(combined, rng)
}

func gameModifiersFromCombined(combined CombinedModifiers, rng *rand.Rand) (modifiers.GameModifiers, error) {
	reckoning, err := RollReckoning(rng)
	if err != nil {
		return modifiers.GameModifiers{}, err
	}
	return modifiers.GameModifiers{
		Bounty:    combined.Bounty,
		Shift:     combined.Shift,
		Favored:   combined.Favored,
		Reckoning: reckoning.Weights,
	}, nil
}

func Validate(hero Hero) error {
	if hero.ID == "" || hero.Name == "" {
		return fmt.Errorf("hero ID and name are required")
	}
	if hero.Bounty < -3 || hero.Bounty > 3 || hero.Shift < 0 || hero.Shift > 3 ||
		hero.Favored < -1 || hero.Favored > 2 {
		return fmt.Errorf("hero %q has modifiers outside the allowed envelope", hero.ID)
	}
	switch hero.Unlock {
	case "default", "hero_lootbox", "campaign:basic":
		return nil
	default:
		if strings.HasPrefix(hero.Unlock, "achievement:") && strings.TrimPrefix(hero.Unlock, "achievement:") != "" {
			return nil
		}
		if strings.HasPrefix(hero.Unlock, "marks:") {
			amount, err := strconv.ParseInt(strings.TrimPrefix(hero.Unlock, "marks:"), 10, 64)
			if err == nil && amount > 0 {
				return nil
			}
		}
		return fmt.Errorf("hero %q has unsupported unlock path %q", hero.ID, hero.Unlock)
	}
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
