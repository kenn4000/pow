package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"prisonersofwar/ai"
	"prisonersofwar/catalog"
	"prisonersofwar/game"
	"prisonersofwar/heroes"
	"prisonersofwar/profile"
	gameui "prisonersofwar/ui"
)

type mode int

const (
	humanVsHuman mode = iota
	humanVsEasy
	humanVsNormal
	humanVsHard
)

type app struct {
	scanner             *bufio.Scanner
	rng                 *rand.Rand
	mode                mode
	lastWinner          int
	playerProfile       *profile.Profile
	forceNoColor        bool
	gameScoreAfterR2    [2]int
	gameSuitDoublingWin bool
	gameRoundSweeps     int
	playerHeroID        string
	opponentHeroID      string
	playerTuning        heroes.Tuning
	opponentTuning      heroes.Tuning
}

func main() {
	simGames := flag.Int("sim", 0, "run N AI-vs-AI games without the terminal UI")
	simHeroGames := flag.Int("sim-heroes", 0, "run N games for all 66 hero pairings")
	simCampDraws := flag.Int("sim-campdraw", 0, "run N headless Camp Draws and report reward statistics")
	ui := flag.String("ui", "web", "user interface: web or terminal")
	modeFlag := flag.String("mode", "", "game mode: hvh, easy, normal, or hard")
	listen := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	noColor := flag.Bool("no-color", false, "disable ANSI suit colors")
	dumpEconomy := flag.Bool("dump-economy", false, "print economy, skill, chroma, and Camp Draw configuration as JSON")
	flag.Parse()

	if *simGames < 0 {
		fmt.Fprintln(os.Stderr, "--sim must be zero or greater")
		os.Exit(2)
	}
	if *simHeroGames < 0 {
		fmt.Fprintln(os.Stderr, "--sim-heroes must be zero or greater")
		os.Exit(2)
	}
	if *simCampDraws < 0 {
		fmt.Fprintln(os.Stderr, "--sim-campdraw must be zero or greater")
		os.Exit(2)
	}
	if *dumpEconomy {
		if err := dumpEconomyConfig(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	rng, err := secureRand()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *simGames > 0 {
		if err := simulate(*simGames, rng); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *simHeroGames > 0 {
		if err := simulateHeroes(*simHeroGames, rng); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *simCampDraws > 0 {
		if err := simulateCampDraws(*simCampDraws, rng); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *ui == "web" {
		if *modeFlag != "" {
			if _, err := parseMode(*modeFlag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
		if err := startWebUI(*listen, *modeFlag); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *ui != "terminal" {
		fmt.Fprintln(os.Stderr, "unknown UI; use web or terminal")
		os.Exit(2)
	}
	runTerminal(*modeFlag, *noColor, rng)
}

func runTerminal(modeFlag string, noColor bool, rng *rand.Rand) {
	selectedMode, err := parseMode(modeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	gameApp := app{
		scanner:      bufio.NewScanner(os.Stdin),
		rng:          rng,
		mode:         selectedMode,
		forceNoColor: noColor || os.Getenv("NO_COLOR") != "",
	}
	playerProfile, err := profile.LoadProfile()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := refreshEnergy(playerProfile, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := playerProfile.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	colorsEnabled = playerProfile.ColorOn && !gameApp.forceNoColor
	if err := gameApp.initializeProgressionProfile(playerProfile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := gameApp.progressionMenu(playerProfile, selectedMode); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var colorsEnabled = true

func parseMode(value string) (mode, error) {
	switch strings.ToLower(value) {
	case "":
		return -1, nil
	case "hvh":
		return humanVsHuman, nil
	case "easy":
		return humanVsEasy, nil
	case "normal":
		return humanVsNormal, nil
	case "hard":
		return humanVsHard, nil
	default:
		return -1, fmt.Errorf("unknown mode %q; use hvh, easy, normal, or hard", value)
	}
}

func (a *app) chooseMode() (mode, error) {
	fmt.Println("Prisoners of War")
	fmt.Println("1) Human vs Human")
	fmt.Println("2) Human vs AI (Easy)")
	fmt.Println("3) Human vs AI (Normal)")
	fmt.Println("4) Human vs AI (Hard)")
	for {
		answer, err := a.ask("Choose a mode (1-4): ")
		if err != nil {
			return 0, err
		}
		switch strings.TrimSpace(answer) {
		case "1":
			return humanVsHuman, nil
		case "2":
			return humanVsEasy, nil
		case "3":
			return humanVsNormal, nil
		case "4":
			return humanVsHard, nil
		default:
			fmt.Println("Enter 1, 2, 3, or 4.")
		}
	}
}

func (a *app) playGame() (game.GameState, error) {
	state, err := a.newHeroGame()
	if err != nil {
		return game.GameState{}, err
	}
	return a.playGameState(state)
}

func (a *app) newHeroGame() (game.GameState, error) {
	playerID, opponentID := a.playerHeroID, a.opponentHeroID
	if playerID == "" {
		playerID = profile.DefaultHeroID
	}
	if opponentID == "" {
		opponentID = profile.DefaultHeroID
	}
	playerHero, ok := heroes.ByID(playerID)
	if !ok {
		return game.GameState{}, fmt.Errorf("unknown player hero %q", playerID)
	}
	opponentHero, ok := heroes.ByID(opponentID)
	if !ok {
		return game.GameState{}, fmt.Errorf("unknown opponent hero %q", opponentID)
	}
	gameModifiers, err := heroes.GameModifiersForWithTuning(playerHero, opponentHero, a.playerTuning, a.opponentTuning, a.rng)
	if err != nil {
		return game.GameState{}, err
	}
	state, err := game.NewGameWithModifiers(a.rng, gameModifiers)
	if err != nil {
		return game.GameState{}, err
	}
	fmt.Printf("\n%s vs %s\n", playerHero.Name, opponentHero.Name)
	if err := gameui.RenderModifierHUD(os.Stdout, state.Modifiers, state.Bounty.Suit.String(), colorsEnabled && !a.forceNoColor); err != nil {
		return game.GameState{}, err
	}
	return state, nil
}

func (a *app) playGameState(state game.GameState) (game.GameState, error) {
	a.gameScoreAfterR2 = [2]int{}
	a.gameSuitDoublingWin = false
	a.gameRoundSweeps = 0
	fmt.Println("\nPrisoners of War — S Scissors (red), P Paper (yellow), R Rock (blue)")
	fmt.Printf("Bounty: %s\n", a.card(state.Bounty))
	if err := gameui.RenderModifierHUD(os.Stdout, state.Modifiers, state.Bounty.Suit.String(), colorsEnabled && !a.forceNoColor); err != nil {
		return game.GameState{}, err
	}

	for state.Round <= 3 {
		if state.Round == 3 {
			if err := a.assignRoundThreeSeats(&state); err != nil {
				return game.GameState{}, err
			}
		}
		if err := a.playRound(&state); err != nil {
			return game.GameState{}, err
		}
		if state.Round == 2 {
			a.gameScoreAfterR2 = [2]int{
				game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total,
				game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total,
			}
		}
		if state.Round == 3 {
			break
		}
		if state.Round < 3 {
			swap := state.Round == 1
			if err := game.EndRound(&state, swap, true); err != nil {
				return game.GameState{}, err
			}
			fmt.Printf("\nRound %d complete. Refilled hands: P1 %d, P2 %d.\n",
				state.Round, len(state.P1.Hand), len(state.P2.Hand))
			if err := gameui.RenderModifierHUD(os.Stdout, state.Modifiers, state.Bounty.Suit.String(), colorsEnabled && !a.forceNoColor); err != nil {
				return game.GameState{}, err
			}
			state.Round++
		}
	}
	if err := gameui.RenderModifierHUD(os.Stdout, state.Modifiers, state.Bounty.Suit.String(), colorsEnabled && !a.forceNoColor); err != nil {
		return game.GameState{}, err
	}
	a.lastWinner = a.showFinal(state)
	return state, nil
}

func (a *app) playRound(state *game.GameState) error {
	fmt.Printf("\n========== ROUND %d ==========\n", state.Round)
	if err := gameui.RenderModifierHUD(os.Stdout, state.Modifiers, state.Bounty.Suit.String(), colorsEnabled && !a.forceNoColor); err != nil {
		return err
	}
	a.showScores(*state)
	for index := 0; index < 2; index++ {
		player, _ := state.Player(index)
		fmt.Printf("P%d is seat %s (%d cards in hand).\n", index+1, player.Seat, len(player.Hand))
	}
	if err := a.commitRound(state); err != nil {
		return err
	}
	return a.resolveRound(state)
}

func (a *app) commitRound(state *game.GameState) error {
	for playerIndex := 0; playerIndex < 2; playerIndex++ {
		if a.isHuman(playerIndex) {
			if a.mode == humanVsHuman {
				fmt.Printf("\nPass the keyboard to Player %d.\n", playerIndex+1)
				a.clearScreen()
				if _, err := a.ask(fmt.Sprintf("Player %d, press Enter to view your hand: ", playerIndex+1)); err != nil {
					return err
				}
			}
			a.showDecision(*state, playerIndex)
		}
		indices, err := a.chooseCommit(*state, playerIndex)
		if err != nil {
			return err
		}
		if err := game.Commit(state, playerIndex, indices); err != nil {
			return err
		}
		if a.mode == humanVsHuman && playerIndex == 0 {
			fmt.Println("\nPlayer 1 has committed. Pass the keyboard to Player 2.")
			a.clearScreen()
		}
	}
	return nil
}

func (a *app) resolveRound(state *game.GameState) error {
	var roundWinnerCount [2]int
	for exchange := 0; exchange < 3; exchange++ {
		if err := gameui.RenderModifierHUD(os.Stdout, state.Modifiers, state.Bounty.Suit.String(), colorsEnabled && !a.forceNoColor); err != nil {
			return err
		}
		attackerSeat := game.SeatX
		if exchange == 1 {
			attackerSeat = game.SeatY
		}
		attackerIndex, err := state.PlayerAtSeat(attackerSeat)
		if err != nil {
			return err
		}
		defenderIndex := 1 - attackerIndex
		attacker, _ := state.Player(attackerIndex)
		defender, _ := state.Player(defenderIndex)
		fmt.Printf("\nExchange %d: P%d (seat %s) attacks.\n", exchange+1, attackerIndex+1, attacker.Seat)
		if len(attacker.Pool) == 0 || len(defender.Pool) == 0 {
			fmt.Println("Exchange skipped: a player has no pool card available.")
			continue
		}
		attackCard, defendCard, outcome, err := a.playTerminalExchange(state, attackerIndex, defenderIndex, exchange)
		if err != nil {
			return err
		}
		switch outcome.Winner {
		case 0:
			roundWinnerCount[attackerIndex]++
			if outcome.AttackerValue > attackCard.Face {
				a.gameSuitDoublingWin = true
			}
		case 1:
			roundWinnerCount[defenderIndex]++
			if outcome.DefenderValue > defendCard.Face {
				a.gameSuitDoublingWin = true
			}
		}
		a.showOutcome(attackerIndex, attackCard, defendCard, outcome)
		a.showScores(*state)
	}
	if roundWinnerCount[0] == 3 || roundWinnerCount[1] == 3 {
		a.gameRoundSweeps++
	}
	game.DiscardPools(state)
	if err := state.Validate(); err != nil {
		return err
	}
	return gameui.RenderModifierHUD(os.Stdout, state.Modifiers, state.Bounty.Suit.String(), colorsEnabled && !a.forceNoColor)
}

func (a *app) playTerminalExchange(state *game.GameState, attackerIndex, defenderIndex, exchange int) (game.Card, game.Card, game.Outcome, error) {
	attacker, _ := state.Player(attackerIndex)
	defender, _ := state.Player(defenderIndex)
	autoLast := exchange == 2 && len(attacker.Pool) == 1 && len(defender.Pool) == 1
	attackIndex, defendIndex := 0, 0
	if !autoLast {
		a.showDecision(*state, attackerIndex)
		var err error
		attackIndex, err = a.choosePool(*state, attackerIndex, true, nil)
		if err != nil {
			return game.Card{}, game.Card{}, game.Outcome{}, err
		}
	}
	attackCard := attacker.Pool[attackIndex]
	fmt.Printf("P%d reveals %s.\n", attackerIndex+1, a.card(attackCard))
	if autoLast {
		fmt.Println("Final cards are played automatically.")
	} else {
		a.showDecision(*state, defenderIndex)
		fmt.Printf("P%d defends after seeing %s.\n", defenderIndex+1, a.card(attackCard))
		var err error
		defendIndex, err = a.choosePool(*state, defenderIndex, false, &attackCard)
		if err != nil {
			return game.Card{}, game.Card{}, game.Outcome{}, err
		}
	}
	defendCard := defender.Pool[defendIndex]
	outcome, err := game.PlayExchange(state, attackerIndex, attackIndex, defenderIndex, defendIndex)
	return attackCard, defendCard, outcome, err
}

func (a *app) assignRoundThreeSeats(state *game.GameState) error {
	score1 := game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total
	score2 := game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total
	chooser := 0
	if score2 < score1 {
		chooser = 1
	} else if score1 == score2 {
		chooser = a.rng.Intn(2)
	}
	fmt.Printf("\nAfter Round 2: P1 %d, P2 %d. P%d chooses a seat for Round 3.\n", score1, score2, chooser+1)
	var seat game.Seat
	if a.isHuman(chooser) {
		for {
			answer, err := a.ask("Choose X or Y: ")
			if err != nil {
				return err
			}
			switch strings.ToUpper(strings.TrimSpace(answer)) {
			case "X":
				seat = game.SeatX
			case "Y":
				seat = game.SeatY
			default:
				fmt.Println("Enter X or Y.")
				continue
			}
			break
		}
	} else {
		difficulty := a.aiDifficulty()
		seat, _ = ai.ChooseSeat(*state, chooser, difficulty, a.rng)
		fmt.Printf("AI chooses seat %s.\n", seat)
	}
	player, _ := state.Player(chooser)
	opponent, _ := state.Player(1 - chooser)
	player.Seat, opponent.Seat = seat, otherSeat(seat)
	return nil
}

func (a *app) chooseCommit(state game.GameState, playerIndex int) ([]int, error) {
	player, _ := state.Player(playerIndex)
	required := min(len(player.Hand), 3)
	if !a.isHuman(playerIndex) {
		indices, err := ai.ChooseCommit(state, playerIndex, a.aiDifficulty(), a.rng)
		if err != nil {
			return nil, err
		}
		fmt.Printf("P%d commits %d cards.\n", playerIndex+1, len(indices))
		return indices, nil
	}
	for {
		fmt.Printf("Select %d cards to commit to your pool:\n", required)
		a.showCards(player.Hand)
		answer, err := a.ask("Card numbers (space or comma separated): ")
		if err != nil {
			return nil, err
		}
		indices, err := parseIndices(answer, len(player.Hand), required)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Print("Committed: ")
		a.showSelected(player.Hand, indices)
		confirm, err := a.ask("Confirm? (y/n): ")
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(strings.TrimSpace(confirm), "y") {
			return indices, nil
		}
	}
}

func (a *app) choosePool(state game.GameState, playerIndex int, attacking bool, revealed *game.Card) (int, error) {
	player, _ := state.Player(playerIndex)
	if !a.isHuman(playerIndex) {
		index, err := ai.ChoosePoolIndex(state, playerIndex, attacking, revealed, a.aiDifficulty(), a.rng)
		if err != nil {
			return 0, err
		}
		return index, nil
	}
	for {
		fmt.Printf("P%d remaining pool:\n", playerIndex+1)
		a.showCards(player.Pool)
		prompt := "Choose a pool card to defend with: "
		if attacking {
			prompt = "Choose a pool card to attack with: "
		}
		answer, err := a.ask(prompt)
		if err != nil {
			return 0, err
		}
		index, err := strconv.Atoi(strings.TrimSpace(answer))
		if err != nil || index < 1 || index > len(player.Pool) {
			fmt.Println("Enter a valid pool card number.")
			continue
		}
		return index - 1, nil
	}
}

func (a *app) showDecision(state game.GameState, playerIndex int) {
	player, _ := state.Player(playerIndex)
	fmt.Printf("\nP%d — Round %d — scores P1:%d P2:%d — Bounty suit: %s\n",
		playerIndex+1, state.Round, game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total,
		game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total, a.suit(state.Bounty.Suit))
	if a.isHuman(playerIndex) {
		fmt.Print("Hand: ")
		a.showCards(player.Hand)
	} else {
		fmt.Printf("P%d hand: hidden\n", playerIndex+1)
	}
	if a.isHuman(playerIndex) {
		fmt.Print("Pool: ")
		a.showCards(player.Pool)
	} else {
		fmt.Println("Pool: hidden")
	}
}

func (a *app) showCards(cards []game.Card) {
	for i, card := range cards {
		fmt.Printf("%d:%s ", i+1, a.card(card))
	}
	fmt.Println()
}

func (a *app) showSelected(cards []game.Card, indices []int) {
	for _, index := range indices {
		fmt.Printf("%s ", a.card(cards[index]))
	}
	fmt.Println()
}

func (a *app) showOutcome(attackerIndex int, attacker, defender game.Card, outcome game.Outcome) {
	fmt.Printf("[%s = %d] vs [%s = %d] → ", a.card(attacker), outcome.AttackerValue,
		a.card(defender), outcome.DefenderValue)
	switch outcome.Winner {
	case 0:
		fmt.Printf("P%d captures %s.\n", attackerIndex+1, a.card(defender))
	case 1:
		fmt.Printf("P%d captures %s.\n", 2-attackerIndex, a.card(attacker))
	default:
		fmt.Println("clash; both cards discarded.")
	}
}

func (a *app) showScores(state game.GameState) {
	fmt.Printf("Scores — P1: %d | P2: %d\n",
		game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total,
		game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total)
}

func (a *app) showFinal(state game.GameState) int {
	fmt.Println("\n========== FINAL JAILS ==========")
	for index, player := range state.Players() {
		breakdown := game.ScorePlayer(player, state.Bounty.Suit, state.Modifiers)
		fmt.Printf("P%d jail: ", index+1)
		a.showCards(player.Jail)
		for suit := game.Blade; suit <= game.Shield; suit++ {
			count := 0
			for _, card := range player.Jail {
				if card.Suit == suit {
					count++
				}
			}
			fmt.Printf("  %s: %d weighted captures × highest = %d\n", suit, breakdown.WeightedCount[suit], breakdown.BySuit[suit])
		}
		fmt.Printf("  Bounty bonus (%s): %+d; total: %d\n", state.Bounty.Suit, breakdown.BountyBonus, breakdown.Total)
	}
	p1 := game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers)
	p2 := game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers)
	winner, reason := finalWinner(p1, p2, a.rng)
	fmt.Printf("\nP1: %d | P2: %d\nP%d wins by %s.\n", p1.Total, p2.Total, winner+1, reason)
	return winner
}

func finalWinner(p1, p2 game.ScoreBreakdown, rng *rand.Rand) (int, string) {
	switch {
	case p1.Total != p2.Total:
		if p1.Total > p2.Total {
			return 0, "higher total score"
		}
		return 1, "higher total score"
	case p1.DistinctSuits != p2.DistinctSuits:
		if p1.DistinctSuits > p2.DistinctSuits {
			return 0, "more distinct suits"
		}
		return 1, "more distinct suits"
	case p1.HighestCard != p2.HighestCard:
		if p1.HighestCard > p2.HighestCard {
			return 0, "highest single card"
		}
		return 1, "highest single card"
	case rng.Intn(2) == 0:
		return 0, "coin flip"
	default:
		return 1, "coin flip"
	}
}

func (a *app) isHuman(playerIndex int) bool {
	return playerIndex == 0 || a.mode == humanVsHuman
}

func (a *app) aiDifficulty() ai.Difficulty {
	switch a.mode {
	case humanVsEasy:
		return ai.Easy
	case humanVsNormal:
		return ai.Normal
	default:
		return ai.Hard
	}
}

func (a *app) card(card game.Card) string {
	value := card.Code()
	if !colorsEnabled {
		return value
	}
	color := a.ansiSuitColor(card.Suit)
	return color + value + "\033[0m"
}

func (a *app) suit(suit game.Suit) string {
	if !colorsEnabled {
		return suit.String()
	}
	color := a.ansiSuitColor(suit)
	return color + suit.String() + "\033[0m"
}

func (a *app) ansiSuitColor(suit game.Suit) string {
	defaultColors := map[game.Suit]string{game.Blade: "red", game.Fang: "yellow", game.Shield: "blue"}
	color := defaultColors[suit]
	if a.playerProfile != nil {
		suitID := map[game.Suit]string{game.Blade: "B", game.Fang: "F", game.Shield: "S"}[suit]
		assignment := a.playerProfile.AssignedChromas[suitID]
		if chroma, ok := catalog.FindChroma(assignment); ok {
			color = chroma.Color
		} else if assignment != "" {
			color = assignment
		}
	}
	ansi, ok := map[string]string{
		"red": "\033[31m", "crimson": "\033[31m", "blue": "\033[34m", "cobalt": "\033[34m",
		"yellow": "\033[33m", "sunflower": "\033[33m", "green": "\033[32m", "emerald": "\033[32m",
		"orange": "\033[33m", "tangerine": "\033[33m", "purple": "\033[35m", "amethyst": "\033[35m",
		"brown": "\033[33m", "walnut": "\033[33m", "silver": "\033[37m", "rose-gold": "\033[35m",
		"aurora": "\033[36m", "obsidian": "\033[90m", "prismatic": "\033[96m",
	}[color]
	if !ok {
		return "\033[37m"
	}
	return ansi
}

func (a *app) effective(card, opponent game.Card) string {
	value := a.card(card)
	if card.Suit.Beats(opponent.Suit) {
		return value + " ×2"
	}
	return value
}

func (a *app) clearScreen() {
	for i := 0; i < 24; i++ {
		fmt.Println()
	}
}

func (a *app) ask(prompt string) (string, error) {
	fmt.Print(prompt)
	if !a.scanner.Scan() {
		if err := a.scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("input closed")
	}
	return a.scanner.Text(), nil
}

func parseIndices(input string, cardCount, required int) ([]int, error) {
	fields := strings.Fields(strings.NewReplacer(",", " ", ";", " ").Replace(input))
	if len(fields) != required {
		return nil, fmt.Errorf("select exactly %d distinct cards", required)
	}
	indices := make([]int, 0, required)
	seen := make(map[int]bool)
	for _, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 1 || value > cardCount || seen[value-1] {
			return nil, errors.New("use valid, unique card numbers")
		}
		seen[value-1] = true
		indices = append(indices, value-1)
	}
	return indices, nil
}

func otherSeat(seat game.Seat) game.Seat {
	if seat == game.SeatX {
		return game.SeatY
	}
	return game.SeatX
}

func simulate(count int, rng *rand.Rand) error {
	var scoreSum [2]int
	var wins [2]int
	totalPrisoners, totalClashes, bountyPoints := 0, 0, 0
	for gameNumber := 0; gameNumber < count; gameNumber++ {
		reckoning, err := heroes.RollReckoning(rng)
		if err != nil {
			return err
		}
		state, err := game.NewGameWithModifiers(rng, game.GameModifiers{Reckoning: reckoning.Weights})
		if err != nil {
			return err
		}
		for state.Round <= 3 {
			roundPlayed := state.Round
			if state.Round == 3 {
				if err := assignSimulationSeat(&state, [2]ai.Difficulty{ai.Hard, ai.Hard}, rng); err != nil {
					return err
				}
			}
			if err := commitSimulationPlayers(&state, [2]ai.Difficulty{ai.Hard, ai.Hard}, rng); err != nil {
				return err
			}
			for exchange := 0; exchange < 3; exchange++ {
				seat := game.SeatX
				if exchange == 1 {
					seat = game.SeatY
				}
				attacker, _ := state.PlayerAtSeat(seat)
				defender := 1 - attacker
				attackerState, _ := state.Player(attacker)
				defenderState, _ := state.Player(defender)
				if len(attackerState.Pool) == 0 || len(defenderState.Pool) == 0 {
					continue
				}
				attackIndex, err := ai.ChoosePoolIndex(state, attacker, true, nil, ai.Hard, rng)
				if err != nil {
					return err
				}
				attackCard := attackerState.Pool[attackIndex]
				defendIndex, err := ai.ChoosePoolIndex(state, defender, false, &attackCard, ai.Hard, rng)
				if err != nil {
					return err
				}
				if _, err := game.PlayExchange(&state, attacker, attackIndex, defender, defendIndex); err != nil {
					return err
				}
			}
			if err := finishSimulationRound(&state, roundPlayed); err != nil {
				return err
			}
			if roundPlayed == 3 {
				break
			}
		}
		p1 := game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers)
		p2 := game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers)
		scoreSum[0] += p1.Total
		scoreSum[1] += p2.Total
		totalPrisoners += len(state.P1.Jail) + len(state.P2.Jail)
		totalClashes += state.Clashes
		bountyPoints += p1.BountyBonus + p2.BountyBonus
		winner, _ := finalWinner(p1, p2, rng)
		wins[winner]++
	}
	fmt.Printf("Simulated %d Hard-vs-Hard games\n", count)
	fmt.Printf("P1/P2 win rate: %.1f%% / %.1f%%\n", percentage(wins[0], count), percentage(wins[1], count))
	fmt.Printf("Mean scores: P1 %.2f, P2 %.2f\n", float64(scoreSum[0])/float64(count), float64(scoreSum[1])/float64(count))
	fmt.Printf("Mean prisoners: %.2f; mean clashes: %.2f (%.1f%% of exchanges)\n",
		float64(totalPrisoners)/float64(count), float64(totalClashes)/float64(count),
		percentage(totalClashes, count*9))
	fmt.Printf("Bounty contribution: %d points total (%.1f%% of points)\n", bountyPoints,
		percentage(bountyPoints, scoreSum[0]+scoreSum[1]))
	return nil
}

type heroSimulationStats struct {
	p1Wins       int
	p1RandomWins int
	scoreTotal   int
	scoreDiff    int
	stalls       int
	clashes      int
}

func simulateHeroes(gamesPerPair int, rng *rand.Rand) error {
	_, _, err := simulateHeroMatchups(os.Stdout, gamesPerPair, rng)
	return err
}

func simulateHeroMatchups(writer io.Writer, gamesPerPair int, rng *rand.Rand) (int, int, error) {
	if writer == nil {
		return 0, 0, errors.New("hero simulator output is required")
	}
	if gamesPerPair < 1 {
		return 0, 0, errors.New("hero simulator game count must be positive")
	}
	if rng == nil {
		return 0, 0, errors.New("random source is required")
	}
	all := heroes.All()
	passedPairs := 0
	pairCount := 0
	var report strings.Builder
	fmt.Fprintln(&report, "Hero pair | P1 WR | vs random | mean score | mean diff | stall | clash | check")
	fmt.Fprintf(&report, "Simulating all 66 pairings, %d games per strategy (Greedy/Greedy and Greedy/Easy).\n", gamesPerPair)
	for firstIndex, first := range all {
		for _, second := range all[firstIndex:] {
			pairCount++
			stats := heroSimulationStats{}
			for gameNumber := 0; gameNumber < gamesPerPair; gameNumber++ {
				result, err := simulateHeroGame(first, second, ai.Greedy, ai.Greedy, rng)
				if err != nil {
					return passedPairs, pairCount, err
				}
				if result.p1Score > result.p2Score {
					stats.p1Wins++
				}
				stats.scoreTotal += result.p1Score + result.p2Score
				diff := result.p1Score - result.p2Score
				if diff < 0 {
					diff = -diff
				}
				stats.scoreDiff += diff
				stats.stalls += result.stalls
				stats.clashes += result.clashes

				randomResult, err := simulateHeroGame(first, second, ai.Greedy, ai.Easy, rng)
				if err != nil {
					return passedPairs, pairCount, err
				}
				if randomResult.p1Score > randomResult.p2Score {
					stats.p1RandomWins++
				}
			}
			p1WinRate := percentage(stats.p1Wins, gamesPerPair)
			randomWinRate := percentage(stats.p1RandomWins, gamesPerPair)
			meanScore := float64(stats.scoreTotal) / float64(gamesPerPair*2)
			meanDiff := float64(stats.scoreDiff) / float64(gamesPerPair)
			stallRate := percentage(stats.stalls, gamesPerPair*3)
			clashRate := percentage(stats.clashes, gamesPerPair*9)
			passed := p1WinRate >= 46 && p1WinRate <= 54 && randomWinRate >= 65 &&
				meanDiff <= 42 && stallRate <= 5 && clashRate <= 6
			status := "FAIL"
			if passed {
				status = "PASS"
				passedPairs++
			}
			fmt.Fprintf(&report, "%-17s %5.1f%% %9.1f%% %11.2f %10.2f %6.1f%% %6.1f%% %s\n",
				first.ID+" + "+second.ID, p1WinRate, randomWinRate, meanScore, meanDiff, stallRate, clashRate, status)
		}
	}
	fmt.Fprintf(&report, "Passing pairings: %d/%d (target: at least 50)\n", passedPairs, pairCount)
	if _, err := io.WriteString(writer, report.String()); err != nil {
		return passedPairs, pairCount, fmt.Errorf("write hero simulator report: %w", err)
	}
	return passedPairs, pairCount, nil
}

type heroSimulationGameResult struct {
	winner  int
	p1Score int
	p2Score int
	clashes int
	stalls  int
}

func simulateHeroGame(first, second heroes.Hero, p1Difficulty, p2Difficulty ai.Difficulty, rng *rand.Rand) (heroSimulationGameResult, error) {
	gameModifiers, err := heroes.GameModifiersFor(first, second, rng)
	if err != nil {
		return heroSimulationGameResult{}, err
	}
	state, err := game.NewGameWithModifiers(rng, gameModifiers)
	if err != nil {
		return heroSimulationGameResult{}, err
	}
	difficulties := [2]ai.Difficulty{p1Difficulty, p2Difficulty}
	stalls := 0
	for state.Round <= 3 {
		roundPlayed := state.Round
		if roundPlayed == 3 {
			if err := assignSimulationSeat(&state, difficulties, rng); err != nil {
				return heroSimulationGameResult{}, err
			}
		}
		if err := commitSimulationPlayers(&state, difficulties, rng); err != nil {
			return heroSimulationGameResult{}, err
		}
		roundWins := 0
		for exchange := 0; exchange < 3; exchange++ {
			attackerSeat := game.SeatX
			if exchange == 1 {
				attackerSeat = game.SeatY
			}
			attacker, err := state.PlayerAtSeat(attackerSeat)
			if err != nil {
				return heroSimulationGameResult{}, err
			}
			defender := 1 - attacker
			attackerPlayer, _ := state.Player(attacker)
			defenderPlayer, _ := state.Player(defender)
			if len(attackerPlayer.Pool) == 0 || len(defenderPlayer.Pool) == 0 {
				continue
			}
			attackIndex, err := ai.ChoosePoolIndex(state, attacker, true, nil, difficulties[attacker], rng)
			if err != nil {
				return heroSimulationGameResult{}, err
			}
			attackCard := attackerPlayer.Pool[attackIndex]
			defendIndex, err := ai.ChoosePoolIndex(state, defender, false, &attackCard, difficulties[defender], rng)
			if err != nil {
				return heroSimulationGameResult{}, err
			}
			outcome, err := game.PlayExchange(&state, attacker, attackIndex, defender, defendIndex)
			if err != nil {
				return heroSimulationGameResult{}, err
			}
			if outcome.Winner != -1 {
				roundWins++
			}
		}
		if roundWins <= 1 {
			stalls++
		}
		if err := finishSimulationRound(&state, roundPlayed); err != nil {
			return heroSimulationGameResult{}, err
		}
		if roundPlayed == 3 {
			break
		}
	}
	p1 := game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers)
	p2 := game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers)
	winner, _ := finalWinner(p1, p2, rng)
	return heroSimulationGameResult{winner: winner, p1Score: p1.Total, p2Score: p2.Total, clashes: state.Clashes, stalls: stalls}, nil
}

func percentage(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(part) / float64(total)
}

func assignSimulationSeat(state *game.GameState, difficulties [2]ai.Difficulty, rng *rand.Rand) error {
	s1 := game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total
	s2 := game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total
	chooser := 0
	if s2 < s1 || (s1 == s2 && rng.Intn(2) == 1) {
		chooser = 1
	}
	seat, err := ai.ChooseSeat(*state, chooser, difficulties[chooser], rng)
	if err != nil {
		return err
	}
	player, _ := state.Player(chooser)
	opponent, _ := state.Player(1 - chooser)
	player.Seat, opponent.Seat = seat, otherSeat(seat)
	return nil
}

func commitSimulationPlayers(state *game.GameState, difficulties [2]ai.Difficulty, rng *rand.Rand) error {
	for index := 0; index < 2; index++ {
		indices, err := ai.ChooseCommit(*state, index, difficulties[index], rng)
		if err != nil {
			return err
		}
		if err := game.Commit(state, index, indices); err != nil {
			return err
		}
	}
	return nil
}

func finishSimulationRound(state *game.GameState, roundPlayed int) error {
	game.DiscardPools(state)
	if roundPlayed < 3 {
		if err := game.EndRound(state, state.Round == 1, true); err != nil {
			return err
		}
		state.Round++
	}
	return state.Validate()
}
