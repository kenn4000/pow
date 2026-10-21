package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"prisonersofwar/achievements"
	"prisonersofwar/energy"
	"prisonersofwar/lootbox"
	"prisonersofwar/profile"
)

func renderLandingPage(writer io.Writer, p *profile.Profile, now time.Time) error {
	if writer == nil {
		return errors.New("landing page output is required")
	}
	if p == nil {
		return errors.New("profile is required")
	}

	currentEnergy := energy.Current(p, now)
	adsLeft := energy.DailyAdLimit - p.AdsWatchedToday
	if adsLeft < 0 {
		adsLeft = 0
	}
	unseen := len(unseenAchievementIDs(p))
	commanderID := p.PlayerID
	if len(commanderID) > 8 {
		commanderID = commanderID[:8]
	}

	lines := []string{
		fmt.Sprintf("%-39s Marks: %s", "PRISONERS OF WAR", formatLandingNumber(p.Marks)),
		fmt.Sprintf("%-39s Crowns: %s", "Commander ["+commanderID+"] [Recruit]", formatLandingNumber(p.Crowns)),
		fmt.Sprintf("%-39s Energy: %d/%d", "[Avatar placeholder]", currentEnergy, p.MaxEnergy),
		"",
		"FEATURED",
		"Featured chroma: none today",
		"",
		"QUICK ACTIONS",
		"[1] PLAY",
	}
	if p.DrawsAvailable > 0 || p.TotalPendingBoxes() > 0 {
		campAction := "[2] Camp Draw"
		if p.DrawsAvailable > 0 {
			campAction += fmt.Sprintf(" (%d)", p.DrawsAvailable)
		} else {
			campAction += " / Pending Boxes"
		}
		lines = append(lines, campAction)
		if p.TotalPendingBoxes() > 0 {
			lines = append(lines, fmt.Sprintf("Pending Boxes: %d", p.TotalPendingBoxes()))
		}
	}
	lines = append(lines, "[3] Shop")
	skillAction := "[4] Skill Tree"
	if p.SkillPoints > 0 {
		skillAction += fmt.Sprintf(" (%d STP)", p.SkillPoints)
	}
	lines = append(lines, skillAction)
	achievementAction := "[5] Achievements"
	if unseen > 0 {
		achievementAction += fmt.Sprintf(" (%d new)", unseen)
	}
	lines = append(lines, achievementAction, "", "STATUS",
		fmt.Sprintf("Login Streak: Day %d of 30 (Best: %d)", p.LoginStreakCurrent, p.LoginStreakBest),
		fmt.Sprintf("Camp Draw: %d / %d wins to next draw", p.WinsSinceCampDraw, lootbox.RequiredWins(p)),
		landingEnergyStatus(p, currentEnergy, adsLeft, now),
		"[R] Energy refills",
		"",
		"[6] Profile   [7] Settings   [Q] Quit")

	width := 57
	for _, line := range lines {
		if length := utf8.RuneCountInString(line); length > width {
			width = length
		}
	}
	var output strings.Builder
	border := "┌" + strings.Repeat("─", width+2) + "┐\n"
	divider := "├" + strings.Repeat("─", width+2) + "┤\n"
	output.WriteString(border)
	for _, line := range lines {
		if line == "FEATURED" || line == "QUICK ACTIONS" || line == "STATUS" {
			output.WriteString(divider)
		}
		output.WriteString("│ ")
		output.WriteString(line)
		output.WriteString(strings.Repeat(" ", width-utf8.RuneCountInString(line)))
		output.WriteString(" │\n")
	}
	output.WriteString("└" + strings.Repeat("─", width+2) + "┘\n")
	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("render landing page: %w", err)
	}
	return nil
}

func landingEnergyStatus(p *profile.Profile, currentEnergy, adsLeft int, now time.Time) string {
	status := fmt.Sprintf("Energy: %d/%d", currentEnergy, p.MaxEnergy)
	if currentEnergy < p.MaxEnergy {
		if remaining := energy.TimeUntilNext(p, now); remaining > 0 {
			seconds := int((remaining + time.Second - 1) / time.Second)
			status += fmt.Sprintf(" (next in %d:%02d)", seconds/60, seconds%60)
		}
		if adsLeft > 0 {
			status += fmt.Sprintf(". [W]atch Ad (+%d, %d left today)", energy.EnergyPerAd, adsLeft)
		}
	}
	return status
}

func formatLandingNumber(value int64) string {
	digits := fmt.Sprintf("%d", value)
	if len(digits) <= 3 {
		return digits
	}
	firstGroup := len(digits) % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	var result strings.Builder
	result.WriteString(digits[:firstGroup])
	for index := firstGroup; index < len(digits); index += 3 {
		result.WriteByte(',')
		result.WriteString(digits[index : index+3])
	}
	return result.String()
}

func unseenAchievementIDs(p *profile.Profile) []string {
	if p == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(p.SeenAchievementIDs))
	for _, id := range p.SeenAchievementIDs {
		seen[id] = struct{}{}
	}
	unseen := make([]string, 0)
	for _, achievement := range achievements.All() {
		isUnlocked := p.Achievements[achievement.ID]
		if len(achievement.Tiers) > 0 {
			isUnlocked = p.AchievementTiers[achievement.ID] > 0
		}
		if isUnlocked {
			if _, alreadySeen := seen[achievement.ID]; !alreadySeen {
				unseen = append(unseen, achievement.ID)
			}
		}
	}
	return unseen
}

func markAchievementsSeen(p *profile.Profile) {
	if p == nil {
		return
	}
	p.SeenAchievementIDs = append(p.SeenAchievementIDs, unseenAchievementIDs(p)...)
}

func (a *app) settingsMenu(p *profile.Profile) (bool, error) {
	if p == nil {
		return false, errors.New("profile is required")
	}
	path, err := profile.ProfileFilePath()
	if err != nil {
		return false, err
	}
	for {
		a.clearScreen()
		colorState := "off"
		if p.ColorOn {
			colorState = "on"
		}
		quitState := "off"
		if p.ConfirmQuit {
			quitState = "on"
		}
		fmt.Println("=== SETTINGS ===")
		fmt.Printf("Color output: %s", colorState)
		if a.forceNoColor && p.ColorOn {
			fmt.Print(" (disabled by command-line option)")
		}
		fmt.Println()
		fmt.Printf("Confirm before quitting: %s\n", quitState)
		fmt.Printf("Profile file: %s\n", path)
		fmt.Println("[1] Toggle color output")
		fmt.Println("[2] Toggle quit confirmation")
		fmt.Println("[3] Delete profile")
		fmt.Println("[0] Back")
		answer, err := a.ask("Choose a setting: ")
		if err != nil {
			return false, err
		}
		switch strings.TrimSpace(answer) {
		case "0":
			return false, nil
		case "1":
			p.ColorOn = !p.ColorOn
			colorsEnabled = p.ColorOn && !a.forceNoColor
			if err := p.Save(); err != nil {
				return false, err
			}
		case "2":
			p.ConfirmQuit = !p.ConfirmQuit
			if err := p.Save(); err != nil {
				return false, err
			}
		case "3":
			deleted, err := a.confirmDeleteProfile(path)
			if err != nil {
				return false, err
			}
			if deleted {
				return true, nil
			}
		default:
			fmt.Println("Enter 1, 2, 3, or 0.")
		}
	}
}

func (a *app) confirmDeleteProfile(path string) (bool, error) {
	answer, err := a.ask(fmt.Sprintf("Delete profile at %s? (y/N): ", path))
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(strings.TrimSpace(answer), "y") &&
		!strings.EqualFold(strings.TrimSpace(answer), "yes") {
		return false, nil
	}
	answer, err = a.ask("This permanently removes your profile. Type DELETE to confirm: ")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(answer) != "DELETE" {
		fmt.Println("Profile deletion cancelled.")
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("delete profile %q: %w", path, err)
	}
	fmt.Println("Profile deleted. Exiting so it will not be recreated this session.")
	return true, nil
}

func (a *app) energyRefillMenu(p *profile.Profile) error {
	for {
		current := energy.Current(p, time.Now())
		fmt.Printf("\n=== ENERGY | %d/%d | Crowns %d ===\n", current, p.MaxEnergy, p.Crowns)
		fmt.Printf("[1] Buy full refill (%d Crowns)\n", energy.CrownsPerFullRefill)
		fmt.Println("[2] Use daily free refill (if unlocked)")
		fmt.Println("[0] Back")
		answer, err := a.ask("Choose an energy option: ")
		if err != nil {
			return err
		}
		switch strings.TrimSpace(answer) {
		case "0":
			return nil
		case "1":
			err = energy.BuyFullRefill(p, time.Now())
		case "2":
			err = energy.UseDailyFreeRefill(p, time.Now())
		default:
			fmt.Println("Enter 1, 2, or 0.")
			continue
		}
		if err != nil {
			fmt.Println("Refill unavailable:", err)
			continue
		}
		if err := p.Save(); err != nil {
			return err
		}
	}
}
