package main

import "testing"

func TestCardCatalogIncludesDeckAndEveryArtwork(t *testing.T) {
	catalog, err := loadCardCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cards) != 27 || len(catalog.Artwork) != 72 {
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
