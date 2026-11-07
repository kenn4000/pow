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
	if len(catalog.Cards) != 27 || len(catalog.Artwork) < 240 {
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
		parts := strings.Split(art.Skin, "/")
		if len(parts) != 3 || art.Color != parts[2] {
			wantColor := ""
			if len(parts) > 0 {
				wantColor = strings.ReplaceAll(parts[len(parts)-1], "-", " ")
			}
			t.Errorf("artwork %q color = %q, want folder color %q", art.ID, art.Color, wantColor)
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
	for _, id := range []string{
		"shop/conq/lime/small_idle.png", "shop/conq/gold/med_attack.png", "shop/conq/pink/large_win.png", "shop/conq/blue/large_lose.png",
		"shop/alien/yellow/large_idle.png", "shop/amazon/green/large_idle.png", "shop/bee/silver/large_idle.png",
		"shop/cat/brown/large_idle.png", "shop/dog/gray/large_idle.png", "shop/dragon/ruby/large_idle.png",
		"shop/dwarf/purple/large_idle.png", "shop/orc/white/large_idle.png", "shop/reptile/pink/large_idle.png",
		"shop/zulu/orange/large_idle.png",
	} {
		if !artwork[id] {
			t.Errorf("Conquistador artwork %q is missing", id)
		}
	}
}

func TestArtworkThumbnailKeepsAspectRatioAndReducesTransferSize(t *testing.T) {
	name := "shop/tribal/blue/small_idle.png"
	asset, err := skinSpriteFrame(name)
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
	if image.Width != 160 || image.Height != 223 {
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
