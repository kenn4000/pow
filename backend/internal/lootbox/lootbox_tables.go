// Camp Draw tuning: the CPU receives four cards, each draw takes ten wins,
// and at most three draws are stored. Expected box values are approximate
// design targets: wooden ~900 Marks, silver ~3,200, gold ~5,300, diamond
// ~8,300. The expected draw value is ~3,685 Marks (~$0.74), or ~$2.12 per
// ten hours at the specified play pace. Adjust the prize weights below.
package lootbox

import (
	"fmt"
	"prisonersofwar/internal/profile"
)

func LootTableForProfile(boxType string, p *profile.Profile) ([]LootPrize, error) {
	table, err := LootTable(boxType)
	if err != nil {
		return nil, err
	}
	return adjustForLuckyFind(table, p), nil
}

const (
	CPUHandSize     = 4
	WinsPerCampDraw = 10
	MaxDrawsStored  = 3
)

const (
	PrizeMarks  = "marks"
	PrizeCrowns = "crowns"
	PrizeChroma = "chroma"
)

type LootPrize struct {
	Weight int
	Type   string
	Amount int64
	Rarity string
}

func LootTable(boxType string) ([]LootPrize, error) {
	switch boxType {
	case "wooden":
		return []LootPrize{
			{Weight: 60, Type: PrizeMarks, Amount: 200},
			{Weight: 50, Type: PrizeMarks, Amount: 500},
			{Weight: 30, Type: PrizeMarks, Amount: 1000},
			{Weight: 20, Type: PrizeMarks, Amount: 2000},
			{Weight: 10, Type: PrizeMarks, Amount: 5000},
			{Weight: 20, Type: PrizeCrowns, Amount: 50},
			{Weight: 10, Type: PrizeChroma, Rarity: "common"},
		}, nil
	case "silver":
		return []LootPrize{
			{Weight: 50, Type: PrizeMarks, Amount: 800},
			{Weight: 40, Type: PrizeMarks, Amount: 2000},
			{Weight: 30, Type: PrizeMarks, Amount: 4000},
			{Weight: 16, Type: PrizeMarks, Amount: 8000},
			{Weight: 30, Type: PrizeCrowns, Amount: 80},
			{Weight: 10, Type: PrizeCrowns, Amount: 150},
			{Weight: 16, Type: PrizeChroma, Rarity: "common"},
			{Weight: 6, Type: PrizeChroma, Rarity: "uncommon"},
			{Weight: 2, Type: PrizeChroma, Rarity: "rare"},
		}, nil
	case "gold":
		return []LootPrize{
			{Weight: 40, Type: PrizeMarks, Amount: 4000},
			{Weight: 36, Type: PrizeMarks, Amount: 8000},
			{Weight: 24, Type: PrizeMarks, Amount: 12000},
			{Weight: 16, Type: PrizeMarks, Amount: 20000},
			{Weight: 24, Type: PrizeCrowns, Amount: 150},
			{Weight: 10, Type: PrizeCrowns, Amount: 400},
			{Weight: 20, Type: PrizeChroma, Rarity: "common"},
			{Weight: 16, Type: PrizeChroma, Rarity: "uncommon"},
			{Weight: 10, Type: PrizeChroma, Rarity: "rare"},
			{Weight: 3, Type: PrizeChroma, Rarity: "epic"},
			{Weight: 1, Type: PrizeChroma, Rarity: "seasonal"},
		}, nil
	case "diamond":
		return []LootPrize{
			{Weight: 50, Type: PrizeCrowns, Amount: 100},
			{Weight: 40, Type: PrizeCrowns, Amount: 200},
			{Weight: 30, Type: PrizeCrowns, Amount: 300},
			{Weight: 20, Type: PrizeCrowns, Amount: 500},
			{Weight: 10, Type: PrizeCrowns, Amount: 800},
			{Weight: 16, Type: PrizeChroma, Rarity: "common"},
			{Weight: 16, Type: PrizeChroma, Rarity: "uncommon"},
			{Weight: 12, Type: PrizeChroma, Rarity: "rare"},
			{Weight: 4, Type: PrizeChroma, Rarity: "epic"},
			{Weight: 2, Type: PrizeChroma, Rarity: "seasonal"},
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
	default:
		return 0
	}
}
