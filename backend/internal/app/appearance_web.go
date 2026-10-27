package app

import (
	"errors"
	"strings"

	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/profile"
)

func applySuitAppearance(p *profile.Profile, request actionRequest) (string, error) {
	if request.Action == "rotate-colors" {
		p.AssignedChromas["B"], p.AssignedChromas["F"], p.AssignedChromas["S"] = p.AssignedChromas["S"], p.AssignedChromas["B"], p.AssignedChromas["F"]
		return "Rotated suit appearances.", nil
	}
	if request.Action != "set-suit-color" {
		return "", errors.New("unknown shop action")
	}
	if request.Suit != "B" && request.Suit != "F" && request.Suit != "S" {
		return "", errors.New("choose a valid suit")
	}
	color := strings.ToLower(request.ID)
	if !containsHeroID([]string{
		"red", "blue", "yellow", "green", "orange", "purple", "lime green", "gold", "pink", "sky blue",
		"silver", "brown", "gray", "ruby", "white",
	}, color) {
		return "", errors.New("choose an available suit color")
	}
	for suit, assigned := range p.AssignedChromas {
		if suit == request.Suit {
			continue
		}
		other := assigned
		if chroma, ok := catalog.FindChroma(assigned); ok {
			other = chroma.Color
		}
		if other == color {
			return "", errors.New("each suit must use a different color")
		}
	}
	p.AssignedChromas[request.Suit] = color
	return "Suit color saved to your commander.", nil
}
