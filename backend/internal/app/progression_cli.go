package app

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/ai"
	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
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
		choice := strings.ToLower(strings.TrimSpace(answer))
		if a.runSimpleProgressionAction(choice, p, preferred) {
			continue
		}
		switch choice {
		case "2":
			a.tryCampDrawHub(p)
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
			if err := a.watchProgressionAd(p); err != nil {
				return err
			}
		case "q":
			confirmed, err := a.confirmQuit(p)
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			return p.Save()
		default:
			fmt.Println("Choose 1-9, W, R, or Q.")
		}
	}
}

func (a *app) tryCampDrawHub(p *profile.Profile) {
	if p.DrawsAvailable > 0 || p.TotalPendingBoxes() > 0 {
		if err := a.campDrawHub(p); err != nil {
			fmt.Println("Camp Draw unavailable:", err)
		}
	}
}

func (a *app) runSimpleProgressionAction(choice string, p *profile.Profile, preferred mode) bool {
	type action struct {
		label string
		run   func() error
	}
	actions := map[string]action{
		"1": {"Unable to start match:", func() error { return a.playMenu(p, preferred) }},
		"3": {"Shop action failed:", func() error { return a.shopMenu(p) }},
		"4": {"Skill action failed:", func() error { return a.skillMenu(p) }},
		"8": {"Heroes unavailable:", func() error { return a.heroesMenu(p) }},
		"9": {"Campaign unavailable:", func() error { return a.campaignsMenu(p) }},
		"r": {"Energy refill unavailable:", func() error { return a.energyRefillMenu(p) }},
	}
	selected, ok := actions[choice]
	if !ok {
		return false
	}
	if err := selected.run(); err != nil {
		fmt.Println(selected.label, err)
	}
	return true
}

func (a *app) watchProgressionAd(p *profile.Profile) error {
	if p.Energy >= p.MaxEnergy || p.AdsWatchedToday >= energy.DailyAdLimit {
		fmt.Println("Watch Ad is not available right now.")
		return nil
	}
	fmt.Println("Simulating a short ad break...")
	time.Sleep(2 * time.Second)
	if err := energy.WatchAd(p, time.Now()); err != nil {
		fmt.Println("Ad reward unavailable:", err)
		return nil
	}
	fmt.Printf("Ad reward: +%d energy (now %d/%d).\n", energy.EnergyPerAd, p.Energy, p.MaxEnergy)
	return p.Save()
}

func (a *app) confirmQuit(p *profile.Profile) (bool, error) {
	if !p.ConfirmQuit {
		return true, nil
	}
	answer, err := a.ask("Quit Prisoners of War? (y/N): ")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes"), nil
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

func (a *app) campaignsMenu(p *profile.Profile) error {
	if p == nil {
		return errors.New("profile is required")
	}
	for {
		a.clearScreen()
		fmt.Println("=== CAMPAIGNS ===")
		allCampaigns := campaigns.All()
		for index, campaign := range allCampaigns {
			progress := p.Campaigns[campaign.ID]
			state := "unlocked"
			completed := 0
			if progress != nil {
				completed = progress.MissionsCompleted
				if progress.Completed {
					state = "complete"
				}
			}
			if !campaigns.IsUnlocked(p, campaign.ID) {
				state = "Locked - complete prior campaign"
			}
			fmt.Printf("[%d] %s — %d/%d missions (%s)\n",
				index+1, campaign.Name, completed, len(campaign.Missions), state)
		}
		fmt.Println("[0] Back")
		answer, err := a.ask("Choose a campaign: ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(answer) == "0" {
			return nil
		}
		index, err := strconv.Atoi(strings.TrimSpace(answer))
		if err != nil || index < 1 || index > len(allCampaigns) {
			fmt.Println("Enter a listed campaign number.")
			continue
		}
		campaign := allCampaigns[index-1]
		if !campaigns.IsUnlocked(p, campaign.ID) {
			fmt.Println("Complete Basic Training to unlock Advanced Training.")
			continue
		}
		if err := campaigns.Start(p, campaign.ID); err != nil {
			return err
		}
		if err := p.Save(); err != nil {
			return err
		}
		if err := a.campaignMissionsMenu(p, campaign); err != nil {
			return err
		}
	}
}

func (a *app) campaignMissionsMenu(p *profile.Profile, campaign campaigns.Campaign) error {
	for {
		progress := p.Campaigns[campaign.ID]
		completed := 0
		if progress != nil {
			completed = progress.MissionsCompleted
		}
		fmt.Printf("\n=== %s: %d/%d missions ===\n%s\n",
			campaign.Name, completed, len(campaign.Missions), campaign.Description)
		printCampaignMissions(campaign, progress)
		if completed < len(campaign.Missions) {
			fmt.Println("[P] Play Next Mission")
		} else {
			fmt.Println("Campaign complete.")
		}
		if completed > 0 {
			fmt.Println("[R] Replay Any Completed Mission")
		}
		fmt.Println("[0] Back")
		answer, err := a.ask("Choose an action: ")
		if err != nil {
			return err
		}
		switch strings.ToUpper(strings.TrimSpace(answer)) {
		case "0":
			return nil
		case "P":
			if completed >= len(campaign.Missions) {
				fmt.Println("This campaign has no remaining missions.")
				continue
			}
			mission, ok := findCampaignMission(campaign, completed+1)
			if !ok || !campaigns.CanPlayMission(p, campaign.ID, completed+1) {
				return fmt.Errorf("campaign progress for %q is inconsistent", campaign.ID)
			}
			if err := a.startCampaignMission(p, campaign, mission, false); err != nil {
				fmt.Println("Mission could not be started:", err)
			}
		case "R":
			if completed == 0 {
				fmt.Println("Complete a mission before replaying it.")
				continue
			}
			mission, err := a.chooseCompletedCampaignMission(campaign, progress)
			if err != nil {
				return err
			}
			if mission.Index == 0 {
				continue
			}
			if err := a.startCampaignMission(p, campaign, mission, true); err != nil {
				fmt.Println("Mission could not be started:", err)
			}
		default:
			fmt.Println("Enter P, R, or 0.")
		}
	}
}

func printCampaignMissions(campaign campaigns.Campaign, progress *profile.CampaignProgress) {
	for _, mission := range campaign.Missions {
		state := "locked"
		if progress != nil && missionIsComplete(progress, mission.Index) {
			state = "complete"
		} else if progress == nil && mission.Index == 1 || progress != nil && !progress.Completed && mission.Index == progress.MissionsCompleted+1 {
			state = "next"
		}
		fmt.Printf("[%d] %s — %s (%s)\n", mission.Index, mission.Name, mission.Flavor, state)
	}
}

func (a *app) chooseCompletedCampaignMission(campaign campaigns.Campaign, progress *profile.CampaignProgress) (campaigns.Mission, error) {
	fmt.Println("Completed missions:")
	for _, mission := range campaign.Missions {
		if missionIsComplete(progress, mission.Index) {
			fmt.Printf("[%d] %s\n", mission.Index, mission.Name)
		}
	}
	for {
		answer, err := a.ask("Mission number (or 0 to cancel): ")
		if err != nil {
			return campaigns.Mission{}, err
		}
		if strings.TrimSpace(answer) == "0" {
			return campaigns.Mission{}, nil
		}
		index, err := strconv.Atoi(strings.TrimSpace(answer))
		if err == nil && missionIsComplete(progress, index) {
			if mission, ok := findCampaignMission(campaign, index); ok {
				return mission, nil
			}
		}
		fmt.Println("Choose a completed mission number.")
	}
}

func findCampaignMission(campaign campaigns.Campaign, index int) (campaigns.Mission, bool) {
	for _, mission := range campaign.Missions {
		if mission.Index == index {
			return mission, true
		}
	}
	return campaigns.Mission{}, false
}

func missionIsComplete(progress *profile.CampaignProgress, index int) bool {
	if progress == nil {
		return false
	}
	for _, completed := range progress.CompletedMissionIDs {
		if completed == index {
			return true
		}
	}
	return false
}

func (a *app) chooseCampaignHero(p *profile.Profile, mission campaigns.Mission) (heroes.Hero, error) {
	available := make([]heroes.Hero, 0, len(mission.PlayerHeroRestriction))
	for _, heroID := range mission.PlayerHeroRestriction {
		if !containsHeroID(p.UnlockedHeroes, heroID) {
			continue
		}
		hero, ok := heroes.ByID(heroID)
		if ok {
			available = append(available, hero)
		}
	}
	if len(available) == 0 {
		return heroes.Hero{}, errors.New("no permitted campaign hero is unlocked")
	}
	if len(available) == 1 {
		return available[0], nil
	}
	fmt.Println("Choose a hero for this mission:")
	for index, hero := range available {
		fmt.Printf("[%d] %s\n", index+1, hero.Name)
	}
	for {
		answer, err := a.ask("Hero number: ")
		if err != nil {
			return heroes.Hero{}, err
		}
		index, err := strconv.Atoi(strings.TrimSpace(answer))
		if err == nil && index >= 1 && index <= len(available) {
			return available[index-1], nil
		}
		fmt.Printf("Enter a number from 1 to %d.\n", len(available))
	}
}

func (a *app) startCampaignMission(p *profile.Profile, campaign campaigns.Campaign, mission campaigns.Mission, replay bool) error {
	if !replay {
		if _, err := refreshEnergy(p, time.Now()); err != nil {
			return err
		}
		if energyCost := campaignMissionEnergyCost(p, mission); energyCost > 0 {
			if err := energy.Spend(p, energyCost, time.Now()); err != nil {
				return err
			}
		}
		if err := p.Save(); err != nil {
			return err
		}
	}
	ui := &campaignMissionUI{app: a, profile: p}
	for {
		result, err := campaigns.RunMission(p, mission, a.rng, ui)
		if err != nil {
			return err
		}
		a.playerHeroID, a.opponentHeroID = result.PlayerHeroID, result.CPUHeroID
		a.playerTuning, a.opponentTuning = ui.playerTuning, nil
		now := time.Now()
		matchResult := result.MatchResult
		recordHeroProgress(p, matchResult, a.playerHeroID, a.opponentHeroID)
		updateAchievementStats(p, matchResult, economy.ModeCPUEasy)
		switch {
		case matchResult.Won:
			p.TotalWins++
		case matchResult.Tied:
			p.TotalDraws++
		default:
			p.TotalLosses++
		}
		ctx := achievements.NewMatchContext(p, matchResult, economy.ModeID(economy.ModeCPUEasy), matchResult.PlayerSeat.String())
		ctx.Now = now
		for _, event := range achievements.EvaluateAll(ctx) {
			applyUnlockEvent(p, event)
			showUnlockToast(event)
		}
		p.RecordMatch(buildMatchRecord(ctx, economy.ModeCPUEasy, 0, now))
		if !result.Won {
			if err := p.Save(); err != nil {
				return err
			}
			fmt.Printf("%s was not completed. Review the hint and try again.\n", mission.Name)
			fmt.Printf("Hint: %s\n", mission.Hint)
			answer, err := a.ask("Retry now? (y/n): ")
			if err != nil {
				return err
			}
			if strings.EqualFold(strings.TrimSpace(answer), "y") {
				continue
			}
			return nil
		}
		if replay {
			fmt.Printf("Practice replay complete: %d-%d. No Marks or progression rewards were awarded.\n",
				result.PlayerScore, result.CPUScore)
			return p.Save()
		}
		return a.completeCampaignProgress(p, campaign, mission)
	}
}

func (a *app) completeCampaignProgress(p *profile.Profile, campaign campaigns.Campaign, mission campaigns.Mission) error {
	outcome, err := campaigns.CompleteMission(
		p, campaign.ID, mission.Index, skills.AggregateBonuses(p).CampaignMarksPercent)
	if err != nil {
		return err
	}
	fmt.Printf("Mission complete! +%d Marks.\n", outcome.MissionMarks)
	if outcome.CampaignCompleted {
		fmt.Printf("\n=== CAMPAIGN COMPLETE: %s ===\n", campaign.Name)
		fmt.Printf("Completion reward: +%d Marks.\n", outcome.CompletionMarks)
		for _, heroID := range outcome.HeroesUnlocked {
			for _, event := range evaluateAchievementEvent(p, achievements.EventHeroAcquired, "campaign") {
				showUnlockToast(event)
			}
			fmt.Printf("Unlocked %s.\n", heroName(heroID))
		}
		if outcome.HeroBoxesAwarded > 0 {
			fmt.Printf("Received %d Hero Box(es).\n", outcome.HeroBoxesAwarded)
		}
		if outcome.BadgeAwarded != "" {
			fmt.Printf("Badge earned: %s\n", outcome.BadgeAwarded)
		}
		if campaign.Completion.UnlocksCampaign != "" {
			if next, ok := campaigns.ByID(campaign.Completion.UnlocksCampaign); ok {
				fmt.Printf("Unlocked campaign: %s.\n", next.Name)
			}
		}
		for _, event := range evaluateAchievementEvent(p, achievements.EventCampaignCompleted, "campaign") {
			showUnlockToast(event)
		}
		for _, event := range evaluateAchievementEvent(p, achievements.EventProfileLoad, "campaign") {
			showUnlockToast(event)
		}
		if len(p.MatchHistory) > 0 {
			p.MatchHistory[0].MarksEarned = outcome.MissionMarks + outcome.CompletionMarks
		}
		fmt.Println("Press Enter to continue.")
		if _, err := a.ask(""); err != nil {
			return err
		}
	}
	return p.Save()
}

type campaignMissionUI struct {
	app          *app
	profile      *profile.Profile
	playerTuning heroes.Tuning
}

func (ui *campaignMissionUI) ShowBriefing(mission campaigns.Mission) {
	fmt.Printf("\n=== %s ===\n%s\n\nHint: %s\n", mission.Name, mission.Flavor, mission.Hint)
}

func (ui *campaignMissionUI) ShowStartingHand(playerCards []game.Card) {
	fmt.Print("\nYour hand: ")
	ui.app.showCards(playerCards)
	fmt.Println()
}

func (ui *campaignMissionUI) ChooseHeroForMission(allowed []string) (string, error) {
	hero, err := ui.app.chooseCampaignHero(ui.profile, campaigns.Mission{PlayerHeroRestriction: allowed})
	if err != nil {
		return "", err
	}
	return hero.ID, nil
}

func (ui *campaignMissionUI) ChooseCommit(hand []game.Card, count int) ([]int, error) {
	for {
		fmt.Printf("Select %d cards to commit to your pool:\n", count)
		ui.app.showCards(hand)
		answer, err := ui.app.ask("Card numbers (space or comma separated): ")
		if err != nil {
			return nil, err
		}
		indices, err := parseIndices(answer, len(hand), count)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Print("Committed: ")
		ui.app.showSelected(hand, indices)
		confirm, err := ui.app.ask("Confirm? (y/n): ")
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(strings.TrimSpace(confirm), "y") {
			return indices, nil
		}
	}
}

func (ui *campaignMissionUI) ChoosePoolCard(exchange int, attacking bool, revealed *game.Card, pool []game.Card) (int, error) {
	for {
		fmt.Printf("\nYour remaining pool for exchange %d:\n", exchange)
		ui.app.showCards(pool)
		prompt := "Choose a pool card to defend with: "
		if attacking {
			prompt = "Choose a pool card to attack with: "
		} else if revealed != nil {
			prompt = fmt.Sprintf("CPU revealed %s; choose a pool card to defend with: ", ui.app.card(*revealed))
		}
		answer, err := ui.app.ask(prompt)
		if err != nil {
			return 0, err
		}
		index, err := strconv.Atoi(strings.TrimSpace(answer))
		if err == nil && index >= 1 && index <= len(pool) {
			return index - 1, nil
		}
		fmt.Println("Enter a valid pool card number.")
	}
}

func (ui *campaignMissionUI) ShowMissionSetup(state game.GameState) {
	mods := state.Modifiers
	fmt.Printf("Bounty card: %s | Bounty %+d | Shift %+d | Favored %+d | Round weights %d/%d/%d\n",
		ui.app.card(state.Bounty), mods.Bounty, mods.Shift, mods.Favored,
		mods.Reckoning[0], mods.Reckoning[1], mods.Reckoning[2])
	if state.Round == 1 {
		fmt.Println("CPU cards remain hidden until played.")
	}
}

func (ui *campaignMissionUI) ShowBattleResult(exchange int, attacker, defender game.Card, outcome game.Outcome, attackerIsPlayer bool) {
	fmt.Printf("\nExchange %d: %s vs %s. ", exchange, ui.app.card(attacker), ui.app.card(defender))
	switch outcome.Winner {
	case -1:
		fmt.Println("Clash! Both cards are destroyed; no capture.")
	case 0:
		if attackerIsPlayer {
			fmt.Printf("You win and capture %s.\n", ui.app.card(outcome.Captured))
		} else {
			fmt.Printf("CPU wins and captures %s.\n", ui.app.card(outcome.Captured))
		}
	case 1:
		if attackerIsPlayer {
			fmt.Printf("CPU wins and captures %s.\n", ui.app.card(outcome.Captured))
		} else {
			fmt.Printf("You win and capture %s.\n", ui.app.card(outcome.Captured))
		}
	}
}

func (ui *campaignMissionUI) ChooseRoundThreeSeat(scores [2]int) (game.Seat, error) {
	fmt.Printf("After Round 2: You %d, CPU %d. Choose a seat for Round 3.\n", scores[0], scores[1])
	for {
		answer, err := ui.app.ask("Choose X or Y: ")
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

func (ui *campaignMissionUI) ShowMissionResult(result campaigns.MissionResult) {
	if result.Won {
		fmt.Printf("\nMission won! You %d - %d CPU.\n", result.PlayerScore, result.CPUScore)
	} else {
		fmt.Printf("\nMission lost. You %d - %d CPU.\n", result.PlayerScore, result.CPUScore)
	}
}

func (ui *campaignMissionUI) ChooseMissionTuning() (heroes.Tuning, error) {
	level := skills.AggregateBonuses(ui.profile).ConductorLevel
	if level <= 0 {
		return nil, nil
	}
	tuning, err := ui.app.chooseHeroTuning("You", level)
	if err != nil {
		return nil, err
	}
	ui.playerTuning = tuning
	return tuning, nil
}

func campaignMissionEnergyCost(p *profile.Profile, mission campaigns.Mission) int {
	if mission.Index == 1 && skills.AggregateBonuses(p).CampaignFree {
		return 0
	}
	return 1
}

func (a *app) selectMatchHeroes(p *profile.Profile, selected mode) error {
	playerHero, ok := heroes.ByID(p.DefaultHeroID)
	if !ok || !containsHeroID(p.UnlockedHeroes, playerHero.ID) {
		return fmt.Errorf("default hero %q is not unlocked; choose a default in Heroes", p.DefaultHeroID)
	}
	var opponentHero heroes.Hero
	if selected == humanVsHuman {
		var err error
		playerHero, err = a.chooseUnlockedHero(p, "Player 1")
		if err != nil {
			return err
		}
		opponentHero, err = a.chooseUnlockedHero(p, "Player 2")
		if err != nil {
			return err
		}
	} else {
		var err error
		opponentHero, err = heroes.ChooseForOpponent(p.UnlockedHeroes, playerHero.ID, a.rng)
		if err != nil {
			return err
		}
		fmt.Printf("You play %s; the AI chooses %s.\n", playerHero.Name, opponentHero.Name)
	}
	a.playerHeroID, a.opponentHeroID = playerHero.ID, opponentHero.ID
	a.playerTuning, a.opponentTuning = nil, nil
	var err error
	conductorLevel := skills.AggregateBonuses(p).ConductorLevel
	if conductorLevel > 0 {
		a.playerTuning, err = a.chooseHeroTuning("Player 1", conductorLevel)
		if err != nil {
			return err
		}
		if selected == humanVsHuman {
			a.opponentTuning, err = a.chooseHeroTuning("Player 2", conductorLevel)
			if err != nil {
				return err
			}
		}
	}
	if skills.AggregateBonuses(p).HeroPreview {
		combined, err := heroes.CombineWithTuning(playerHero, opponentHero, a.playerTuning, a.opponentTuning)
		if err != nil {
			return err
		}
		fmt.Printf("Modifier preview: Bounty %+d, Shift %+d, Favored %+d\n",
			combined.Bounty, combined.Shift, combined.Favored)
	}
	return nil
}

func (a *app) chooseHeroTuning(playerLabel string, maxModifiers int) (heroes.Tuning, error) {
	tuning := make(heroes.Tuning)
	used := make(map[string]bool)
	for len(used) < maxModifiers {
		choice, err := a.ask(fmt.Sprintf("%s: tune bounty, shift, favored, or blank to finish: ", playerLabel))
		if err != nil {
			return nil, err
		}
		modifier := strings.ToLower(strings.TrimSpace(choice))
		if modifier == "" {
			break
		}
		if modifier != "bounty" && modifier != "shift" && modifier != "favored" {
			fmt.Println("Choose bounty, shift, favored, or blank.")
			continue
		}
		if used[modifier] {
			fmt.Println("Each modifier can only be tuned once.")
			continue
		}
		direction, err := a.ask("Adjust by +1 or -1 (0 to skip): ")
		if err != nil {
			return nil, err
		}
		value, err := strconv.Atoi(strings.TrimSpace(direction))
		if err != nil || value < -1 || value > 1 {
			fmt.Println("Enter +1, 0, or -1.")
			continue
		}
		if value != 0 {
			tuning[modifier] = value
			used[modifier] = true
		}
	}
	if err := heroes.ValidateTuning(tuning, maxModifiers); err != nil {
		return nil, err
	}
	return tuning, nil
}

func (a *app) chooseUnlockedHero(p *profile.Profile, playerLabel string) (heroes.Hero, error) {
	if skills.AggregateBonuses(p).HeroHistory && len(p.LastOpponentHeroIDs) > 0 {
		fmt.Print("Your opponent's last hero picks:")
		for _, heroID := range p.LastOpponentHeroIDs {
			if opponent, ok := heroes.ByID(heroID); ok {
				fmt.Printf(" %s", opponent.Name)
			}
		}
		fmt.Println()
	} else if p.OpponentLastHeroID != "" {
		if opponent, ok := heroes.ByID(p.OpponentLastHeroID); ok {
			fmt.Printf("Your opponent last played: %s\n", opponent.Name)
		}
	}
	unlocked := make([]heroes.Hero, 0, len(p.UnlockedHeroes))
	fmt.Printf("\n%s — choose a hero:\n", playerLabel)
	for _, heroID := range p.UnlockedHeroes {
		hero, ok := heroes.ByID(heroID)
		if !ok {
			continue
		}
		unlocked = append(unlocked, hero)
		fmt.Printf("[%d] %s — B%+d S%+d F%+d\n", len(unlocked), hero.Name, hero.Bounty, hero.Shift, hero.Favored)
	}
	if len(unlocked) == 0 {
		return heroes.Hero{}, errors.New("profile has no unlocked heroes")
	}
	for {
		answer, err := a.ask("Choose a hero number: ")
		if err != nil {
			return heroes.Hero{}, err
		}
		index, err := strconv.Atoi(strings.TrimSpace(answer))
		if err == nil && index >= 1 && index <= len(unlocked) {
			hero := unlocked[index-1]
			confirmed, err := a.confirmHeroPreview(p, hero)
			if err != nil {
				return heroes.Hero{}, err
			}
			if confirmed {
				return hero, nil
			}
			continue
		}
		fmt.Printf("Enter a number from 1 to %d.\n", len(unlocked))
	}
}

func (a *app) heroesMenu(p *profile.Profile) error {
	if p == nil {
		return errors.New("profile is required")
	}
	for {
		a.clearScreen()
		fmt.Printf("=== HEROES | Default: %s | Marks: %d ===\n", heroName(p.DefaultHeroID), p.Marks)
		for index, hero := range heroes.All() {
			status := heroUnlockStatus(p, hero)
			plays, wins := p.HeroPlayCounts[hero.ID], p.HeroWinCounts[hero.ID]
			winRate := 0.0
			if plays > 0 {
				winRate = float64(wins) * 100 / float64(plays)
			}
			fmt.Printf("[%d] %-15s B%+d S%+d F%+d | plays %d, wins %d (%.0f%%) | %s\n",
				index+1, hero.Name, hero.Bounty, hero.Shift, hero.Favored,
				plays, wins, winRate, status)
			fmt.Printf("    %s\n", hero.Flavor)
		}
		fmt.Println("Select an unlocked hero to set it as your default. Acquire locked heroes in Shop or Campaigns.")
		answer, err := a.ask("Choose a hero number, or 0 to return: ")
		if err != nil {
			return err
		}
		answer = strings.TrimSpace(answer)
		if answer == "0" {
			return nil
		}
		index, err := strconv.Atoi(answer)
		all := heroes.All()
		if err != nil || index < 1 || index > len(all) {
			fmt.Println("Enter a listed hero number.")
			continue
		}
		hero := all[index-1]
		if !containsHeroID(p.UnlockedHeroes, hero.ID) {
			fmt.Printf("%s is locked: %s.\n", hero.Name, heroUnlockStatus(p, hero))
			continue
		}
		confirmed, err := a.confirmHeroPreview(p, hero)
		if err != nil {
			return err
		}
		if !confirmed {
			continue
		}
		p.DefaultHeroID = hero.ID
		if err := p.Save(); err != nil {
			return err
		}
	}
}

func (a *app) confirmHeroPreview(p *profile.Profile, playerHero heroes.Hero) (bool, error) {
	if !skills.AggregateBonuses(p).HeroPreview {
		return true, nil
	}
	opponentID := p.OpponentLastHeroID
	if opponentID == "" {
		opponentID = "recruit"
	}
	opponentHero, ok := heroes.ByID(opponentID)
	if !ok {
		return false, fmt.Errorf("unknown preview opponent hero %q", opponentID)
	}
	combined := heroes.Combine(playerHero, opponentHero)
	fmt.Printf("Preview vs %s: Bounty %+d, Shift %+d, Favored %+d.\n",
		opponentHero.Name, combined.Bounty, combined.Shift, combined.Favored)
	answer, err := a.ask("Lock this hero pick? (Y/n): ")
	if err != nil {
		return false, err
	}
	return !strings.EqualFold(strings.TrimSpace(answer), "n") &&
		!strings.EqualFold(strings.TrimSpace(answer), "no"), nil
}

func heroUnlockStatus(p *profile.Profile, hero heroes.Hero) string {
	if containsHeroID(p.UnlockedHeroes, hero.ID) {
		if p.DefaultHeroID == hero.ID {
			return "unlocked, default"
		}
		return "unlocked"
	}
	switch {
	case hero.Unlock == "campaign:basic":
		return "locked: Campaign 1 or 1200 Crowns"
	case hero.Unlock == "hero_lootbox":
		return "locked: Random Hero Box or 1200 Crowns"
	default:
		return "locked: " + hero.Unlock
	}
}

func containsHeroID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func heroName(id string) string {
	if hero, ok := heroes.ByID(id); ok {
		return hero.Name
	}
	return "None"
}

func recordHeroProgress(p *profile.Profile, result game.MatchResult, playerHeroID, opponentHeroID string) {
	if p.HeroPlayCounts == nil {
		p.HeroPlayCounts = make(map[string]int)
	}
	if p.HeroWinCounts == nil {
		p.HeroWinCounts = make(map[string]int)
	}
	p.LastHeroPicked = playerHeroID
	p.OpponentLastHeroID = opponentHeroID
	p.LastOpponentHeroID = opponentHeroID
	p.LastOpponentHeroIDs = append([]string{opponentHeroID}, p.LastOpponentHeroIDs...)
	if len(p.LastOpponentHeroIDs) > 2 {
		p.LastOpponentHeroIDs = p.LastOpponentHeroIDs[:2]
	}
	for _, matchGame := range result.Games {
		heroID := matchGame.PlayerHeroID
		if heroID == "" {
			heroID = playerHeroID
		}
		p.HeroPlayCounts[heroID]++
		if matchGame.Won {
			p.HeroWinCounts[heroID]++
		}
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
	if err := a.selectMatchHeroes(p, selected); err != nil {
		return err
	}
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
	recordHeroProgress(p, achievementResult, a.playerHeroID, a.opponentHeroID)
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
	p1Score := game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total
	p2Score := game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total
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
		state, err := a.newHeroGame()
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
			game.ScorePlayer(finalState.P1, finalState.Bounty.Suit, finalState.Modifiers).Total,
			game.ScorePlayer(finalState.P2, finalState.Bounty.Suit, finalState.Modifiers).Total,
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
		PlayerScore:           game.ScorePlayer(state.P1, state.Bounty.Suit, state.Modifiers).Total,
		OpponentScore:         game.ScorePlayer(state.P2, state.Bounty.Suit, state.Modifiers).Total,
		PlayerJail:            append([]game.Card(nil), state.P1.Jail...),
		OpponentJail:          append([]game.Card(nil), state.P2.Jail...),
		PlayerCapturedByRound: state.P1.CapturedByRound, OpponentCapturedByRound: state.P2.CapturedByRound,
		Modifiers: state.Modifiers, PlayerHeroID: a.playerHeroID, OpponentHeroID: a.opponentHeroID,
		BountySuit: state.Bounty.Suit, Clashes: state.Clashes,
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
	if err := a.showShopCatalog(p); err != nil {
		return err
	}
	return a.chooseShopAction(p)
}

func (a *app) showShopCatalog(p *profile.Profile) error {
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
	if p.HeroBoxesAvailable > 0 {
		fmt.Printf("[free-hero-box] Open a free Hero Box (%d available)\n", p.HeroBoxesAvailable)
	}
	fmt.Printf("[hero-box] Open a Hero Box for %d Marks\n", heroes.HeroBoxCost(p))
	fmt.Printf("[buy-hero] Buy any unowned hero for %d Crowns\n", heroes.DirectHeroCrownsCost)
	fmt.Println("Hero Box pool: Early (Treasurer, Gorilla), Mid (Gambler, Diplomat, Bureaucrat), Late (Bodybuilder, Hater)")
	if p.ChromaPickTokens > 0 {
		fmt.Printf("[Redeem Pick-Any-Chroma token (%d available)]\n", p.ChromaPickTokens)
	}
	return nil
}

func (a *app) chooseShopAction(p *profile.Profile) error {
	answer, err := a.ask("Enter a chroma ID, bundle:<id>, hero-box, free-hero-box, buy-hero, pick, or blank to return: ")
	if err != nil {
		return err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil
	}
	switch strings.ToLower(answer) {
	case "hero-box":
		return a.openHeroBoxMenu(p, false)
	case "free-hero-box":
		return a.openHeroBoxMenu(p, true)
	case "buy-hero":
		return a.buyHeroMenu(p)
	}
	if strings.EqualFold(answer, "pick") {
		return a.redeemChromaMenu(p)
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

func (a *app) redeemChromaMenu(p *profile.Profile) error {
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

func (a *app) openHeroBoxMenu(p *profile.Profile, free bool) error {
	if free && p.HeroBoxesAvailable < 1 {
		return errors.New("no free Hero Boxes are available")
	}
	cost := heroes.HeroBoxCost(p)
	if free {
		fmt.Printf("Open one free Hero Box? (%d free available) (y/N): ", p.HeroBoxesAvailable)
	} else {
		fmt.Printf("Spend %d Marks to open a Hero Box? (y/N): ", cost)
	}
	answer, err := a.ask("")
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(answer), "y") &&
		!strings.EqualFold(strings.TrimSpace(answer), "yes") {
		return nil
	}
	var result heroes.HeroBoxResult
	if free {
		result, err = heroes.OpenHeroBox(p, a.rng)
	} else {
		result, err = heroes.OpenHeroBoxWithMarks(p, a.rng)
	}
	if err != nil {
		return err
	}
	if result.Duplicate {
		fmt.Printf("Duplicate %s-tier hero; +%d Crowns.\n", result.DuplicateTier, result.Crowns)
	} else if hero, ok := heroes.ByID(result.HeroID); ok {
		fmt.Printf("Hero Box unlocked %s!\n", hero.Name)
		for _, event := range evaluateAchievementEvent(p, achievements.EventHeroAcquired, "box") {
			showUnlockToast(event)
		}
	} else {
		return fmt.Errorf("hero box returned unknown hero %q", result.HeroID)
	}
	return p.Save()
}

func (a *app) buyHeroMenu(p *profile.Profile) error {
	all := heroes.All()
	fmt.Println("=== BUY A HERO ===")
	for _, hero := range all {
		if hero.ID == "recruit" {
			continue
		}
		if containsHeroID(p.UnlockedHeroes, hero.ID) {
			fmt.Printf("%-12s %-16s Owned\n", hero.ID, hero.Name)
		} else {
			fmt.Printf("%-12s %-16s %d Crowns\n", hero.ID, hero.Name, heroes.DirectHeroCrownsCost)
		}
	}
	heroID, err := a.ask("Enter an unowned hero ID, or blank to return: ")
	if err != nil {
		return err
	}
	heroID = strings.TrimSpace(heroID)
	if heroID == "" {
		return nil
	}
	hero, ok := heroes.ByID(heroID)
	if !ok || hero.ID == "recruit" {
		return fmt.Errorf("unknown purchasable hero %q", heroID)
	}
	if containsHeroID(p.UnlockedHeroes, heroID) {
		return fmt.Errorf("%s is already owned", hero.Name)
	}
	answer, err := a.ask(fmt.Sprintf("Spend %d Crowns for %s? (y/N): ", heroes.DirectHeroCrownsCost, hero.Name))
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(answer), "y") &&
		!strings.EqualFold(strings.TrimSpace(answer), "yes") {
		return nil
	}
	if err := heroes.BuyHeroWithCrowns(p, heroID); err != nil {
		return err
	}
	for _, event := range evaluateAchievementEvent(p, achievements.EventHeroAcquired, "purchase") {
		showUnlockToast(event)
	}
	fmt.Printf("Purchased %s.\n", hero.Name)
	return p.Save()
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
