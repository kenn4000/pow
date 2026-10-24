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
	if !ok || len(advanced.Missions) != 5 || advanced.Requires != "basic" ||
		advanced.Completion.Marks != 2000 || advanced.Completion.HeroBoxes != 1 {
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
	for mission := 1; mission <= 5; mission++ {
		outcome, err := CompleteMission(p, "advanced", mission, 0)
		if err != nil {
			t.Fatal(err)
		}
		if mission == 5 && (!outcome.CampaignCompleted || outcome.HeroBoxesAwarded != 1) {
			t.Fatalf("advanced completion outcome = %+v", outcome)
		}
	}
	if p.Marks != 6000 || p.HeroBoxesAvailable != 1 ||
		!hasBadge(p.Badges, "badge_advanced_training") {
		t.Fatalf("advanced rewards: marks %d, boxes %d, badges %v", p.Marks, p.HeroBoxesAvailable, p.Badges)
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
	attacker game.Card
	defender game.Card
	outcome  game.Outcome
}

func (ui *testMissionUI) ShowBriefing(Mission)            {}
func (ui *testMissionUI) ShowStartingHand([]game.Card)    {}
func (ui *testMissionUI) ShowMissionSetup(game.GameState) {}
func (ui *testMissionUI) ShowBattleResult(_ int, attacker, defender game.Card, outcome game.Outcome, _ bool) {
	ui.battles = append(ui.battles, testBattle{attacker: attacker, defender: defender, outcome: outcome})
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
