package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skins"
)

type Chroma struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Skin       string `json:"skin,omitempty"`
	HexColor   string `json:"hexColor,omitempty"`
	Rarity     string `json:"rarity"`
	RarityRank int    `json:"rarityRank"`
	Cost       int64  `json:"cost"`
	Currency   string `json:"currency"`
}

type Currency string

const (
	Marks  Currency = "marks"
	Crowns Currency = "crowns"
)

var chromas []Chroma

var (
	skinID       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	skinHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func init() {
	discovered, err := loadSkinChromas(skins.Files)
	if err != nil {
		panic(fmt.Errorf("load skin shop catalog: %w", err))
	}
	chromas = discovered
}

func loadSkinChromas(assets fs.FS) ([]Chroma, error) {
	sheets, err := skins.ReadShopSheets(assets)
	if err != nil {
		return nil, fmt.Errorf("read shop skin sheets: %w", err)
	}
	result := make([]Chroma, 0, len(sheets))
	seenIDs := make(map[string]bool)
	for _, sheet := range sheets {
		chroma, err := readSkinManifest(sheet)
		if err != nil {
			return nil, err
		}
		if seenIDs[chroma.ID] {
			return nil, fmt.Errorf("duplicate chroma ID %q", chroma.ID)
		}
		seenIDs[chroma.ID] = true
		result = append(result, chroma)
	}
	return result, nil
}

func readSkinManifest(sheet skins.SpriteSheet) (Chroma, error) {
	family, variant := sheet.Family, sheet.Color
	manifestPath := path.Join("skinshop", family+"_"+variant+".json")
	manifest := sheet.Meta.SkinData
	manifest.DisplayName = strings.TrimSpace(manifest.DisplayName)
	if manifest.DisplayName == "" {
		return Chroma{}, fmt.Errorf("skin manifest %q has an empty displayName", manifestPath)
	}
	if !skinHexColor.MatchString(manifest.HexColor) {
		return Chroma{}, fmt.Errorf("skin manifest %q has invalid hexColor %q", manifestPath, manifest.HexColor)
	}
	if manifest.Price <= 0 {
		return Chroma{}, fmt.Errorf("skin manifest %q price must be positive", manifestPath)
	}
	manifest.Currency = strings.ToLower(strings.TrimSpace(manifest.Currency))
	if manifest.Currency != string(Marks) && manifest.Currency != string(Crowns) {
		return Chroma{}, fmt.Errorf("skin manifest %q currency must be marks or crowns", manifestPath)
	}
	if manifest.RarityRank < 0 || manifest.RarityRank > 4 {
		return Chroma{}, fmt.Errorf("skin manifest %q rarityRank must be between 0 and 4", manifestPath)
	}
	manifest.Rarity = [...]string{"common", "uncommon", "rare", "epic", "seasonal"}[manifest.RarityRank]
	manifest.ID = strings.TrimSpace(manifest.ID)
	if manifest.ID == "" {
		manifest.ID = family + "-" + variant
	}
	if !skinID.MatchString(manifest.ID) {
		return Chroma{}, fmt.Errorf("skin manifest %q has invalid id %q", manifestPath, manifest.ID)
	}
	color := strings.ToLower(strings.TrimSpace(manifest.Color))
	if color == "" {
		color = strings.ReplaceAll(variant, "-", " ")
	}
	skinPath := path.Join("shop", family, variant)
	return Chroma{
		ID: manifest.ID, Name: manifest.DisplayName, Color: color, Skin: skinPath,
		HexColor: manifest.HexColor, Rarity: manifest.Rarity, RarityRank: manifest.RarityRank, Cost: manifest.Price,
		Currency: manifest.Currency,
	}, nil
}

func AllChromas() []Chroma {
	return append([]Chroma(nil), chromas...)
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

func BuyChromaFamily(p *profile.Profile, family string) (int64, Currency, error) {
	if p == nil {
		return 0, "", errors.New("profile is required")
	}
	family = strings.ToLower(strings.TrimSpace(family))
	if family == "" {
		return 0, "", errors.New("chroma family is required")
	}

	var purchasable []Chroma
	familyFound := false
	for _, chroma := range chromas {
		if chromaFamilyKey(chroma) != family {
			continue
		}
		familyFound = true
		switch Currency(chroma.Currency) {
		case Marks, Crowns:
			purchasable = append(purchasable, chroma)
		case Currency("event"):
		default:
			return 0, "", fmt.Errorf("unsupported currency %q for chroma %q", chroma.Currency, chroma.ID)
		}
	}
	if !familyFound {
		return 0, "", fmt.Errorf("unknown chroma family %q", family)
	}
	if len(purchasable) < 2 {
		return 0, "", errors.New("family does not have multiple purchasable chromas")
	}

	currency := Currency(purchasable[0].Currency)
	var fullPrice int64
	for _, chroma := range purchasable {
		if Currency(chroma.Currency) != currency {
			return 0, "", fmt.Errorf("family %q has chromas with mixed currencies", family)
		}
		if chroma.Cost <= 0 {
			return 0, "", fmt.Errorf("chroma %q has an invalid price", chroma.ID)
		}
		if p.OwnedChromas[chroma.ID] {
			return 0, "", errors.New("Buy All is only available when no family chromas are owned")
		}
		fullPrice += chroma.Cost
	}
	cost := (fullPrice*4 + 2) / 5

	switch currency {
	case Marks:
		if p.Marks < cost {
			return 0, "", fmt.Errorf("not enough Marks: need %d", cost)
		}
		p.Marks -= cost
	case Crowns:
		if p.Crowns < cost {
			return 0, "", fmt.Errorf("not enough Crowns: need %d", cost)
		}
		p.Crowns -= cost
	default:
		return 0, "", fmt.Errorf("unsupported currency %q for chroma family %q", currency, family)
	}

	if p.OwnedChromas == nil {
		p.OwnedChromas = make(map[string]bool)
	}
	if p.SeenChromas == nil {
		p.SeenChromas = make(map[string]bool)
	}
	for _, chroma := range purchasable {
		p.OwnedChromas[chroma.ID] = true
		p.SeenChromas[chroma.ID] = false
		recordPurchase(p, chroma)
	}
	return cost, currency, nil
}

func chromaFamilyKey(chroma Chroma) string {
	skinPath := strings.ReplaceAll(chroma.Skin, "\\", "/")
	parts := strings.Split(skinPath, "/")
	for i, part := range parts {
		if part == "shop" && i+1 < len(parts) {
			return strings.ToLower(parts[i+1])
		}
	}
	return strings.ToLower(strings.TrimSpace(chroma.Name))
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
	conflictingSuit := ""
	for otherSuit, assigned := range p.AssignedChromas {
		if otherSuit != suit {
			other, found := FindChroma(assigned)
			if found && other.Color == chroma.Color {
				if conflictingSuit != "" {
					return fmt.Errorf("color %q is already assigned to multiple suits", chroma.Color)
				}
				conflictingSuit = otherSuit
			}
			if !found && assigned == chroma.Color {
				if conflictingSuit != "" {
					return fmt.Errorf("color %q is already assigned to multiple suits", chroma.Color)
				}
				conflictingSuit = otherSuit
			}
		}
	}
	if conflictingSuit != "" {
		currentAppearance := p.AssignedChromas[suit]
		if currentAppearance == "" {
			return fmt.Errorf("suit %s has no appearance to swap", suit)
		}
		p.AssignedChromas[conflictingSuit] = currentAppearance
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
