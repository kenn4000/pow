package campaigns

import (
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
		if outcome.MissionMarks != BasicMissionMarks {
			t.Fatalf("basic mission %d marks = %d", mission, outcome.MissionMarks)
		}
		if mission < 10 && outcome.CampaignCompleted {
			t.Fatalf("basic campaign completed after mission %d", mission)
		}
		if mission == 10 && (!outcome.CampaignCompleted || outcome.CompletionMarks != BasicCompleteMarks) {
			t.Fatalf("basic completion outcome = %+v", outcome)
		}
	}
	if p.Marks != 2000 || !p.Campaigns["basic"].Completed || !IsUnlocked(p, "advanced") {
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
		p.Marks != 8000 || p.HeroBoxesAvailable != 1 ||
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
	if p.Marks != 8000 || p.HeroBoxesAvailable != 1 || p.SkillPoints != 1 ||
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
		p.Marks != previousMarks || p.HeroBoxesAvailable != previousBoxes {
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
	if outcome.MissionMarks != 150 || p.Marks != 150 || !CanPlayMission(p, "basic", 2) {
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
