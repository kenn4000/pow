package app

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestCardCatalogIncludesDeckAndEveryArtwork(t *testing.T) {
	catalog, err := loadCardCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cards) != 27 || len(catalog.Artwork) != 120 {
		t.Fatalf("catalog has %d cards and %d artwork files", len(catalog.Cards), len(catalog.Artwork))
	}

	artwork := map[string]cardArtwork{}
	for _, art := range catalog.Artwork {
		if _, duplicate := artwork[art.ID]; duplicate {
			t.Fatalf("duplicate asset %q", art.ID)
		}
		artwork[art.ID] = art
		if art.Width <= 0 || art.Height <= 0 || art.Bytes <= 0 {
			t.Fatalf("invalid metadata: %+v", art)
		}
		if art.Path != "skins/"+art.ID {
			t.Fatalf("invalid asset URL %q", art.Path)
		}
	}
	cards := map[string]bool{}
	for _, card := range catalog.Cards {
		if cards[card.ID] {
			t.Fatalf("duplicate card %q", card.ID)
		}
		cards[card.ID] = true
		if card.Suit == card.Beats || card.Suit == card.LosesTo || card.Beats == card.LosesTo {
			t.Fatalf("invalid suit cycle: %+v", card)
		}
		if _, ok := artwork[card.DefaultSkin+"/"+card.Tier+"_idle.png"]; !ok {
			t.Fatalf("missing artwork for %s", card.ID)
		}
	}
}

func TestCardCatalogExcludesHeroIcons(t *testing.T) {
	catalog, err := loadCardCatalog()
	if err != nil {
		t.Fatal(err)
	}
	artwork := make(map[string]bool, len(catalog.Artwork))
	for _, art := range catalog.Artwork {
		if strings.HasPrefix(art.ID, "hero_icons/") {
			t.Fatalf("hero icon included as card artwork: %q", art.ID)
		}
		artwork[art.ID] = true
	}
	for _, id := range []string{"conq_lime/small_idle.png", "conq_gold/med_attack.png", "conq_pink/large_win.png", "conq_blue/large_lose.png"} {
		if !artwork[id] {
			t.Errorf("Conquistador artwork %q is missing", id)
		}
	}
}

func TestArtworkThumbnailKeepsAspectRatioAndReducesTransferSize(t *testing.T) {
	name := "tribal_blue/small_idle.png"
	asset, err := webFiles.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	data, err := thumbnailArtwork(name, asset, 160)
	if err != nil {
		t.Fatal(err)
	}
	image, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if image.Width != 160 || image.Height < 250 || image.Height > 270 {
		t.Fatalf("unexpected thumbnail dimensions: %+v", image)
	}
	if len(data) >= len(asset) {
		t.Fatalf("thumbnail is %d bytes, original %d", len(data), len(asset))
	}
	cached, err := thumbnailArtwork(name, nil, 160)
	if err != nil || !bytes.Equal(data, cached) {
		t.Fatal("cached thumbnail changed")
	}
}
