package ui

import (
	"fmt"
	"io"
	"strings"

	"prisonersofwar/modifiers"
)

func RenderModifierHUD(writer io.Writer, gameModifiers modifiers.GameModifiers, bountySuit string, color bool) error {
	if writer == nil {
		return fmt.Errorf("modifier HUD output is required")
	}
	lines := []string{"GAME MODIFIERS"}
	if gameModifiers.Bounty != 0 {
		lines = append(lines, fmt.Sprintf("Bounty: %s (%s cards score %s extra)", signed(gameModifiers.Bounty, color), bountySuit, signed(gameModifiers.Bounty, color)))
	}
	if gameModifiers.Shift != 0 {
		lines = append(lines, fmt.Sprintf("Shift: %s (all cards effectively %s)", signed(gameModifiers.Shift, color), signed(gameModifiers.Shift, color)))
	}
	if gameModifiers.Favored != 0 {
		lines = append(lines, fmt.Sprintf("Favored: %s (cycle-winner %s)", signed(gameModifiers.Favored, color), direction(gameModifiers.Favored)))
	}
	if gameModifiers.Reckoning == [3]int{1, 1, 1} {
		lines = append(lines, "Round weights: even (no reckoning)")
	} else {
		lines = append(lines, fmt.Sprintf("Round weights: %d / %d / %d", gameModifiers.Reckoning[0], gameModifiers.Reckoning[1], gameModifiers.Reckoning[2]))
	}
	lines = append(lines, "Bounty suit: "+bountySuit)

	width := 0
	for _, line := range lines {
		if length := visibleLength(line); length > width {
			width = length
		}
	}
	var output strings.Builder
	border := "+" + strings.Repeat("-", width+2) + "+\n"
	output.WriteString(border)
	for _, line := range lines {
		output.WriteString("| ")
		output.WriteString(line)
		output.WriteString(strings.Repeat(" ", width-visibleLength(line)))
		output.WriteString(" |\n")
	}
	output.WriteString(border)
	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("render modifier HUD: %w", err)
	}
	return nil
}

func visibleLength(value string) int {
	length := 0
	inEscape := false
	for _, character := range value {
		if character == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if character == 'm' {
				inEscape = false
			}
			continue
		}
		length++
	}
	return length
}

func signed(value int, color bool) string {
	text := fmt.Sprintf("%+d", value)
	if !color {
		return text
	}
	code := "90"
	if value > 0 {
		code = "32"
	} else if value < 0 {
		code = "31"
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func direction(value int) string {
	if value > 0 {
		return fmt.Sprintf("gains %d", value)
	}
	return fmt.Sprintf("loses %d", -value)
}
