package app

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/skins"
)

type cardArtwork struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Skin   string `json:"skin"`
	Color  string `json:"color"`
	Tier   string `json:"tier"`
	Pose   string `json:"pose"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int64  `json:"bytes"`
}

type catalogCard struct {
	ID          string `json:"id"`
	Suit        string `json:"suit"`
	Face        int    `json:"face"`
	Tier        string `json:"tier"`
	Beats       string `json:"beats"`
	LosesTo     string `json:"losesTo"`
	DefaultSkin string `json:"defaultSkin"`
}

type cardCatalog struct {
	Cards   []catalogCard `json:"cards"`
	Artwork []cardArtwork `json:"artwork"`
}

func loadCardCatalog() (cardCatalog, error) {
	result := cardCatalog{Cards: []catalogCard{}, Artwork: []cardArtwork{}}
	defaultSkins := []string{"shop/tribal/blue", "shop/tribal/yellow", "shop/tribal/red"}
	for _, card := range game.NewDeck() {
		tier := "large"
		if card.Face <= 3 {
			tier = "small"
		} else if card.Face <= 6 {
			tier = "med"
		}
		result.Cards = append(result.Cards, catalogCard{card.Code(), card.Suit.String(), card.Face, tier, game.Suit((int(card.Suit) + 1) % 3).String(), game.Suit((int(card.Suit) + 2) % 3).String(), defaultSkins[card.Suit]})
	}
	for _, chroma := range catalog.AllChromas() {
		parts := strings.Split(chroma.Skin, "/")
		if len(parts) != 3 || parts[0] != "shop" {
			return cardCatalog{}, fmt.Errorf("invalid chroma skin path %q", chroma.Skin)
		}
		sheet, err := skins.ReadShopSheet(webFiles, parts[1], parts[2])
		if err != nil {
			return cardCatalog{}, err
		}
		sheetInfo, err := fs.Stat(webFiles, path.Join("skinshop", sheet.Meta.Image))
		if err != nil {
			return cardCatalog{}, fmt.Errorf("stat skin sprite image %q: %w", sheet.Meta.Image, err)
		}
		for _, tier := range []string{"small", "med", "large"} {
			for _, pose := range []string{"idle", "attack", "win", "lose"} {
				filename := tier + "_" + pose + ".png"
				frame, ok := sheet.Frames[filename]
				if !ok {
					return cardCatalog{}, fmt.Errorf("skin sprite sheet %q is missing frame %q", sheet.Meta.Image, filename)
				}
				assetPath := path.Join(chroma.Skin, filename)
				result.Artwork = append(result.Artwork, cardArtwork{
					ID: assetPath, Path: "skins/" + assetPath, Skin: chroma.Skin,
					Color: strings.ReplaceAll(parts[2], "-", " "), Tier: tier, Pose: pose,
					Width: frame.SourceSize.Width, Height: frame.SourceSize.Height, Bytes: sheetInfo.Size(),
				})
			}
		}
	}
	return result, nil
}

func (s *webServer) handleCardCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := loadCardCatalog()
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}
