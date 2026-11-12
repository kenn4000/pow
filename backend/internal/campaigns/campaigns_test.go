package campaigns

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/profile"
)

func TestCampaignDefinitions(t *testing.T) {
	basic, ok := ByID("basic")
	if !ok || len(basic.Missions) != 10 || basic.Requires != "" ||
		basic.Completion.Marks != 1000 || basic.Completion.UnlocksCampaign != "advanced" {
		t.Fatalf("basic campaign definition = %+v", basic)
	}
	advanced, ok := ByID("advanced")
	if !ok || len(advanced.Missions) != 10 || advanced.Requires != "basic" ||
		advanced.Completion.Marks != 2000 || advanced.Completion.HeroBoxes != 1 ||
		advanced.Completion.SkillPoints != 1 {
		t.Fatalf("advanced campaign definition = %+v", advanced)
	}
	if !basic.Missions[1].CPUAttacksAll || !basic.Missions[3].CPUAttacksAll ||
		basic.Missions[1].CPUMirrorIndex != 1 {
		t.Fatal("scripted clash and all-attacks missions are not configured")
	}

	basic.Missions[0].PlayerHand[0].Face = 1
	basic.Missions[0].PlayerHeroRestriction[0] = "changed"
	basic.Completion.Heroes[0] = "changed"
	again, _ := ByID("basic")
	if again.Missions[0].PlayerHand[0].Face != 5 ||
		again.Missions[0].PlayerHeroRestriction[0] != "recruit" ||
		again.Completion.Heroes[0] != "warden" {
		t.Fatal("campaign definitions expose mutable internal slices")
	}
}

func TestCampaignMissionRewardTiersAcrossCatalog(t *testing.T) {
	for _, campaign := range All() {
		for _, mission := range campaign.Missions {
			if !hasExpectedMissionReward(mission) {
				t.Errorf("%s mission %d rewards = %d Marks, %d Crowns",
					campaign.ID, mission.Index, mission.RewardMarks, mission.RewardCrowns)
			}
		}
	}
}

func hasExpectedMissionReward(mission Mission) bool {
	switch {
	case mission.Index <= 5:
		return mission.RewardMarks == 350 && mission.RewardCrowns == 0
	case mission.Index <= 10:
		return mission.RewardMarks == 300 && mission.RewardCrowns == 1
	default:
		return mission.RewardMarks == 350 && mission.RewardCrowns == 2
	}
}

func TestSpecialCampaignMissionMappingsAndNoHeroStubs(t *testing.T) {
	ladder, ok := ByID("ladder")
	if !ok || ladder.Name != "Ladder Campaign" || ladder.Variant != "ladder" ||
		ladder.Status != "" || len(ladder.Missions) != 15 ||
		ladder.Missions[0].Disabled || ladder.Missions[0].Ladder == nil ||
		!CanPlayMission(&profile.Profile{}, "ladder", 1) {
		t.Fatalf("Ladder campaign definition = %+v", ladder)
	}
	for index, mission := range ladder.Missions {
		if mission.Index != index+1 || mission.Variant != "ladder" ||
			mission.Ladder == nil || !hasExpectedMissionReward(mission) ||
			mission.Disabled {
			t.Errorf("Ladder mission %d = %+v", index+1, mission)
		}
	}
	rummy, ok := ByID("rummy")
	if !ok || rummy.Variant != "rummy" || len(rummy.Missions) != 15 {
		t.Fatalf("Rummy campaign definition = %+v", rummy)
	}
	for index, mission := range rummy.Missions {
		if mission.Index != index+1 || mission.Variant != "rummy" ||
			mission.Rummy == nil || !hasExpectedMissionReward(mission) ||
			mission.Disabled {
			t.Errorf("Rummy mission %d = %+v", index+1, mission)
		}
	}
	if rummy.Missions[12].AIHero != "treasurer" ||
		rummy.Missions[6].Rummy.Parameters.HandSize != 5 ||
		rummy.Missions[14].Rummy.MatchHands != 2 {
		t.Fatal("Rummy hero substitution, shifted hand setup, or final match is incorrect")
	}

	abandon, ok := ByID("abandon")
	if !ok || abandon.Variant != "abandon" || len(abandon.Missions) != 15 {
		t.Fatalf("abandon definition = %+v", abandon)
	}
	names := []string{
		"First Blood", "Mirror", "The Tie", "Count and Peak", "The Bounty",
		"Don't Abandon", "Flip and Find", "Blind Gamble", "The Mirror Match",
		"Snake Eyes", "Two Rounds", "All-In", "Spread", "Tempo", "Abandon",
	}
	for index, mission := range abandon.Missions {
		if mission.Index != index+1 || mission.Variant != "abandon" ||
			mission.Name != names[index] || mission.Disabled || mission.Abandon == nil ||
			!hasExpectedMissionReward(mission) {
			t.Errorf("Abandon mission %d = %+v", index+1, mission)
		}
	}

	puzzler, ok := ByID("puzzler")
	if !ok || puzzler.Variant != "puzzler" || len(puzzler.Missions) != 20 {
		t.Fatalf("puzzler definition = %+v", puzzler)
	}
	puzzleIDs := []string{
		"gp6_balanced", "gp6_hunters_edge", "gp6_bounty_trap", "gp6_narrow_gate",
		"gp6_bodybuilders_wall", "gp8_balanced", "gp8_bounty_climb", "gp8_forgiving",
		"gp8_gamblers_puzzle", "gp8_tight", "gp10_balanced", "gp10_gorilla_threat",
		"gp10_turtle_lead", "gp10_low_score", "gp10_everyone_solves", "gp12_balanced",
		"gp12_dual_threat", "gp12_shift_dominant", "gp12_forgiving", "gp12_inverted",
	}
	for index, mission := range puzzler.Missions {
		if mission.Index != index+1 || mission.Variant != "puzzler" ||
			mission.VariantID != puzzleIDs[index] || !hasExpectedMissionReward(mission) {
			t.Errorf("puzzler mission %d = %+v", index+1, mission)
		}
	}

	for _, id := range []string{
		"warden", "hunter", "treasurer", "ascendant", "anchor",
		"gambler", "diplomat", "ascetic", "herald", "hater",
	} {
		if _, found := ByID(id); found {
			t.Errorf("hero-specific campaign stub %q is still registered", id)
		}
	}
	if _, ok := ByID("recruit"); ok {
		t.Fatal("Recruit must not have a hero campaign")
	}
}

func TestTugWarCampaignMissionDefinitions(t *testing.T) {
	campaign, ok := ByID("tug-war")
	if !ok || campaign.Variant != "tug-war" || campaign.Requires != "" || len(campaign.Missions) != 15 {
		t.Fatalf("Tug War campaign = %+v", campaign)
	}
	for index, mission := range campaign.Missions {
		if mission.Index != index+1 || mission.Variant != "tug-war" ||
			mission.TugWar == nil || !hasExpectedMissionReward(mission) {
			t.Errorf("Tug War mission %d = %+v", index+1, mission)
		}
	}
	first := campaign.Missions[0].TugWar
	if first.PlayerHand[0] != (game.Card{Suit: game.Shield, Face: 1}) ||
		first.CPUHand[0] != (game.Card{Suit: game.Fang, Face: 2}) ||
		first.Piles[0].Cards[0] != (game.Card{Suit: game.Fang, Face: 1}) ||
		first.Piles[1].Cards[0] != (game.Card{Suit: game.Shield, Face: 2}) ||
		first.BountySuit != game.Blade || first.Pushes != [2]int{2, 0} {
		t.Fatalf("First Steps setup = %+v", first)
	}
	if campaign.Missions[1].TugWar.Piles[0].Cards[0] != (game.Card{Suit: game.Shield, Face: 8}) ||
		campaign.Missions[4].TugWar.Piles[0].Cards[0] != (game.Card{Suit: game.Shield, Face: 8}) ||
		campaign.Missions[14].TugWar.Piles[0].Cards[0] != (game.Card{Suit: game.Shield, Face: 7}) {
		t.Fatal("Tug War campaign substitutions changed scripted pile tops")
	}
	fourth := campaign.Missions[3].TugWar
	if fourth.Piles[0].Zone != 1 || len(fourth.Piles[0].Cards) != 4 ||
		fourth.Piles[0].Cards[0] != (game.Card{Suit: game.Shield, Face: 5}) ||
		fourth.TargetPile != 0 || fourth.MaxRounds != 4 || !fourth.EarlyWin {
		t.Fatalf("High Ground setup = %+v", fourth)
	}
	sixth := campaign.Missions[5].TugWar
	if !sixth.Piles[0].Hidden || !sixth.HideBountyUntilFlip ||
		sixth.Deck[0] != (game.Card{Suit: game.Blade, Face: 2}) {
		t.Fatalf("Fog of War setup = %+v", sixth)
	}
	mirror, gauntlet := campaign.Missions[11].TugWar, campaign.Missions[13].TugWar
	if !mirror.MirrorHero || mirror.SeriesGames != 3 || mirror.SeriesWinsNeeded != 2 ||
		gauntlet.SeriesCPUHeroes[2] != "ascendant" || gauntlet.SeriesPolicies[2] != "spreader" {
		t.Fatalf("series configurations = mirror %+v, gauntlet %+v", mirror, gauntlet)
	}
	originalPileCard := first.Piles[0].Cards[0]
	first.PlayerHand[0].Face = 9
	first.Piles[0].Cards[0].Face = 9
	again, _ := ByID("tug-war")
	if again.Missions[0].TugWar.PlayerHand[0].Face != 1 ||
		again.Missions[0].TugWar.Piles[0].Cards[0] != originalPileCard {
		t.Fatal("Tug War campaign exposes mutable setup slices")
	}
}

func TestTugWarCampaignPresetsUseDistinctPhysicalCards(t *testing.T) {
	campaign, _ := ByID("tug-war")
	for _, mission := range campaign.Missions {
		if mission.TugWar == nil || len(mission.TugWar.PlayerHand) == 0 {
			continue
		}
		t.Run(mission.Name, func(t *testing.T) {
			seen := map[game.Card]string{}
			add := func(card game.Card, location string) {
				t.Helper()
				if previous, exists := seen[card]; exists {
					t.Errorf("card %s appears in both %s and %s", card, previous, location)
				} else {
					seen[card] = location
				}
			}
			for _, card := range mission.TugWar.PlayerHand {
				add(card, "player hand")
			}
			for _, card := range mission.TugWar.CPUHand {
				add(card, "CPU hand")
			}
			for pileIndex, pile := range mission.TugWar.Piles {
				for _, card := range pile.Cards {
					add(card, fmt.Sprintf("pile %d", pileIndex+1))
				}
			}
			for _, card := range mission.TugWar.Deck {
				add(card, "deck")
			}
		})
	}
}

func TestPrepareMissionWithTuningAppliesConductorModifier(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	mission := Mission{AIHero: "recruit", Type: ThreeRound}
	state, _, err := PrepareMissionWithTuning(p, mission, "recruit", heroes.Tuning{"shift": 1},
		rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if state.Modifiers.Shift != 1 {
		t.Fatalf("mission Shift modifier = %d, want 1", state.Modifiers.Shift)
	}
}

func TestPokerCampaignMissionRules(t *testing.T) {
	poker, ok := ByID("poker")
	if !ok || poker.Variant != "poker" || len(poker.Missions) != 15 {
		t.Fatalf("poker campaign = %+v", poker)
	}
	names := []string{
		"The First Hand", "Finding the Nine", "Your Suit", "The Even Table", "The Bonus Table",
		"Chase the Scissors", "Chase the Paper", "Silent Nines", "The Wall",
		"Free Play", "High Bar", "Consistency", "Doubled Bounty", "Nine Storm", "The Crown",
	}
	for index, mission := range poker.Missions {
		if mission.Index != index+1 || mission.Name != names[index] ||
			mission.Variant != "poker" || mission.VariantID != fmt.Sprintf("poker-%02d", index+1) ||
			!hasExpectedMissionReward(mission) || mission.PokerRules == nil {
			t.Errorf("poker mission %d = %+v", index+1, mission)
		}
	}
	for index, par := range []int{160, 175, 185, 185, 195, 200, 200, 175, 195, 205, 210, 175, 210, 215, 235} {
		if poker.Missions[index].PokerRules.Par != par {
			t.Errorf("poker mission %d par = %d, want %d", index+1, poker.Missions[index].PokerRules.Par, par)
		}
	}
	m1, m2, m3 := poker.Missions[0].PokerRules, poker.Missions[1].PokerRules, poker.Missions[2].PokerRules
	if m1.ForcedBountySuit == nil || *m1.ForcedBountySuit != game.Shield ||
		len(m1.StartingHand) != 6 || m1.StartingHand[0] != card(game.Shield, 9) ||
		m2.ForcedBountySuit == nil || *m2.ForcedBountySuit != game.Fang ||
		m3.ForcedBountySuit == nil || *m3.ForcedBountySuit != game.Blade {
		t.Fatal("scripted opening hands or forced bounty suits are incorrect")
	}
	if poker.Missions[5].PokerRules.Ghost == nil ||
		poker.Missions[5].PokerRules.Ghost.Strategy != game.PokerGhostBounty ||
		poker.Missions[6].PokerRules.Ghost == nil ||
		poker.Missions[6].PokerRules.Ghost.Strategy != game.PokerGhostNine ||
		poker.Missions[7].PokerRules.Constraint != game.PokerNoNines ||
		poker.Missions[8].PokerRules.Constraint != game.PokerNoStraightsOrFlushes ||
		poker.Missions[11].PokerRules.StreakCount != 3 ||
		poker.Missions[11].PokerRules.StreakScore != 175 ||
		poker.Missions[12].PokerRules.NineBase != 8 ||
		poker.Missions[12].PokerRules.BountyBase != 3 ||
		poker.Missions[13].PokerRules.NineBase != 16 ||
		len(poker.Missions[14].PokerRules.RecommendedHeroes) != 0 ||
		poker.Missions[14].PokerRules.Lesson != "" {
		t.Fatal("Poker mission rules, ghost opponents, or hidden final hints are incorrect")
	}
	poker.Missions[0].PokerRules.NineBase = -1
	poker.Missions[0].PokerRules.StartingHand[0].Face = 1
	poker.Missions[5].PokerRules.Ghost.StartingHand[0].Face = 1
	again, _ := ByID("poker")
	if again.Missions[0].PokerRules.NineBase != 12 ||
		again.Missions[0].PokerRules.StartingHand[0] != card(game.Shield, 9) ||
		again.Missions[5].PokerRules.Ghost.StartingHand[0] != card(game.Blade, 2) {
		t.Fatal("poker rules expose mutable internal campaign data")
	}
}

func TestNewDraftMissionRules(t *testing.T) {
	advanced, _ := ByID("advanced")
	tests := []struct {
		index       int
		mode        int
		playerSeat  game.Seat
		firstPicker game.Seat
		keepSeat    bool
	}{
		{index: 6, mode: 0, playerSeat: game.SeatY},
		{index: 7, mode: 2, playerSeat: game.SeatX, firstPicker: game.SeatY},
		{index: 8, mode: 4, playerSeat: game.SeatX, firstPicker: game.SeatY},
		{index: 9, mode: 3, playerSeat: game.SeatX, firstPicker: game.SeatX, keepSeat: true},
		{index: 10, mode: 3, playerSeat: game.SeatX, firstPicker: game.SeatY, keepSeat: true},
	}
	for _, test := range tests {
		mission, ok := findMission(advanced, test.index)
		if !ok {
			t.Fatalf("mission %d missing", test.index)
		}
		state, err := newMissionState(mission, game.GameModifiers{Reckoning: [3]int{1, 1, 1}},
			rand.New(rand.NewSource(int64(test.index))))
		if err != nil {
			t.Fatalf("mission %d state: %v", test.index, err)
		}
		if state.P1.Seat != test.playerSeat || state.P2.Seat == test.playerSeat {
			t.Errorf("mission %d seats = %s/%s, player should be %s", test.index, state.P1.Seat, state.P2.Seat, test.playerSeat)
		}
		if mission.PlayerAttacksFirstAll != test.keepSeat {
			t.Errorf("mission %d keep-seat rule = %v, want %v", test.index, mission.PlayerAttacksFirstAll, test.keepSeat)
		}
		if test.mode == 0 {
			if state.Draft != nil || state.DraftMode != 0 || len(state.P1.Hand) != 5 || len(state.P2.Hand) != 5 {
				t.Errorf("No Draft Duel setup: draft=%v mode=%d hands=%d/%d", state.Draft, state.DraftMode, len(state.P1.Hand), len(state.P2.Hand))
			}
			continue
		}
		if state.Draft == nil || state.Draft.Mode != test.mode ||
			len(state.Draft.Pool) != 2+2*test.mode ||
			len(state.Draft.DealtX) != 5-test.mode ||
			len(state.Draft.DealtY) != 5-test.mode ||
			state.Draft.PickOrder[0] != test.firstPicker {
			t.Errorf("mission %d draft setup = %+v", test.index, state.Draft)
		}
		if test.index == 7 {
			for _, card := range state.Draft.DealtY {
				if card.Face != 9 {
					t.Errorf("Unlucky CPU hole card = %s, want a nine", card)
				}
			}
			if state.Bounty.Face == 9 {
				t.Error("Unlucky bounty used one of the CPU's three nines")
			}
		}
	}
}

func TestCampaignUnlocksProgressionAndRewards(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if IsUnlocked(p, "advanced") {
		t.Fatal("Advanced Training should start locked")
	}
	if err := StartCampaign(p, "advanced"); err == nil {
		t.Fatal("starting a locked campaign should fail")
	}
	if !CanPlayMission(p, "basic", 1) || CanPlayMission(p, "basic", 2) {
		t.Fatal("only the first Basic Training mission should initially be playable")
	}
	for mission := 1; mission <= 10; mission++ {
		outcome, err := CompleteMission(p, "basic", mission, 0)
		if err != nil {
			t.Fatal(err)
		}
		wantMarks, wantCrowns := int64(350), int64(0)
		if mission > 5 {
			wantMarks, wantCrowns = 300, 1
		}
		if outcome.MissionMarks != wantMarks || outcome.MissionCrowns != wantCrowns {
			t.Fatalf("basic mission %d reward = %d Marks, %d Crowns; want %d, %d",
				mission, outcome.MissionMarks, outcome.MissionCrowns, wantMarks, wantCrowns)
		}
		if mission < 10 && outcome.CampaignCompleted {
			t.Fatalf("basic campaign completed after mission %d", mission)
		}
		if mission == 10 && (!outcome.CampaignCompleted || outcome.CompletionMarks != BasicCompleteMarks) {
			t.Fatalf("basic completion outcome = %+v", outcome)
		}
	}
	if p.Marks != 4250 || p.Crowns != 5 ||
		!p.Campaigns["basic"].Completed || !IsUnlocked(p, "advanced") {
		t.Fatalf("basic completion: marks %d, progress %+v", p.Marks, p.Campaigns["basic"])
	}
	if !CanPlayMission(p, "basic", 1) || CanPlayMission(p, "advanced", 2) {
		t.Fatal("completed missions should be replayable while locked advanced missions remain unavailable")
	}
	for _, heroID := range []string{"warden", "hunter", "anchor"} {
		if !hasHero(p.UnlockedHeroes, heroID) {
			t.Errorf("basic completion did not unlock %s", heroID)
		}
	}
	if !hasBadge(p.Badges, "badge_basic_training") {
		t.Fatal("basic completion did not grant its badge")
	}
}

func TestPreviouslyCompletedAdvancedCampaignCanAccessAddedMissionsAndSkillPoint(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for mission := 1; mission <= 10; mission++ {
		if _, err := CompleteMission(p, "basic", mission, 0); err != nil {
			t.Fatal(err)
		}
	}
	completed := make([]int, 5)
	for index := range completed {
		completed[index] = index + 1
	}
	p.Marks += AdvancedCompleteMarks + 5*AdvancedMissionMarks
	p.HeroBoxesAvailable = 1
	p.Badges = append(p.Badges, "badge_advanced_training")
	p.Campaigns["advanced"] = &profile.CampaignProgress{
		CampaignID: "advanced", MissionsCompleted: 5, TotalMissions: 5,
		CompletedMissionIDs: completed, Completed: true,
	}
	if !CanPlayMission(p, "advanced", 6) {
		t.Fatal("previously completed campaign cannot access its first newly added mission")
	}
	for mission := 6; mission <= 10; mission++ {
		outcome, err := CompleteMission(p, "advanced", mission, 0)
		if err != nil {
			t.Fatal(err)
		}
		if mission == 10 && (!outcome.CampaignCompleted || outcome.CompletionMarks != 0 ||
			outcome.SkillPointsAwarded != 1) {
			t.Fatalf("new final reward for previously completed campaign = %+v", outcome)
		}
	}
	if !p.Campaigns["advanced"].Completed || p.SkillPoints != 1 ||
		p.Marks != 9500 || p.HeroBoxesAvailable != 1 ||
		!hasBadge(p.Badges, "badge_advanced_training") {
		t.Fatalf("appended mission progression = progress %+v, marks %d, skill points %d, boxes %d",
			p.Campaigns["advanced"], p.Marks, p.SkillPoints, p.HeroBoxesAvailable)
	}
}

func TestAdvancedCampaignRewardsAndReplayDoNotRewardTwice(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for mission := 1; mission <= 10; mission++ {
		if _, err := CompleteMission(p, "basic", mission, 0); err != nil {
			t.Fatal(err)
		}
	}
	for mission := 1; mission <= 10; mission++ {
		outcome, err := CompleteMission(p, "advanced", mission, 0)
		if err != nil {
			t.Fatal(err)
		}
		if mission == 10 && (!outcome.CampaignCompleted || outcome.HeroBoxesAwarded != 1 ||
			outcome.SkillPointsAwarded != 1) {
			t.Fatalf("advanced completion outcome = %+v", outcome)
		}
	}
	if p.Marks != 9500 || p.Crowns != 10 ||
		p.HeroBoxesAvailable != 1 || p.SkillPoints != 1 ||
		!hasBadge(p.Badges, "badge_advanced_training") {
		t.Fatalf("advanced rewards: marks %d, boxes %d, skill points %d, badges %v",
			p.Marks, p.HeroBoxesAvailable, p.SkillPoints, p.Badges)
	}
	previousMarks, previousBoxes := p.Marks, p.HeroBoxesAvailable
	outcome, err := CompleteMission(p, "advanced", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.AlreadyCompleted || outcome.MissionMarks != 0 ||
		p.Marks != previousMarks || p.Crowns != 10 || p.HeroBoxesAvailable != previousBoxes {
		t.Fatal("replaying a completed mission awarded rewards")
	}
}

func TestCampaignMissionUnlocksSequentiallyAndBonus(t *testing.T) {
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteMission(p, "basic", 2, 0); err == nil {
		t.Fatal("mission 2 should remain locked until mission 1 is completed")
	}
	outcome, err := CompleteMission(p, "basic", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.MissionMarks != 525 || p.Marks != 525 || !CanPlayMission(p, "basic", 2) {
		t.Fatalf("campaign bonus/progression = %+v, marks %d", outcome, p.Marks)
	}
	if !CanPlayMission(p, "basic", 1) {
		t.Fatal("completed missions should remain replayable")
	}
}

func TestScriptedBasicMissions(t *testing.T) {
	tests := []struct {
		name      string
		mission   int
		plays     []game.Card
		player    int
		cpu       int
		clashes   int
		discarded game.Card
	}{
		{
			name:    "First Blood captures the three scripted cards",
			mission: 1,
			plays: []game.Card{
				{Suit: game.Shield, Face: 7}, {Suit: game.Shield, Face: 6}, {Suit: game.Shield, Face: 5},
			},
			player: 12,
		},
		{
			name:    "The Clash destroys the mirrored cards",
			mission: 2,
			plays: []game.Card{
				{Suit: game.Shield, Face: 3}, {Suit: game.Shield, Face: 5}, {Suit: game.Shield, Face: 7},
			},
			player: 6, cpu: 3, clashes: 1, discarded: game.Card{Suit: game.Shield, Face: 5},
		},
		{
			name:    "The Cycle uses the suit ranking",
			mission: 3,
			plays: []game.Card{
				{Suit: game.Shield, Face: 5}, {Suit: game.Fang, Face: 5}, {Suit: game.Blade, Face: 5},
			},
			player: 27,
		},
		{
			name:    "The Bounty scores Rock plus two",
			mission: 4,
			plays: []game.Card{
				{Suit: game.Fang, Face: 4}, {Suit: game.Shield, Face: 4}, {Suit: game.Blade, Face: 7},
			},
			player: 9, cpu: 7, clashes: 1,
		},
	}

	campaign, _ := ByID("basic")
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mission, _ := findMission(campaign, test.mission)
			ui := &testMissionUI{plays: test.plays}
			result, err := RunMission(p, mission, rand.New(rand.NewSource(40)), ui)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Won || result.PlayerScore != test.player || result.CPUScore != test.cpu ||
				result.State.Clashes != test.clashes {
				t.Fatalf("mission result = won %t, score %d-%d, clashes %d",
					result.Won, result.PlayerScore, result.CPUScore, result.State.Clashes)
			}
			if test.discarded.Valid() && !containsCard(result.State.Discard, test.discarded) {
				t.Fatalf("clashed player card %s was not discarded", test.discarded)
			}
			if test.mission == 2 {
				battle := ui.battles[1]
				if battle.outcome.Winner != -1 || battle.attacker != battle.defender ||
					battle.attacker.Face != 5 || battle.defender.Face != 5 {
					t.Fatalf("mirror exchange = %+v", battle)
				}
			}
			if !ui.lastResult.Won {
				t.Fatal("UI did not receive the completed mission result")
			}
		})
	}
}

func TestThreeRoundMissionUsesNormalRoundFlow(t *testing.T) {
	campaign, _ := ByID("basic")
	mission, _ := findMission(campaign, 5)
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ui := &testMissionUI{}
	result, err := RunMission(p, mission, rand.New(rand.NewSource(121)), ui)
	if err != nil {
		t.Fatal(err)
	}
	if err := result.State.Validate(); err != nil {
		t.Fatal(err)
	}
	if result.State.Round != 3 || result.State.Modifiers.Reckoning != [3]int{1, 1, 1} ||
		len(result.MatchResult.Games) != 1 {
		t.Fatalf("three-round mission state = round %d, modifiers %+v", result.State.Round, result.State.Modifiers)
	}
}

func TestBlindAggressionStartsEachRoundWithPlayerAttack(t *testing.T) {
	campaign, _ := ByID("advanced")
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for mission := 1; mission <= 10; mission++ {
		if _, err := CompleteMission(p, "basic", mission, 0); err != nil {
			t.Fatal(err)
		}
	}
	for _, missionIndex := range []int{9, 10} {
		mission, _ := findMission(campaign, missionIndex)
		ui := &testMissionUI{}
		_, err := RunMission(p, mission, rand.New(rand.NewSource(int64(missionIndex))), ui)
		if err != nil {
			t.Fatalf("%s: %v", mission.Name, err)
		}
		if len(ui.battles) != 9 {
			t.Fatalf("%s recorded %d exchanges, want 9", mission.Name, len(ui.battles))
		}
		for round := 0; round < 3; round++ {
			for exchange, wantPlayerAttack := range []bool{true, false, true} {
				if got := ui.battles[round*3+exchange].attackerIsPlayer; got != wantPlayerAttack {
					t.Errorf("%s round %d exchange %d player attacking = %v, want %v",
						mission.Name, round+1, exchange+1, got, wantPlayerAttack)
				}
			}
		}
	}
}

func TestBasicMissionModifierOverrides(t *testing.T) {
	campaign, _ := ByID("basic")
	expected := map[int]game.GameModifiers{
		5:  {Bounty: 0, Shift: 0, Favored: 0, Reckoning: [3]int{1, 1, 1}},
		6:  {Bounty: 0, Shift: 0, Favored: 0, Reckoning: [3]int{1, 2, 3}},
		7:  {Bounty: -2, Shift: 0, Favored: 0, Reckoning: [3]int{1, 1, 1}},
		8:  {Bounty: 0, Shift: 0, Favored: 2, Reckoning: [3]int{1, 1, 1}},
		9:  {Bounty: 0, Shift: 2, Favored: 0, Reckoning: [3]int{1, 1, 1}},
		10: {Bounty: 2, Shift: 1, Favored: -1, Reckoning: [3]int{1, 1, 2}},
	}
	recruit, _ := heroes.ByID("recruit")
	combined := heroes.Combine(recruit, recruit)
	for index, want := range expected {
		mission, _ := findMission(campaign, index)
		got, err := missionModifiers(mission, combined, rand.New(rand.NewSource(int64(index))))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("mission %d modifiers = %+v, want %+v", index, got, want)
		}
		if index == 7 || index == 10 {
			state, err := newMissionState(mission, got, rand.New(rand.NewSource(int64(index+100))))
			if err != nil {
				t.Fatal(err)
			}
			if state.Bounty.Suit != game.Shield {
				t.Errorf("mission %d bounty suit = %s, want Rock", index, state.Bounty.Suit)
			}
		}
	}
}

func TestThreeRoundFixedPlayerHand(t *testing.T) {
	mission := Mission{
		Name: "fixed hand", Type: ThreeRound, CPUMirrorIndex: -1,
		PlayerHand: []game.Card{
			{Suit: game.Blade, Face: 2}, {Suit: game.Fang, Face: 4}, {Suit: game.Shield, Face: 6},
		},
		BountySuitOverride: stringValue("R"),
	}
	modifiers := game.GameModifiers{Reckoning: [3]int{1, 1, 1}}
	state, err := newMissionState(mission, modifiers, rand.New(rand.NewSource(88)))
	if err != nil {
		t.Fatal(err)
	}
	if len(state.P1.Hand) != len(mission.PlayerHand) {
		t.Fatalf("fixed player hand has %d cards, want %d", len(state.P1.Hand), len(mission.PlayerHand))
	}
	for index, card := range mission.PlayerHand {
		if state.P1.Hand[index] != card {
			t.Errorf("player hand[%d] = %s, want %s", index, state.P1.Hand[index], card)
		}
	}
	if state.Bounty.Suit != game.Shield {
		t.Fatalf("fixed bounty suit = %s, want Rock", state.Bounty.Suit)
	}
}

type testMissionUI struct {
	plays      []game.Card
	lastResult MissionResult
	battles    []testBattle
}

type testBattle struct {
	attacker         game.Card
	defender         game.Card
	outcome          game.Outcome
	attackerIsPlayer bool
}

func (ui *testMissionUI) ShowBriefing(Mission)            {}
func (ui *testMissionUI) ShowStartingHand([]game.Card)    {}
func (ui *testMissionUI) ShowMissionSetup(game.GameState) {}
func (ui *testMissionUI) ChooseDraftPick([]game.Card, int, int) (int, error) {
	return 0, nil
}
func (ui *testMissionUI) ShowBattleResult(_ int, attacker, defender game.Card, outcome game.Outcome, attackerIsPlayer bool) {
	ui.battles = append(ui.battles, testBattle{
		attacker: attacker, defender: defender, outcome: outcome, attackerIsPlayer: attackerIsPlayer,
	})
}
func (ui *testMissionUI) ChooseRoundThreeSeat([2]int) (game.Seat, error) { return game.SeatX, nil }
func (ui *testMissionUI) ShowMissionResult(result MissionResult)         { ui.lastResult = result }
func (ui *testMissionUI) ChooseMissionTuning() (heroes.Tuning, error)    { return nil, nil }
func (ui *testMissionUI) ChooseHeroForMission(allowed []string) (string, error) {
	return allowed[0], nil
}

func (ui *testMissionUI) ChooseCommit(hand []game.Card, count int) ([]int, error) {
	indices := make([]int, count)
	for index := range indices {
		indices[index] = index
	}
	return indices, nil
}

func (ui *testMissionUI) ChoosePoolCard(exchange int, _ bool, _ *game.Card, pool []game.Card) (int, error) {
	if exchange <= len(ui.plays) {
		target := ui.plays[exchange-1]
		for index, card := range pool {
			if card == target {
				return index, nil
			}
		}
		return 0, &unavailableTestCard{card: target}
	}
	return 0, nil
}

type unavailableTestCard struct {
	card game.Card
}

func (err *unavailableTestCard) Error() string {
	return "test card is unavailable: " + err.card.String()
}

func hasHero(heroIDs []string, target string) bool {
	for _, heroID := range heroIDs {
		if heroID == target {
			return true
		}
	}
	return false
}

func hasBadge(badges []string, target string) bool {
	for _, badge := range badges {
		if badge == target {
			return true
		}
	}
	return false
}
