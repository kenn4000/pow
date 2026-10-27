package catalog

import (
	"encoding/json"
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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Skin     string `json:"skin,omitempty"`
	HexColor string `json:"hexColor,omitempty"`
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
	{ID: "walnut", Name: "Walnut", Color: "brown", Skin: "shop/tribal/orange", HexColor: "#8a5a32", Rarity: "rare", Cost: 30, Currency: string(Crowns)},
	{ID: "silver", Name: "Silver", Color: "silver", HexColor: "#d7dce2", Rarity: "rare", Cost: 30, Currency: string(Crowns)},
	{ID: "rose-gold", Name: "Rose Gold", Color: "rose-gold", Skin: "shop/tribal/red", HexColor: "#c77b72", Rarity: "rare", Cost: 35, Currency: string(Crowns)},
	{ID: "aurora", Name: "Aurora", Color: "aurora", Skin: "shop/tribal/purple", HexColor: "#9457d6", Rarity: "epic", Cost: 50, Currency: string(Crowns)},
	{ID: "obsidian", Name: "Obsidian", Color: "obsidian", Skin: "shop/tribal/blue", HexColor: "#353e57", Rarity: "epic", Cost: 50, Currency: string(Crowns)},
	{ID: "prismatic", Name: "Prismatic", Color: "prismatic", Skin: "shop/tribal/purple", HexColor: "#ba73ed", Rarity: "legendary", Cost: 80, Currency: string(Crowns)},
	{ID: "ember-dog", Name: "Ember Dog", Color: "orange", Skin: "shop/tribal/orange", HexColor: "#ed782e", Rarity: "seasonal", Currency: "event"},
}

type skinManifest struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
	HexColor    string `json:"hexColor"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency"`
	Rarity      string `json:"rarity"`
}

var (
	skinPathPart = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	skinID       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	skinHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func init() {
	discovered, err := loadSkinChromas(skins.Files)
	if err != nil {
		panic(fmt.Errorf("load skin shop catalog: %w", err))
	}
	for _, added := range discovered {
		if _, exists := FindChroma(added.ID); exists {
			panic(fmt.Errorf("load skin shop catalog: duplicate chroma ID %q", added.ID))
		}
		chromas = append(chromas, added)
	}
}

func loadSkinChromas(assets fs.FS) ([]Chroma, error) {
	families, err := fs.ReadDir(assets, "shop")
	if err != nil {
		return nil, fmt.Errorf("read shop skin families: %w", err)
	}
	result := make([]Chroma, 0)
	seenIDs := make(map[string]bool)
	for _, family := range families {
		if !family.IsDir() || !skinPathPart.MatchString(family.Name()) {
			return nil, fmt.Errorf("invalid skin family %q", family.Name())
		}
		familyPath := path.Join("shop", family.Name())
		variants, err := fs.ReadDir(assets, familyPath)
		if err != nil {
			return nil, fmt.Errorf("read skin family %q: %w", family.Name(), err)
		}
		if len(variants) == 0 {
			return nil, fmt.Errorf("skin family %q has no color variants", family.Name())
		}
		for _, variant := range variants {
			if !variant.IsDir() || !skinPathPart.MatchString(variant.Name()) {
				return nil, fmt.Errorf("invalid skin variant %q in family %q", variant.Name(), family.Name())
			}
			skinPath := path.Join(familyPath, variant.Name())
			chroma, err := readSkinManifest(assets, skinPath, family.Name(), variant.Name())
			if err != nil {
				return nil, err
			}
			if seenIDs[chroma.ID] {
				return nil, fmt.Errorf("duplicate chroma ID %q", chroma.ID)
			}
			seenIDs[chroma.ID] = true
			result = append(result, chroma)
		}
	}
	return result, nil
}

func readSkinManifest(assets fs.FS, skinPath, family, variant string) (Chroma, error) {
	manifestPath := path.Join(skinPath, "skin.json")
	data, err := fs.ReadFile(assets, manifestPath)
	if err != nil {
		return Chroma{}, fmt.Errorf("read skin manifest %q: %w", manifestPath, err)
	}
	var manifest skinManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Chroma{}, fmt.Errorf("decode skin manifest %q: %w", manifestPath, err)
	}
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
	manifest.Rarity = strings.ToLower(strings.TrimSpace(manifest.Rarity))
	if manifest.Rarity == "" {
		manifest.Rarity = "common"
	}
	switch manifest.Rarity {
	case "common", "uncommon", "rare", "epic", "legendary", "seasonal", "super-rare":
	default:
		return Chroma{}, fmt.Errorf("skin manifest %q has unsupported rarity %q", manifestPath, manifest.Rarity)
	}
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
	for _, tier := range []string{"small", "med", "large"} {
		for _, pose := range []string{"idle", "attack", "win", "lose"} {
			artPath := path.Join(skinPath, tier+"_"+pose+".png")
			info, err := fs.Stat(assets, artPath)
			if err != nil {
				return Chroma{}, fmt.Errorf("skin %q is missing artwork %q: %w", skinPath, artPath, err)
			}
			if info.IsDir() {
				return Chroma{}, fmt.Errorf("skin artwork %q is not a file", artPath)
			}
		}
	}
	return Chroma{
		ID: manifest.ID, Name: manifest.DisplayName, Color: color, Skin: skinPath,
		HexColor: manifest.HexColor, Rarity: manifest.Rarity, Cost: manifest.Price,
		Currency: manifest.Currency,
	}, nil
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
