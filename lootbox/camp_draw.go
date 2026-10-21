package lootbox

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/catalog"
	"prisonersofwar/game"
	"prisonersofwar/profile"
	"prisonersofwar/skills"
)

type BoxOpenResult struct {
	BoxType        string `json:"box_type"`
	Marks          int64  `json:"marks,omitempty"`
	Crowns         int64  `json:"crowns,omitempty"`
	ChromaID       string `json:"chroma_id,omitempty"`
	ChromaRarity   string `json:"chroma_rarity,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

type ExchangeResult struct {
	PlayerCard game.Card   `json:"player_card"`
	CPUCard    game.Card   `json:"cpu_card"`
	Outcome    int         `json:"outcome"`
	PlayerPool []game.Card `json:"player_pool"`
	CPUPool    []game.Card `json:"cpu_pool"`
}

type CampDrawResult struct {
	BoxType    string           `json:"box_type"`
	PlayerWins int              `json:"player_wins"`
	Exchanges  []ExchangeResult `json:"exchanges"`
	Reward     BoxOpenResult    `json:"reward"`
}

type CampDrawUI interface {
	ShowStartingHand(cards []game.Card)
	ChooseCardForExchange(exchangeIndex int, isDefending bool, cpuCard *game.Card, playerPool []game.Card) int
	ShowExchangeResult(ex ExchangeResult)
	ShowBoxResult(boxType string, reward BoxOpenResult)
}

func CPUAttackerPick(pool []game.Card, _ *rand.Rand) game.Card {
	if len(pool) == 0 {
		return game.Card{}
	}
	best := pool[0]
	for _, card := range pool[1:] {
		if card.Face > best.Face || (card.Face == best.Face && card.Suit < best.Suit) {
			best = card
		}
	}
	return best
}

func CPUDefenderPick(pool []game.Card, attack game.Card) game.Card {
	if len(pool) == 0 || !attack.Valid() {
		return game.Card{}
	}
	bestCategory := -1
	var best game.Card
	for _, card := range pool {
		if !card.Valid() {
			continue
		}
		outcome := game.Resolve(attack, card)
		category := 0
		switch outcome.Winner {
		case 1:
			category = 2
		case -1:
			category = 1
		}
		if category > bestCategory ||
			(category == bestCategory && (best.Face == 0 || card.Face < best.Face)) ||
			(category == bestCategory && card.Face == best.Face && card.Suit < best.Suit) {
			bestCategory, best = category, card
		}
	}
	return best
}

func OpenCampDraw(p *profile.Profile, rng *rand.Rand, ui CampDrawUI) (CampDrawResult, error) {
	if p == nil {
		return CampDrawResult{}, errors.New("profile is required")
	}
	if rng == nil {
		return CampDrawResult{}, errors.New("random source is required")
	}
	if ui == nil {
		return CampDrawResult{}, errors.New("Camp Draw UI is required")
	}
	if p.DrawsAvailable < 1 {
		return CampDrawResult{}, errors.New("no Camp Draws are available")
	}
	deck := game.NewDeck()
	game.Shuffle(deck, rng)
	playerPool := append([]game.Card(nil), deck[:3]...)
	cpuPool := append([]game.Card(nil), deck[3:3+CPUHandSize]...)
	ui.ShowStartingHand(append([]game.Card(nil), playerPool...))

	result := CampDrawResult{Exchanges: make([]ExchangeResult, 0, 3)}
	for exchangeIndex := 0; exchangeIndex < 3; exchangeIndex++ {
		var playerCard, cpuCard game.Card
		isDefending := exchangeIndex != 1
		if isDefending {
			cpuCard = CPUAttackerPick(cpuPool, rng)
			cardForUI := cpuCard
			playerIndex := ui.ChooseCardForExchange(exchangeIndex, true, &cardForUI, append([]game.Card(nil), playerPool...))
			if playerIndex < 0 || playerIndex >= len(playerPool) {
				return CampDrawResult{}, fmt.Errorf("invalid player card selection %d for exchange %d", playerIndex, exchangeIndex+1)
			}
			playerCard = playerPool[playerIndex]
		} else {
			playerIndex := ui.ChooseCardForExchange(exchangeIndex, false, nil, append([]game.Card(nil), playerPool...))
			if playerIndex < 0 || playerIndex >= len(playerPool) {
				return CampDrawResult{}, fmt.Errorf("invalid player card selection %d for exchange %d", playerIndex, exchangeIndex+1)
			}
			playerCard = playerPool[playerIndex]
			cpuCard = CPUDefenderPick(cpuPool, playerCard)
			if !cpuCard.Valid() {
				return CampDrawResult{}, errors.New("CPU could not choose a valid defense card")
			}
		}

		attacker, defender := cpuCard, playerCard
		if exchangeIndex == 1 {
			attacker, defender = playerCard, cpuCard
		}
		outcome := game.Resolve(attacker, defender)
		playerWins := outcome.Winner == 1
		if exchangeIndex == 1 {
			playerWins = outcome.Winner == 0
		}
		playerResult := 1
		if outcome.Winner != -1 && !playerWins {
			playerResult = 0
		} else if playerWins {
			playerResult = 2
			result.PlayerWins++
		}
		playerPool = removeCardAt(playerPool, indexOfCard(playerPool, playerCard))
		cpuPool = removeCardAt(cpuPool, indexOfCard(cpuPool, cpuCard))
		exchange := ExchangeResult{
			PlayerCard: playerCard, CPUCard: cpuCard, Outcome: playerResult,
			PlayerPool: append([]game.Card(nil), playerPool...),
			CPUPool:    append([]game.Card(nil), cpuPool...),
		}
		result.Exchanges = append(result.Exchanges, exchange)
		ui.ShowExchangeResult(exchange)
	}

	result.BoxType = boxTypeForWins(result.PlayerWins)
	result.Reward = RollBox(result.BoxType, p, rng)
	applyBoxReward(p, result.Reward)
	ui.ShowBoxResult(result.BoxType, result.Reward)
	p.DrawsAvailable--
	return result, nil
}

func boxTypeForWins(wins int) string {
	switch wins {
	case 3:
		return "diamond"
	case 2:
		return "gold"
	case 1:
		return "silver"
	default:
		return "wooden"
	}
}

func removeCardAt(pool []game.Card, index int) []game.Card {
	copy(pool[index:], pool[index+1:])
	return pool[:len(pool)-1]
}

func indexOfCard(pool []game.Card, target game.Card) int {
	for index, card := range pool {
		if card == target {
			return index
		}
	}
	return -1
}

func RollBox(boxType string, p *profile.Profile, rng *rand.Rand) BoxOpenResult {
	result := BoxOpenResult{BoxType: boxType}
	if p == nil {
		result.FallbackReason = "profile is required"
		return result
	}
	if rng == nil {
		result.FallbackReason = "random source is required"
		return result
	}
	table, err := LootTable(boxType)
	if err != nil {
		result.FallbackReason = err.Error()
		return result
	}
	table = adjustForLuckyFind(table, p)
	totalWeight := 0
	for _, prize := range table {
		totalWeight += prize.Weight
	}
	roll := rng.Intn(totalWeight)
	var selected LootPrize
	for _, prize := range table {
		if roll < prize.Weight {
			selected = prize
			break
		}
		roll -= prize.Weight
	}
	if selected.Type != PrizeChroma {
		switch selected.Type {
		case PrizeMarks:
			result.Marks = selected.Amount
		case PrizeCrowns:
			result.Crowns = selected.Amount
		}
		return result
	}

	chroma, fallback := rollChroma(selected.Rarity, p, rng)
	if chroma != nil {
		result.ChromaID = chroma.ID
		result.ChromaRarity = chroma.Rarity
		return result
	}
	result.Marks = rarityFallbackMarks(selected.Rarity)
	result.FallbackReason = fallback
	return result
}

func adjustForLuckyFind(table []LootPrize, p *profile.Profile) []LootPrize {
	result := append([]LootPrize(nil), table...)
	if skills.AggregateBonuses(p).RareRewardPercent == 0 {
		return result
	}
	rareIndex, currencyIndex := -1, -1
	for index, prize := range result {
		if prize.Type == PrizeChroma && prize.Rarity == "rare" {
			rareIndex = index
		}
		if prize.Type == PrizeMarks || prize.Type == PrizeCrowns {
			if currencyIndex < 0 || prize.Amount > result[currencyIndex].Amount {
				currencyIndex = index
			}
		}
	}
	if rareIndex < 0 || currencyIndex < 0 {
		return result
	}
	shift := 10
	if result[currencyIndex].Weight < shift {
		shift = result[currencyIndex].Weight
	}
	result[currencyIndex].Weight -= shift
	result[rareIndex].Weight += shift
	return result
}

func rollChroma(rarity string, p *profile.Profile, rng *rand.Rand) (*catalog.Chroma, string) {
	matches := chromasForRarity(rarity)
	if len(matches) == 0 {
		return nil, "no chromas exist at this rarity"
	}
	first := matches[rng.Intn(len(matches))]
	if !skills.AggregateBonuses(p).Curator || !p.OwnedChromas[first.ID] {
		return &first, ""
	}
	second := matches[rng.Intn(len(matches))]
	if !p.OwnedChromas[second.ID] {
		return &second, ""
	}
	return nil, "Curator rerolled an owned chroma twice"
}

func chromasForRarity(rarity string) []catalog.Chroma {
	var matches []catalog.Chroma
	for _, chroma := range catalog.AllChromas() {
		if chroma.Rarity == rarity {
			matches = append(matches, chroma)
		}
	}
	return matches
}

func applyBoxReward(p *profile.Profile, reward BoxOpenResult) {
	p.Marks += reward.Marks
	p.Crowns += reward.Crowns
	if reward.ChromaID != "" {
		if p.OwnedChromas == nil {
			p.OwnedChromas = make(map[string]bool)
		}
		if p.SeenChromas == nil {
			p.SeenChromas = make(map[string]bool)
		}
		p.OwnedChromas[reward.ChromaID] = true
		p.SeenChromas[reward.ChromaID] = false
	}
}

func OpenPendingBox(p *profile.Profile, boxType string, rng *rand.Rand) (BoxOpenResult, error) {
	if p == nil {
		return BoxOpenResult{}, errors.New("profile is required")
	}
	if _, err := LootTable(boxType); err != nil {
		return BoxOpenResult{}, err
	}
	if rng == nil {
		return BoxOpenResult{}, errors.New("random source is required")
	}
	if p.PendingBoxes[boxType] <= 0 {
		return BoxOpenResult{}, fmt.Errorf("no pending %s boxes are available", boxType)
	}
	reward := RollBox(boxType, p, rng)
	if reward.FallbackReason != "" && reward.Marks == 0 && reward.Crowns == 0 && reward.ChromaID == "" {
		return BoxOpenResult{}, fmt.Errorf("roll %s box: %s", boxType, reward.FallbackReason)
	}
	p.PendingBoxes[boxType]--
	applyBoxReward(p, reward)
	return reward, nil
}

func RequiredWins(p *profile.Profile) int {
	return skills.CampDrawThreshold(p, WinsPerCampDraw)
}

func AuditConfig() map[string]any {
	result := map[string]any{
		"cpu_hand_size": CPUHandSize, "wins_per_camp_draw": WinsPerCampDraw,
		"max_draws_stored": MaxDrawsStored, "loot_tables": make(map[string][]LootPrize),
	}
	tables := result["loot_tables"].(map[string][]LootPrize)
	for _, boxType := range []string{"wooden", "silver", "gold", "diamond"} {
		tables[boxType], _ = LootTable(boxType)
	}
	return result
}

type HeadlessCampDrawUI struct{}

func (HeadlessCampDrawUI) ShowStartingHand([]game.Card)        {}
func (HeadlessCampDrawUI) ShowExchangeResult(ExchangeResult)   {}
func (HeadlessCampDrawUI) ShowBoxResult(string, BoxOpenResult) {}

func (HeadlessCampDrawUI) ChooseCardForExchange(_ int, isDefending bool, cpuCard *game.Card, playerPool []game.Card) int {
	if len(playerPool) == 0 {
		return -1
	}
	if isDefending && cpuCard != nil {
		selected := CPUDefenderPick(playerPool, *cpuCard)
		return indexOfCard(playerPool, selected)
	}
	selected := CPUAttackerPick(playerPool, nil)
	return indexOfCard(playerPool, selected)
}
