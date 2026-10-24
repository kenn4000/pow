package app

import (
	"strings"

	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/skills"
)

// Player-facing names for identifiers that appear in success messages. Each
// falls back to the identifier itself so an unexpected ID never hides a result.

func skillName(id string) string {
	if node, ok := skills.FindNode(id); ok && node.Name != "" {
		return node.Name
	}
	return id
}

func chromaName(id string) string {
	if chroma, ok := catalog.FindChroma(id); ok && chroma.Name != "" {
		return chroma.Name
	}
	return id
}

func bundleName(id string) string {
	for _, bundle := range catalog.AllBundles() {
		if bundle.ID == id && bundle.Name != "" {
			return bundle.Name
		}
	}
	return id
}

func suitDisplayName(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "B", "BLADE", "SCISSORS":
		return "Scissors"
	case "F", "FANG", "PAPER":
		return "Paper"
	case "S", "SHIELD", "ROCK":
		return "Rock"
	default:
		return code
	}
}
