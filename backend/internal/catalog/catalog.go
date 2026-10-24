package catalog

import (
	"errors"
	"fmt"

	"prisonersofwar/internal/profile"
)

type Chroma struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Rarity   string `json:"rarity"`
	Cost     int64  `json:"cost"`
	Currency string `json:"currency"`
	Bundle   string `json:"bundle,omitempty"`
}

type Bundle struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ChromaIDs  []string `json:"chroma_ids"`
	CostCrowns int64    `json:"cost_crowns"`
}

type Currency string

const (
	Marks  Currency = "marks"
	Crowns Currency = "crowns"
)

var chromas = []Chroma{
	{ID: "crimson", Name: "Tribal (red)", Color: "red", Rarity: "common", Cost: 250, Currency: string(Marks)},
	{ID: "cobalt", Name: "Tribal (blue)", Color: "blue", Rarity: "common", Cost: 250, Currency: string(Marks)},
	{ID: "sunflower", Name: "Tribal (yellow)", Color: "yellow", Rarity: "common", Cost: 250, Currency: string(Marks)},
	{ID: "conq-lime", Name: "Conquistador (lime green)", Color: "lime", Rarity: "common", Cost: 100, Currency: string(Marks)},
	{ID: "conq-gold", Name: "Conquistador (gold)", Color: "gold", Rarity: "common", Cost: 100, Currency: string(Marks)},
	{ID: "conq-pink", Name: "Conquistador (pink)", Color: "pink", Rarity: "common", Cost: 100, Currency: string(Marks)},
	{ID: "conq-blue", Name: "Conquistador (sky blue)", Color: "sky blue", Rarity: "common", Cost: 100, Currency: string(Marks)},
	{ID: "emerald", Name: "Tribal (green)", Color: "green", Rarity: "uncommon", Cost: 15, Currency: string(Crowns)},
	{ID: "tangerine", Name: "Tribal (orange)", Color: "orange", Rarity: "uncommon", Cost: 15, Currency: string(Crowns)},
	{ID: "amethyst", Name: "Tribal (purple)", Color: "purple", Rarity: "uncommon", Cost: 20, Currency: string(Crowns)},
	{ID: "walnut", Name: "Walnut", Color: "brown", Rarity: "rare", Cost: 30, Currency: string(Crowns)},
	{ID: "silver", Name: "Silver", Color: "silver", Rarity: "rare", Cost: 30, Currency: string(Crowns)},
	{ID: "rose-gold", Name: "Rose Gold", Color: "rose-gold", Rarity: "rare", Cost: 35, Currency: string(Crowns)},
	{ID: "aurora", Name: "Aurora", Color: "aurora", Rarity: "epic", Cost: 50, Currency: string(Crowns)},
	{ID: "obsidian", Name: "Obsidian", Color: "obsidian", Rarity: "epic", Cost: 50, Currency: string(Crowns)},
	{ID: "prismatic", Name: "Prismatic", Color: "prismatic", Rarity: "legendary", Cost: 80, Currency: string(Crowns)},
	{ID: "ember-dog", Name: "Ember Dog", Color: "orange", Rarity: "seasonal", Currency: "event"},
}

var bundles = []Bundle{
	{ID: "woodland", Name: "Woodland Bundle", ChromaIDs: []string{"emerald", "walnut"}, CostCrowns: 40},
	{ID: "spectrum", Name: "Spectrum Bundle", ChromaIDs: []string{"amethyst", "tangerine", "silver"}, CostCrowns: 55},
}

func AllChromas() []Chroma {
	return append([]Chroma(nil), chromas...)
}

func AllBundles() []Bundle {
	result := make([]Bundle, len(bundles))
	for i, bundle := range bundles {
		result[i] = bundle
		result[i].ChromaIDs = append([]string(nil), bundle.ChromaIDs...)
	}
	return result
}

func FindChroma(id string) (Chroma, bool) {
	for _, chroma := range chromas {
		if chroma.ID == id {
			return chroma, true
		}
	}
	return Chroma{}, false
}

func BuyChroma(p *profile.Profile, id string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	chroma, ok := FindChroma(id)
	if !ok {
		return fmt.Errorf("unknown chroma %q", id)
	}
	if p.OwnedChromas[id] {
		return errors.New("chroma is already owned")
	}
	switch Currency(chroma.Currency) {
	case Marks:
		if p.Marks < chroma.Cost {
			return fmt.Errorf("not enough Marks: need %d", chroma.Cost)
		}
		p.Marks -= chroma.Cost
	case Crowns:
		if p.Crowns < chroma.Cost {
			return fmt.Errorf("not enough Crowns: need %d", chroma.Cost)
		}
		p.Crowns -= chroma.Cost
	case Currency("event"):
		return errors.New("seasonal chromas are event-only and cannot be purchased directly")
	default:
		return fmt.Errorf("unsupported currency %q for chroma %q", chroma.Currency, id)
	}
	p.OwnedChromas[id] = true
	p.SeenChromas[id] = false
	recordPurchase(p, chroma)
	return nil
}

func BuyBundle(p *profile.Profile, id string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	var bundle Bundle
	found := false
	for _, candidate := range bundles {
		if candidate.ID == id {
			bundle, found = candidate, true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown bundle %q", id)
	}
	if p.PurchasedBundles[id] {
		return nil
	}
	bundleChromas := make([]Chroma, 0, len(bundle.ChromaIDs))
	for _, chromaID := range bundle.ChromaIDs {
		chroma, ok := FindChroma(chromaID)
		if !ok {
			return fmt.Errorf("bundle %q references unknown chroma %q", id, chromaID)
		}
		bundleChromas = append(bundleChromas, chroma)
	}
	if p.Crowns < bundle.CostCrowns {
		return fmt.Errorf("not enough Crowns: need %d", bundle.CostCrowns)
	}
	p.Crowns -= bundle.CostCrowns
	p.PurchasedBundles[id] = true
	for _, chroma := range bundleChromas {
		chromaID := chroma.ID
		if !p.OwnedChromas[chromaID] {
			p.OwnedChromas[chromaID] = true
			p.SeenChromas[chromaID] = false
			recordPurchase(p, chroma)
		}
	}
	return nil
}

func RedeemChromaPick(p *profile.Profile, chromaID string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	if p.ChromaPickTokens < 1 {
		return errors.New("no Pick Any Chroma tokens are available")
	}
	_, ok := FindChroma(chromaID)
	if !ok {
		return fmt.Errorf("unknown chroma %q", chromaID)
	}
	if p.OwnedChromas[chromaID] {
		return errors.New("chroma is already owned")
	}
	p.ChromaPickTokens--
	p.OwnedChromas[chromaID] = true
	p.SeenChromas[chromaID] = false
	return nil
}

func AssignChroma(p *profile.Profile, suit, chromaID string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	requestedSuit := suit
	suit = normalizeSuit(suit)
	if suit == "" {
		return fmt.Errorf("invalid suit %q: use B, F, or S", requestedSuit)
	}
	chroma, ok := FindChroma(chromaID)
	if !ok || !p.OwnedChromas[chromaID] {
		return fmt.Errorf("chroma %q is not owned", chromaID)
	}
	for otherSuit, assigned := range p.AssignedChromas {
		if otherSuit != suit {
			other, found := FindChroma(assigned)
			if found && other.Color == chroma.Color {
				return fmt.Errorf("color %q is already assigned to suit %s", chroma.Color, otherSuit)
			}
			if !found && assigned == chroma.Color {
				return fmt.Errorf("color %q is already assigned to suit %s", chroma.Color, otherSuit)
			}
		}
	}
	p.AssignedChromas[suit] = chroma.ID
	return nil
}

func MarkChromaSeen(p *profile.Profile, chromaID string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	if _, ok := FindChroma(chromaID); !ok {
		return fmt.Errorf("unknown chroma %q", chromaID)
	}
	if !p.OwnedChromas[chromaID] {
		return fmt.Errorf("chroma %q is not owned", chromaID)
	}
	p.SeenChromas[chromaID] = true
	return nil
}

func ChromaForSuit(p *profile.Profile, suit string) (Chroma, bool) {
	if p == nil {
		return Chroma{}, false
	}
	suit = normalizeSuit(suit)
	if suit == "" {
		return Chroma{}, false
	}
	id := p.AssignedChromas[suit]
	if chroma, ok := FindChroma(id); ok && p.OwnedChromas[id] {
		return chroma, true
	}
	return Chroma{}, false
}

func normalizeSuit(suit string) string {
	switch suit {
	case "B", "b", "Blade", "blade", "Scissors", "scissors":
		return "B"
	case "F", "f", "Fang", "fang", "Paper", "paper":
		return "F"
	case "S", "s", "Shield", "shield", "Rock", "rock":
		return "S"
	default:
		return ""
	}
}

func recordPurchase(p *profile.Profile, chroma Chroma) {
	p.TotalChromasPurchased++
	switch chroma.Rarity {
	case "rare":
		p.RareChromasPurchased++
	case "epic", "legendary", "seasonal", "super-rare":
		p.SuperRareChromasPurchased++
	}
}
