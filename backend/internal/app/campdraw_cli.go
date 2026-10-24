package app

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
)

type terminalCampDrawUI struct {
	app  *app
	err  error
	wins int
}

func (ui *terminalCampDrawUI) ShowStartingHand(cards []game.Card) {
	fmt.Println("\n=== CAMP DRAW ===")
	fmt.Printf("Your hand: %s\n", formatCards(cards))
	fmt.Printf("CPU hand: hidden (%d cards)\n", lootbox.CPUHandSize)
}

func (ui *terminalCampDrawUI) ChooseCardForExchange(exchangeIndex int, isDefending bool, cpuCard *game.Card, playerPool []game.Card) int {
	action := "ATTACK"
	if isDefending {
		action = "DEFEND"
	}
	for {
		fmt.Printf("\n=== Exchange %d/3: %s ===\n", exchangeIndex+1, action)
		if isDefending && cpuCard != nil {
			fmt.Printf("CPU attacks with: %s\n", cpuCard)
		} else {
			fmt.Println("CPU defense is hidden.")
		}
		fmt.Printf("Your pool: %s\n", formatCards(playerPool))
		prompt := "Choose a card to attack with (1-N): "
		if isDefending {
			prompt = "Choose a card to defend with (1-N): "
		}
		answer, err := ui.app.ask(prompt)
		if err != nil {
			ui.err = err
			return -1
		}
		var selected int
		if _, err := fmt.Sscanf(strings.TrimSpace(answer), "%d", &selected); err != nil ||
			selected < 1 || selected > len(playerPool) {
			fmt.Printf("Enter a number from 1 to %d.\n", len(playerPool))
			continue
		}
		return selected - 1
	}
}

func (ui *terminalCampDrawUI) ShowExchangeResult(exchange lootbox.ExchangeResult) {
	switch exchange.Outcome {
	case 2:
		ui.wins++
		fmt.Printf("You WON — captured CPU's %s. (Wins: %d)\n", exchange.CPUCard, ui.wins)
	case 1:
		fmt.Printf("Tie — no card captured. (Wins: %d)\n", ui.wins)
	default:
		fmt.Printf("You LOST — CPU captured your %s. (Wins: %d)\n", exchange.PlayerCard, ui.wins)
	}
}

func (ui *terminalCampDrawUI) ShowBoxResult(boxType string, reward lootbox.BoxOpenResult) {
	fmt.Printf("\n+--------------------+\n| %s BOX OPENED |\n+--------------------+\n", strings.ToUpper(boxType))
	printBoxReward(reward)
}

func formatCards(cards []game.Card) string {
	parts := make([]string, len(cards))
	for index, card := range cards {
		parts[index] = fmt.Sprintf("[%d] %s", index+1, card)
	}
	return strings.Join(parts, "  ")
}

func printBoxReward(reward lootbox.BoxOpenResult) {
	switch {
	case reward.ChromaID != "":
		chromaName := reward.ChromaID
		if chroma, ok := catalog.FindChroma(reward.ChromaID); ok {
			chromaName = chroma.Name
		}
		fmt.Printf("+ Chroma: %s (%s)\n", chromaName, reward.ChromaRarity)
	case reward.Crowns > 0:
		fmt.Printf("+%s Crowns\n", formatLandingNumber(reward.Crowns))
	default:
		fmt.Printf("+%s Marks\n", formatLandingNumber(reward.Marks))
	}
	if reward.FallbackReason != "" {
		fmt.Printf("Chroma fallback: %s\n", reward.FallbackReason)
	}
}

func (a *app) campDrawHub(p *profile.Profile) error {
	for {
		fmt.Println("\n=== CAMP DRAW ===")
		if p.DrawsAvailable > 0 {
			fmt.Printf("[1] Open Camp Draw minigame (%d available)\n", p.DrawsAvailable)
		}
		if p.TotalPendingBoxes() > 0 {
			fmt.Println("[2] Open Pending Box")
		}
		fmt.Println("[0] Back")
		answer, err := a.ask("Choose an option: ")
		if err != nil {
			return err
		}
		switch strings.TrimSpace(answer) {
		case "0":
			return nil
		case "1":
			if p.DrawsAvailable == 0 {
				continue
			}
			ui := &terminalCampDrawUI{app: a}
			result, err := lootbox.OpenCampDraw(p, a.rng, ui)
			if ui.err != nil {
				return ui.err
			}
			if err != nil {
				return err
			}
			fmt.Printf("Camp Draw complete: %d wins, %s box.\n", result.PlayerWins, result.BoxType)
			events, err := processCampDrawAchievements(p, time.Now())
			if err != nil {
				return err
			}
			for _, event := range events {
				showUnlockToast(event)
			}
			if _, err := a.ask("Press Enter to continue."); err != nil {
				return err
			}
		case "2":
			if p.TotalPendingBoxes() > 0 {
				if err := a.pendingBoxesMenu(p); err != nil {
					return err
				}
			}
		default:
			fmt.Println("Enter 1, 2, or 0.")
		}
	}
}

func (a *app) pendingBoxesMenu(p *profile.Profile) error {
	boxTypes := []string{"wooden", "silver", "gold", "diamond"}
	for {
		fmt.Println("\n=== PENDING BOXES ===")
		for index, boxType := range boxTypes {
			fmt.Printf("[%d] %-8s %d\n", index+1, boxType, p.PendingBoxes[boxType])
		}
		fmt.Println("[0] Back")
		answer, err := a.ask("Choose a box type to open: ")
		if err != nil {
			return err
		}
		var selected int
		if strings.TrimSpace(answer) == "0" {
			return nil
		}
		if _, err := fmt.Sscanf(strings.TrimSpace(answer), "%d", &selected); err != nil ||
			selected < 1 || selected > len(boxTypes) {
			fmt.Printf("Enter 1-%d or 0.\n", len(boxTypes))
			continue
		}
		boxType := boxTypes[selected-1]
		if p.PendingBoxes[boxType] <= 0 {
			fmt.Printf("No %s boxes are available.\n", boxType)
			continue
		}
		answer, err = a.ask(fmt.Sprintf("Open one %s box now? (y/N): ", boxType))
		if err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(answer), "y") &&
			!strings.EqualFold(strings.TrimSpace(answer), "yes") {
			continue
		}
		reward, err := lootbox.OpenPendingBox(p, boxType, a.rng)
		if err != nil {
			return err
		}
		printBoxReward(reward)
		if err := p.Save(); err != nil {
			return err
		}
	}
}

func simulateCampDraws(count int, rng *rand.Rand) error {
	if count <= 0 {
		return fmt.Errorf("Camp Draw simulation count must be positive")
	}
	if rng == nil {
		return fmt.Errorf("random source is required")
	}
	p, err := profile.New(time.Now())
	if err != nil {
		return err
	}
	boxCounts := map[string]int{"wooden": 0, "silver": 0, "gold": 0, "diamond": 0}
	winCounts := [4]int{}
	var marksTotal, crownsTotal, chromaTotal int64
	for index := 0; index < count; index++ {
		p.DrawsAvailable = 1
		result, err := lootbox.OpenCampDraw(p, rng, lootbox.HeadlessCampDrawUI{})
		if err != nil {
			return fmt.Errorf("simulate Camp Draw %d: %w", index+1, err)
		}
		boxCounts[result.BoxType]++
		winCounts[result.PlayerWins]++
		marksTotal += result.Reward.Marks
		crownsTotal += result.Reward.Crowns
		if result.Reward.ChromaID != "" {
			chromaTotal++
		}
	}
	fmt.Printf("Simulated %d Camp Draws (CPU hand size %d)\n", count, lootbox.CPUHandSize)
	fmt.Printf("Wins: 0=%d, 1=%d, 2=%d, 3=%d\n",
		winCounts[0], winCounts[1], winCounts[2], winCounts[3])
	fmt.Printf("Boxes: wooden=%d, silver=%d, gold=%d, diamond=%d\n",
		boxCounts["wooden"], boxCounts["silver"], boxCounts["gold"], boxCounts["diamond"])
	fmt.Printf("Average reward per draw: %.2f Marks, %.2f Crowns, %.4f chromas\n",
		float64(marksTotal)/float64(count), float64(crownsTotal)/float64(count),
		float64(chromaTotal)/float64(count))
	return nil
}

var _ lootbox.CampDrawUI = (*terminalCampDrawUI)(nil)
