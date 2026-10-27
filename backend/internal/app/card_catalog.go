package app

import (
	"fmt"
	"image/png"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"prisonersofwar/internal/game"
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
	skins := []string{"shop/tribal/blue", "shop/tribal/yellow", "shop/tribal/red"}
	for _, card := range game.NewDeck() {
		tier := "large"
		if card.Face <= 3 {
			tier = "small"
		} else if card.Face <= 6 {
			tier = "med"
		}
		result.Cards = append(result.Cards, catalogCard{card.Code(), card.Suit.String(), card.Face, tier, game.Suit((int(card.Suit) + 1) % 3).String(), game.Suit((int(card.Suit) + 2) % 3).String(), skins[card.Suit]})
	}
	err := fs.WalkDir(webFiles, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasPrefix(path, "shop/") || !strings.HasSuffix(path, ".png") {
			return nil
		}
		art, err := inspectArtwork(path)
		if err != nil {
			return err
		}
		result.Artwork = append(result.Artwork, art)
		return nil
	})
	return result, err
}

func inspectArtwork(assetPath string) (cardArtwork, error) {
	skin := path.Dir(assetPath)
	parts := strings.Split(skin, "/")
	if len(parts) != 3 || parts[0] != "shop" {
		return cardArtwork{}, fmt.Errorf("invalid artwork path %q", assetPath)
	}
	file := path.Base(assetPath)
	if path.Ext(file) != ".png" {
		return cardArtwork{}, fmt.Errorf("invalid artwork name %q", file)
	}
	color := strings.ReplaceAll(parts[2], "-", " ")
	tier, pose, ok := strings.Cut(strings.TrimSuffix(file, ".png"), "_")
	if !ok {
		return cardArtwork{}, fmt.Errorf("invalid artwork path %q", assetPath)
	}
	asset, err := webFiles.Open(assetPath)
	if err != nil {
		return cardArtwork{}, err
	}
	defer asset.Close()
	info, err := asset.Stat()
	if err != nil {
		return cardArtwork{}, err
	}
	dimensions, err := png.DecodeConfig(asset)
	if err != nil {
		return cardArtwork{}, err
	}
	return cardArtwork{assetPath, "skins/" + assetPath, skin, color, tier, pose, dimensions.Width, dimensions.Height, info.Size()}, nil
}

func (s *webServer) handleCardCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := loadCardCatalog()
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}
