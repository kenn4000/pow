package main

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"prisonersofwar/achievements"
	"prisonersofwar/ai"
	"prisonersofwar/catalog"
	"prisonersofwar/economy"
	"prisonersofwar/energy"
	"prisonersofwar/game"
	"prisonersofwar/lootbox"
	"prisonersofwar/profile"
	"prisonersofwar/skills"
)

func secureRand() (*rand.Rand, error) {
	var seedBytes [8]byte
	if _, err := crand.Read(seedBytes[:]); err != nil {
		return nil, fmt.Errorf("seed game randomness: %w", err)
	}
	return rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(seedBytes[:])))), nil
}

func refreshEnergy(p *profile.Profile, now time.Time) (int, error) {
	if p == nil {
		return 0, errors.New("profile is required")
	}
	return energy.Current(p, now), nil
}

func dumpEconomyConfig(writer io.Writer) error {
	config := map[string]any{
		"payouts": economy.AuditConfig(),
		"energy": map[string]any{
			"daily_ad_limit":         energy.DailyAdLimit,
			"energy_per_ad":          energy.EnergyPerAd,
			"crowns_per_full_refill": energy.CrownsPerFullRefill,
			"starting_energy":        profile.DefaultMaxEnergy,
			"recharge_per_energy":    profile.DefaultEnergyRecharge.String(),
		},
		"camp_draw_and_catalog": lootbox.AuditConfig(),
		"skills":                skills.AllNodes(),
		"chromas":               catalog.AllChromas(),
		"bundles":               catalog.AllBundles(),
		"achievements":          achievements.All(),
		"achievement_sets":      achievements.AllSets(),
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("encode economy configuration: %w", err)
	}
	return nil
}

func (a *app) initializeProgressionProfile(p *profile.Profile) error {
	now := time.Now()
	loadEvents := achievements.EvaluateAll(achievements.Context{
		Profile: p,
		Event:   achievements.EventProfileLoad,
		Now:     now,
	})
	for _, event := range loadEvents {
		applyUnlockEvent(p, event)
		showUnlockToast(event)
	}
	p.ApplyLogin(now)
	events := achievements.EvaluateAll(achievements.Context{
		Profile: p,
		Event:   achievements.EventLogin,
		Now:     now,
	})
	for _, event := range events {
		applyUnlockEvent(p, event)
		showUnlockToast(event)
	}
	return p.Save()
}

func (a *app) progressionMenu(p *profile.Profile, preferred mode) error {
	if p == nil {
		return errors.New("profile is required")
	}
	a.playerProfile = p
	for {
		a.clearScreen()
		if err := renderLandingPage(os.Stdout, p, time.Now()); err != nil {
			return err
		}
		answer, err := a.ask("Select an action: ")
		if err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "1":
			if err := a.playMenu(p, preferred); err != nil {
				fmt.Println("Unable to start match:", err)
			}
		case "2":
			if p.DrawsAvailable > 0 || p.TotalPendingBoxes() > 0 {
				if err := a.campDrawHub(p); err != nil {
					fmt.Println("Camp Draw unavailable:", err)
				}
			}
		case "3":
			if err := a.shopMenu(p); err != nil {
				fmt.Println("Shop action failed:", err)
			}
		case "4":
			if err := a.skillMenu(p); err != nil {
				fmt.Println("Skill action failed:", err)
			}
		case "5":
			markAchievementsSeen(p)
			if err := p.Save(); err != nil {
				return err
			}
			if err := a.achievementsMenu(p); err != nil {
				fmt.Println("Achievements unavailable:", err)
			}
		case "6":
			a.showProfile(p)
		case "7":
			quitAfterDelete, err := a.settingsMenu(p)
			if err != nil {
				fmt.Println("Settings action failed:", err)
			} else if quitAfterDelete {
				return nil
			}
		case "w":
			if p.Energy >= p.MaxEnergy || p.AdsWatchedToday >= energy.DailyAdLimit {
				fmt.Println("Watch Ad is not available right now.")
				continue
			}
			fmt.Println("Simulating a short ad break...")
			time.Sleep(2 * time.Second)
			if err := energy.WatchAd(p, time.Now()); err != nil {
				fmt.Println("Ad reward unavailable:", err)
			} else {
				fmt.Printf("Ad reward: +%d energy (now %d/%d).\n", energy.EnergyPerAd, p.Energy, p.MaxEnergy)
				if err := p.Save(); err != nil {
					return err
				}
			}
		case "r":
			if err := a.energyRefillMenu(p); err != nil {
				fmt.Println("Energy refill unavailable:", err)
			}
		case "q":
			if p.ConfirmQuit {
				answer, err := a.ask("Quit Prisoners of War? (y/N): ")
				if err != nil {
					return err
				}
				if !strings.EqualFold(strings.TrimSpace(answer), "y") &&
					!strings.EqualFold(strings.TrimSpace(answer), "yes") {
					continue
				}
			}
			if err := p.Save(); err != nil {
				return err
			}
			return nil
		default:
			fmt.Println("Choose 1-7, W, R, or Q.")
		}
	}
}

func (a *app) playMenu(p *profile.Profile, preferred mode) error {
	selected := preferred
	if selected < humanVsEasy || selected > humanVsHard {
		selected = humanVsEasy
	}
	fmt.Println("\n=== PLAY ===")
	fmt.Printf("[1] Casual CPU match (selected mode: %s)\n", modeName(selected))
	fmt.Println("[2] Easy AI")
	fmt.Println("[3] Normal AI")
	fmt.Println("[4] Hard AI")
	fmt.Println("[5] Ranked best-of-three vs Hard AI")
	fmt.Println("[6] Friend casual (local)")
	fmt.Println("[7] Friend-ranked best-of-three (local)")
	fmt.Println("[0] Back")
	answer, err := a.ask("Choose a mode: ")
	if err != nil {
		return err
	}
	var selectedMode mode
	var payoutMode economy.Mode
	bestOfThree := false
	switch strings.TrimSpace(answer) {
	case "0":
		return nil
	case "1":
		selectedMode = selected
		payoutMode, err = economyModeForCPU(selectedMode, false)
	case "2":
		selectedMode, payoutMode = humanVsEasy, economy.ModeCPUEasy
	case "3":
		selectedMode, payoutMode = humanVsNormal, economy.ModeCPUNormal
	case "4":
		selectedMode, payoutMode = humanVsHard, economy.ModeCPUHard
	case "5":
		selectedMode, payoutMode, bestOfThree = humanVsHard, economy.ModeRanked, true
	case "6":
		selectedMode, payoutMode = humanVsHuman, economy.ModeFriendCasual
	case "7":
		selectedMode, payoutMode, bestOfThree = humanVsHuman, economy.ModeFriendRanked, true
	default:
		fmt.Println("Enter a listed mode number.")
		return nil
	}
	if err != nil {
		return err
	}
	return a.startMatch(p, selectedMode, payoutMode, bestOfThree)
}

func modeName(selected mode) string {
	switch selected {
	case humanVsEasy:
		return "Easy AI"
	case humanVsNormal:
		return "Normal AI"
	case humanVsHard:
		return "Hard AI"
	default:
		return "Easy AI"
	}
}

func economyModeForCPU(selected mode, ranked bool) (economy.Mode, error) {
	if ranked {
		return economy.ModeRanked, nil
	}
	switch selected {
	case humanVsEasy:
		return economy.ModeCPUEasy, nil
	case humanVsNormal:
		return economy.ModeCPUNormal, nil
	case humanVsHard:
		return economy.ModeCPUHard, nil
	default:
		return "", errors.New("selected mode is not a CPU mode")
	}
}

func (a *app) startMatch(p *profile.Profile, selected mode, payoutMode economy.Mode, bestOfThree bool) error {
	if _, err := refreshEnergy(p, time.Now()); err != nil {
		return err
	}
	if err := energy.Spend(p, 1, time.Now()); err != nil {
		return err
	}
	if err := p.Save(); err != nil {
		return err
	}
	a.mode = selected
	var result economy.MatchResult
	var achievementResult game.MatchResult
	var err error
	if bestOfThree {
		result, achievementResult, err = a.playBestOfThree()
	} else {
		result, achievementResult, err = a.playSingleMatch()
	}
	if err != nil {
		return err
	}
	now := time.Now()
	payout, err := economy.CalculatePayout(result, payoutMode, p, result.PlayerSeat, now)
	if err != nil {
		return err
	}
	updateAchievementStats(p, achievementResult, payoutMode)
	ctx := achievements.NewMatchContext(p, achievementResult, economy.ModeID(payoutMode), achievementResult.PlayerSeat.String())
	ctx.Now = now
	unlockEvents := achievements.EvaluateAll(ctx)
	for _, event := range unlockEvents {
		applyUnlockEvent(p, event)
	}
	p.RecordMatch(buildMatchRecord(ctx, payoutMode, payout.TotalMarks, now))
	if err := p.Save(); err != nil {
		return err
	}
	switch {
	case payout.PlayerWon:
		fmt.Printf("\nYOU WON — +%d Marks, +%d Crowns", payout.TotalMarks, payout.CrownsAwarded)
	case payout.Draw:
		fmt.Printf("\nDRAW — +%d Marks", payout.TotalMarks)
	default:
		fmt.Printf("\nYOU LOST — +%d Marks", payout.TotalMarks)
	}
	if payout.CampDrawAwarded {
		fmt.Print(" | Camp Draw earned")
	}
	fmt.Println()
	for _, event := range unlockEvents {
		showUnlockToast(event)
	}
	return nil
}

func (a *app) playSingleMatch() (economy.MatchResult, game.MatchResult, error) {
	state, err := a.playGame()
	if err != nil {
		return economy.MatchResult{}, game.MatchResult{}, err
	}
	p1Score := game.Score(state.P1.Jail, state.Bounty.Suit).Total
	p2Score := game.Score(state.P2.Jail, state.Bounty.Suit).Total
	summary := a.matchGameSummary(state)
	winnerSeat := int(state.P1.Seat)
	if a.lastWinner == 1 {
		winnerSeat = int(state.P2.Seat)
	}
	economyResult := economy.MatchResult{
		WinnerSeat: winnerSeat, PlayerSeat: int(state.P1.Seat),
		PlayerScore: p1Score, OpponentScore: p2Score,
	}
	achievementResult := game.MatchResult{
		Won: a.lastWinner == 0, Tied: p1Score == p2Score, PlayerSeat: state.P1.Seat,
		PlayerScore: p1Score, OpponentScore: p2Score, Games: []game.MatchGame{summary},
	}
	return economyResult, achievementResult, nil
}

func (a *app) playBestOfThree() (economy.MatchResult, game.MatchResult, error) {
	var wins [2]int
	var totals [2]int
	var firstSeats [2]game.Seat
	var finalState game.GameState
	matchGames := make([]game.MatchGame, 0, 3)
	for matchGame := 0; matchGame < 3 && wins[0] < 2 && wins[1] < 2; matchGame++ {
		state, err := game.NewGame(a.rng)
		if err != nil {
			return economy.MatchResult{}, game.MatchResult{}, err
		}
		switch matchGame {
		case 1:
			if err := assignInitialSeats(&state, otherSeat(firstSeats[0])); err != nil {
				return economy.MatchResult{}, game.MatchResult{}, err
			}
		case 2:
			chooser := 0
			if totals[1] < totals[0] {
				chooser = 1
			} else if totals[0] == totals[1] {
				chooser = a.rng.Intn(2)
			}
			seat, err := a.chooseMatchSeat(state, chooser, totals)
			if err != nil {
				return economy.MatchResult{}, game.MatchResult{}, err
			}
			if err := assignInitialSeats(&state, seat); err != nil {
				return economy.MatchResult{}, game.MatchResult{}, err
			}
		}
		if matchGame == 0 {
			firstSeats = [2]game.Seat{state.P1.Seat, state.P2.Seat}
		}
		fmt.Printf("\n===== Ranked game %d of 3 =====\n", matchGame+1)
		finalState, err = a.playGameState(state)
		if err != nil {
			return economy.MatchResult{}, game.MatchResult{}, err
		}
		scores := [2]int{
			game.Score(finalState.P1.Jail, finalState.Bounty.Suit).Total,
			game.Score(finalState.P2.Jail, finalState.Bounty.Suit).Total,
		}
		totals[0] += scores[0]
		totals[1] += scores[1]
		wins[a.lastWinner]++
		matchGames = append(matchGames, a.matchGameSummary(finalState))
	}
	fmt.Printf("\nMatch record P1 %d-%d P2 | total scores %d-%d\n", wins[0], wins[1], totals[0], totals[1])
	winnerSeat := -1
	if wins[0] > wins[1] {
		winnerSeat = int(finalState.P1.Seat)
	} else if wins[1] > wins[0] {
		winnerSeat = int(finalState.P2.Seat)
	}
	economyResult := economy.MatchResult{
		WinnerSeat: winnerSeat, PlayerSeat: int(finalState.P1.Seat),
		PlayerScore: totals[0], OpponentScore: totals[1],
	}
	achievementResult := game.MatchResult{
		Won: wins[0] > wins[1], Tied: totals[0] == totals[1], PlayerSeat: finalState.P1.Seat,
		PlayerScore: totals[0], OpponentScore: totals[1], Games: matchGames,
	}
	return economyResult, achievementResult, nil
}

func (a *app) matchGameSummary(state game.GameState) game.MatchGame {
	return game.MatchGame{
		Won: a.lastWinner == 0, PlayerSeat: state.P1.Seat,
		PlayerScore:   game.Score(state.P1.Jail, state.Bounty.Suit).Total,
		OpponentScore: game.Score(state.P2.Jail, state.Bounty.Suit).Total,
		PlayerJail:    append([]game.Card(nil), state.P1.Jail...),
		OpponentJail:  append([]game.Card(nil), state.P2.Jail...),
		BountySuit:    state.Bounty.Suit, Clashes: state.Clashes,
		ScoreAfterR2: a.gameScoreAfterR2[0], OpponentAfterR2: a.gameScoreAfterR2[1],
		SuitDoublingWin: a.gameSuitDoublingWin, RoundSweeps: a.gameRoundSweeps,
	}
}

func (a *app) chooseMatchSeat(state game.GameState, chooser int, totals [2]int) (game.Seat, error) {
	fmt.Printf("After two ranked games: cumulative scores P1 %d, P2 %d. P%d chooses a seat.\n",
		totals[0], totals[1], chooser+1)
	if a.isHuman(chooser) {
		for {
			answer, err := a.ask("Choose X or Y for the deciding game: ")
			if err != nil {
				return 0, err
			}
			switch strings.ToUpper(strings.TrimSpace(answer)) {
			case "X":
				return game.SeatX, nil
			case "Y":
				return game.SeatY, nil
			default:
				fmt.Println("Enter X or Y.")
			}
		}
	}
	seat, err := ai.ChooseSeat(state, chooser, a.aiDifficulty(), a.rng)
	if err != nil {
		return 0, err
	}
	fmt.Printf("AI chooses seat %s.\n", seat)
	return seat, nil
}

func assignInitialSeats(state *game.GameState, p1Seat game.Seat) error {
	if p1Seat != game.SeatX && p1Seat != game.SeatY {
		return fmt.Errorf("invalid initial seat %d", p1Seat)
	}
	state.P1.Seat, state.P2.Seat = p1Seat, otherSeat(p1Seat)
	targetP1, targetP2 := 5, 6
	if state.P1.Seat == game.SeatX {
		targetP1, targetP2 = 6, 5
	}
	for len(state.P1.Hand) > targetP1 {
		last := len(state.P1.Hand) - 1
		state.Deck = append(state.Deck, state.P1.Hand[last])
		state.P1.Hand = state.P1.Hand[:last]
	}
	for len(state.P1.Hand) < targetP1 {
		if len(state.Deck) == 0 {
			return errors.New("not enough cards to reassign ranked seats")
		}
		last := len(state.Deck) - 1
		state.P1.Hand = append(state.P1.Hand, state.Deck[last])
		state.Deck = state.Deck[:last]
	}
	for len(state.P2.Hand) > targetP2 {
		last := len(state.P2.Hand) - 1
		state.Deck = append(state.Deck, state.P2.Hand[last])
		state.P2.Hand = state.P2.Hand[:last]
	}
	for len(state.P2.Hand) < targetP2 {
		if len(state.Deck) == 0 {
			return errors.New("not enough cards to reassign ranked seats")
		}
		last := len(state.Deck) - 1
		state.P2.Hand = append(state.P2.Hand, state.Deck[last])
		state.Deck = state.Deck[:last]
	}
	if err := state.Validate(); err != nil {
		return fmt.Errorf("validate ranked seat assignment: %w", err)
	}
	return nil
}

func (a *app) showProfile(p *profile.Profile) {
	now := time.Now()
	current, _ := refreshEnergy(p, now)
	fmt.Println("\n=== PLAYER PROFILE ===")
	fmt.Printf("ID: %s\n", p.PlayerID)
	fmt.Printf("Matches: %d wins, %d losses, %d draws | streak %d (best %d)\n",
		p.TotalWins, p.TotalLosses, p.TotalDraws, p.CurrentStreak, p.BestStreak)
	fmt.Printf("Marks: %d | Crowns: %d\n", p.Marks, p.Crowns)
	fmt.Printf("Skill Tree Points: %d | Chroma Pick tokens: %d | Badges: %d\n",
		p.SkillPoints, p.ChromaPickTokens, len(p.Badges))
	fmt.Printf("Energy: %d/%d", current, p.MaxEnergy)
	if wait := energy.TimeUntilNext(p, now); wait > 0 {
		fmt.Printf(" | next energy in %s", wait.Round(time.Second))
	}
	fmt.Printf(" | ads today: %d/%d\n", p.AdsWatchedToday, energy.DailyAdLimit)
	fmt.Printf("Camp Draws: %d available | %d consecutive wins toward next draw (need %d)\n",
		p.DrawsAvailable, p.WinsSinceCampDraw, lootbox.RequiredWins(p))
	owned := 0
	for _, value := range p.OwnedChromas {
		if value {
			owned++
		}
	}
	fmt.Printf("Chromas owned: %d | skills unlocked: %d\n", owned, len(p.SkillNodes))
	fmt.Printf("Suit colors: Scissors %s, Paper %s, Rock %s\n",
		colorAssignment(p, "B"), colorAssignment(p, "F"), colorAssignment(p, "S"))
	if err := p.Save(); err != nil {
		fmt.Println("Could not save refreshed profile:", err)
	}
}

func colorAssignment(p *profile.Profile, suit string) string {
	assignment := p.AssignedChromas[suit]
	if chroma, ok := catalog.FindChroma(assignment); ok {
		return chroma.Color
	}
	return assignment
}

func (a *app) shopMenu(p *profile.Profile) error {
	fmt.Printf("\n=== CHROMA CATALOG | Marks %d | Crowns %d ===\n", p.Marks, p.Crowns)
	fmt.Println("Real-money equivalent: not configured; Crowns are informational in this local build.")
	for _, chroma := range catalog.AllChromas() {
		if p.OwnedChromas[chroma.ID] {
			if err := catalog.MarkChromaSeen(p, chroma.ID); err != nil {
				return err
			}
		}
		fmt.Printf("%-12s %-14s %-10s %d %s", chroma.ID, chroma.Name, chroma.Rarity, chroma.Cost, chroma.Currency)
		if p.OwnedChromas[chroma.ID] {
			fmt.Print(" (owned)")
		}
		fmt.Println()
	}
	if err := p.Save(); err != nil {
		return err
	}
	for _, bundle := range catalog.AllBundles() {
		fmt.Printf("bundle:%s — %s (%d Crowns; includes %s)\n",
			bundle.ID, bundle.Name, bundle.CostCrowns, strings.Join(bundle.ChromaIDs, ", "))
	}
	if p.ChromaPickTokens > 0 {
		fmt.Printf("[Redeem Pick-Any-Chroma token (%d available)]\n", p.ChromaPickTokens)
	}
	answer, err := a.ask("Enter a chroma ID, bundle:<id>, 'pick' to redeem a token, or blank to return: ")
	if err != nil {
		return err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil
	}
	if strings.EqualFold(answer, "pick") {
		if p.ChromaPickTokens < 1 {
			return errors.New("no Pick Any Chroma tokens are available")
		}
		for _, chroma := range catalog.AllChromas() {
			state := ""
			if p.OwnedChromas[chroma.ID] {
				state = " (owned)"
			}
			fmt.Printf("%-12s %-14s %-10s%s\n", chroma.ID, chroma.Name, chroma.Rarity, state)
		}
		chromaID, err := a.ask("Choose an unowned chroma to claim: ")
		if err != nil {
			return err
		}
		if err := redeemPickAny(p, strings.TrimSpace(chromaID)); err != nil {
			return err
		}
		fmt.Printf("Claimed %s.\n", strings.TrimSpace(chromaID))
		return p.Save()
	}
	if strings.HasPrefix(answer, "bundle:") {
		if err := catalog.BuyBundle(p, strings.TrimPrefix(answer, "bundle:")); err != nil {
			return err
		}
	} else {
		if err := catalog.BuyChroma(p, answer); err != nil {
			return err
		}
		if chroma, ok := catalog.FindChroma(answer); ok {
			fmt.Printf("Purchased %s (%s).\n", chroma.Name, chroma.Color)
			suit, err := a.ask("Assign it to B (Scissors), F (Paper), S (Rock), or blank to skip: ")
			if err != nil {
				return err
			}
			if strings.TrimSpace(suit) != "" {
				if err := catalog.AssignChroma(p, suit, answer); err != nil {
					fmt.Println("Color assignment failed:", err)
				}
			}
		}
	}
	events, err := updatePurchaseAchievements(p, time.Now())
	if err != nil {
		return err
	}
	for _, event := range events {
		showUnlockToast(event)
	}
	return nil
}

func (a *app) skillMenu(p *profile.Profile) error {
	fmt.Printf("\n=== SKILL TREE | Crowns %d | Skill Tree Points %d ===\n", p.Crowns, p.SkillPoints)
	for _, node := range skills.AllNodes() {
		state := "locked"
		if p.SkillNodes[node.ID] {
			state = "unlocked"
		} else if can, _ := skills.CanUnlock(p, node.ID); can {
			state = "available"
		} else if p.SkillPoints > 0 {
			if can, _ := skills.CanUnlockWithSkillPoint(p, node.ID); can {
				state = "available (SP)"
			}
		}
		fmt.Printf("%-18s %-10s %2d Crowns — %s", node.ID, state, node.CostCrowns, node.Description)
		if node.Prerequisite != "" {
			fmt.Printf(" (requires %s)", node.Prerequisite)
		}
		fmt.Println()
	}
	answer, err := a.ask("Enter a skill node ID to unlock, or blank to return: ")
	if err != nil {
		return err
	}
	if strings.TrimSpace(answer) == "" {
		return nil
	}
	id := strings.TrimSpace(answer)
	payment := "crowns"
	if p.SkillPoints > 0 {
		payment, err = a.ask("Unlock with Crowns (C) or one Skill Tree Point (S)? ")
		if err != nil {
			return err
		}
	}
	switch strings.ToLower(strings.TrimSpace(payment)) {
	case "c", "crowns":
		if err := skills.UnlockNode(p, id); err != nil {
			return err
		}
	case "s", "skill point", "skill-point":
		if err := skills.UnlockNodeWithSkillPoint(p, id); err != nil {
			return err
		}
	default:
		return errors.New("choose C for Crowns or S for a Skill Tree Point")
	}
	return p.Save()
}
