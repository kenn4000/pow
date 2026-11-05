// Camp Draw tuning: the CPU receives four cards, each draw takes ten wins,
// and at most three draws are stored. Box prize weights are percentages.
package lootbox

import (
	"fmt"
	"prisonersofwar/internal/profile"
)

func LootTableForProfile(boxType string, _ *profile.Profile) ([]LootPrize, error) {
	return LootTable(boxType)
}

const (
	CPUHandSize     = 4
	WinsPerCampDraw = 10
	MaxDrawsStored  = 3
)

const (
	PrizeMarks  = "marks"
	PrizeCrowns = "crowns"
	PrizeEnergy = "energy"
	PrizeChroma = "chroma"
)

type LootPrize struct {
	Weight      int
	Type        string
	Amount      int64
	BonusCrowns int64
	Rarity      string
}

func LootTable(boxType string) ([]LootPrize, error) {
	switch boxType {
	case "wooden":
		return []LootPrize{
			{Weight: 38, Type: PrizeMarks, Amount: 55, BonusCrowns: 1},
			{Weight: 34, Type: PrizeMarks, Amount: 115, BonusCrowns: 1},
			{Weight: 17, Type: PrizeMarks, Amount: 240, BonusCrowns: 1},
			{Weight: 3, Type: PrizeMarks, Amount: 500, BonusCrowns: 1},
			{Weight: 1, Type: PrizeMarks, Amount: 1000, BonusCrowns: 1},
			{Weight: 5, Type: PrizeEnergy, Amount: 1, BonusCrowns: 1},
			{Weight: 2, Type: PrizeChroma, Rarity: "common"},
		}, nil
	case "silver":
		return []LootPrize{
			{Weight: 32, Type: PrizeMarks, Amount: 105, BonusCrowns: 2},
			{Weight: 32, Type: PrizeMarks, Amount: 210, BonusCrowns: 2},
			{Weight: 18, Type: PrizeMarks, Amount: 420, BonusCrowns: 2},
			{Weight: 4, Type: PrizeMarks, Amount: 850, BonusCrowns: 2},
			{Weight: 2, Type: PrizeMarks, Amount: 1500, BonusCrowns: 2},
			{Weight: 8, Type: PrizeEnergy, Amount: 1, BonusCrowns: 2},
			{Weight: 3, Type: PrizeChroma, Rarity: "common"},
			{Weight: 1, Type: PrizeChroma, Rarity: "uncommon"},
		}, nil
	case "gold":
		return []LootPrize{
			{Weight: 32, Type: PrizeMarks, Amount: 115, BonusCrowns: 4},
			{Weight: 27, Type: PrizeMarks, Amount: 240, BonusCrowns: 4},
			{Weight: 18, Type: PrizeMarks, Amount: 490, BonusCrowns: 4},
			{Weight: 4, Type: PrizeMarks, Amount: 1000, BonusCrowns: 4},
			{Weight: 1, Type: PrizeMarks, Amount: 2200, BonusCrowns: 4},
			{Weight: 8, Type: PrizeEnergy, Amount: 2, BonusCrowns: 4},
			{Weight: 5, Type: PrizeChroma, Rarity: "common"},
			{Weight: 3, Type: PrizeChroma, Rarity: "uncommon"},
			{Weight: 2, Type: PrizeChroma, Rarity: "rare"},
		}, nil
	case "diamond":
		return []LootPrize{
			{Weight: 50, Type: PrizeMarks, Amount: 500, BonusCrowns: 8},
			{Weight: 25, Type: PrizeMarks, Amount: 1000, BonusCrowns: 8},
			{Weight: 9, Type: PrizeMarks, Amount: 2000, BonusCrowns: 8},
			{Weight: 5, Type: PrizeMarks, Amount: 4000, BonusCrowns: 8},
			{Weight: 2, Type: PrizeEnergy, Amount: 10, BonusCrowns: 8},
			{Weight: 5, Type: PrizeChroma, Rarity: "rare"},
			{Weight: 3, Type: PrizeChroma, Rarity: "epic"},
			{Weight: 1, Type: PrizeChroma, Rarity: "seasonal_or_premium"},
		}, nil
	default:
		return nil, fmt.Errorf("invalid box type %q", boxType)
	}
}

func rarityFallbackMarks(rarity string) int64 {
	switch rarity {
	case "common":
		return 5000
	case "uncommon":
		return 12000
	case "rare":
		return 25000
	case "epic":
		return 50000
	case "seasonal":
		return 30000
	case "seasonal_or_premium":
		return 30000
	default:
		return 0
	}
}
