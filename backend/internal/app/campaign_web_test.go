package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/ladder"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/tugwar"
)

func campaignTestServer(t *testing.T) *webServer {
	t.Helper()
	p, err := profile.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.SetSaveHandler(func(*profile.Profile) error { return nil })
	if err := (&app{}).initializeProgressionProfile(p); err != nil {
		t.Fatal(err)
	}
	return &webServer{playerProfile: p, rng: rand.New(rand.NewSource(42)), defaultMode: "easy"}
}

func finishTestMission(t *testing.T, web *webServer) {
	t.Helper()
	for step := 0; step < 100 && web.session.phase != "gameover"; step++ {
		request := actionRequest{Action: web.session.phase, Index: 0, Cards: []int{0, 1, 2}}
		switch request.Action {
		case "draft":
			if web.session.actor == 0 {
				request.Action = "draft-pick"
			} else {
				request.Action = "draft-ai-pick"
			}
		case "result":
			request.Action = "continue"
		}
		if err := web.applyAction(request); err != nil {
			t.Fatalf("%s: %v", request.Action, err)
		}
		if err := web.advanceAI(); err != nil {
			t.Fatal(err)
		}
		snapshot := snapshotSession(web.session)
		web.session = snapshot.restore()
	}
	if web.session.phase != "gameover" {
		t.Fatal("mission did not finish")
	}
}

func expectedCampaignMissionReward(index int) (int64, int64) {
	switch {
	case index <= 5:
		return 350, 0
	case index <= 10:
		return 300, 1
	default:
		return 350, 2
	}
}

func TestWebCampaignCompletionReplayAndSnapshots(t *testing.T) {
	web := campaignTestServer(t)
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 2}); err == nil {
		t.Fatal("locked mission was allowed")
	}
	energy := web.playerProfile.Energy
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
	}
	if web.session.variant != "main" || web.session.campaign.Variant != "main" {
		t.Fatalf("standard campaign variant = %q / %q", web.session.variant, web.session.campaign.Variant)
	}
	if web.playerProfile.Energy != energy-1 {
		t.Fatal("mission energy was not charged")
	}
	finishTestMission(t, web)
	if web.session.winner != 0 || !missionIsComplete(web.playerProfile.Campaigns["basic"], 1) {
		t.Fatal("first mission did not award completion")
	}
	if len(web.session.battles) != 3 || len(web.session.achievementUpdates) == 0 {
		t.Fatal("mission omitted its persisted battle log or achievement delta")
	}
	if web.session.payout.TotalMarks != 350 {
		t.Fatalf("mission payout = %d", web.session.payout.TotalMarks)
	}
	marks, games, remaining := web.playerProfile.Marks, web.playerProfile.TotalGames, web.playerProfile.Energy
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
	}
	if !web.session.campaign.Replay {
		t.Fatal("completed mission was not marked as practice")
	}
	if web.playerProfile.Energy != remaining-1 {
		t.Fatal("campaign replay did not charge one Energy")
	}
	finishTestMission(t, web)
	if web.playerProfile.Marks != marks || web.playerProfile.TotalGames != games || web.playerProfile.Energy != remaining-1 {
		t.Fatal("practice changed rewards or statistics, or charged the wrong Energy")
	}
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 2}); err != nil {
		t.Fatal(err)
	}
	finishTestMission(t, web)
	if web.session.state.Clashes != 1 {
		t.Fatalf("scripted mirror produced %d clashes", web.session.state.Clashes)
	}
	if err := web.session.state.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestWebCampaignListingMapsCatalogRewardsAndOmitsHeroStubs(t *testing.T) {
	web := campaignTestServer(t)
	response := httptest.NewRecorder()
	web.handleCampaigns(response, httptest.NewRequest(http.MethodGet, "/api/campaigns", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("campaign listing status = %d: %s", response.Code, response.Body.String())
	}
	var page webCampaignPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]webCampaignSummary, len(page.Campaigns))
	for _, campaign := range page.Campaigns {
		byID[campaign.ID] = campaign
		for _, mission := range campaign.Missions {
			wantMarks, wantCrowns := expectedCampaignMissionReward(mission.Index)
			if mission.RewardMarks != wantMarks || mission.RewardCrowns != wantCrowns {
				t.Errorf("%s mission %d reward = %d Marks, %d Crowns; want %d, %d",
					campaign.ID, mission.Index, mission.RewardMarks, mission.RewardCrowns,
					wantMarks, wantCrowns)
			}
		}
	}
	tugWar := byID["tug-war"]
	if tugWar.Variant != "tug-war" || len(tugWar.Missions) != 15 ||
		tugWar.Missions[0].Variant != "tug-war" || !tugWar.Missions[0].CanPlay ||
		tugWar.Missions[1].CanPlay {
		t.Fatalf("Tug War campaign listing = %+v", tugWar)
	}
	for _, id := range []string{"ladder", "rummy"} {
		campaign := byID[id]
		if campaign.Variant != id || len(campaign.Missions) != 15 ||
			campaign.Status != "" || !campaign.Missions[0].Enabled ||
			campaign.Missions[0].Status != "" {
			t.Errorf("%s campaign listing = %+v", id, campaign)
		}
		for index, mission := range campaign.Missions {
			if mission.Variant != id || !mission.Enabled ||
				mission.Name == "" || mission.Description == "" {
				t.Errorf("%s API mission %d = %+v", id, index+1, mission)
			}
		}
		if id == "ladder" && len(campaign.Missions[14].OpponentHeroes) != len(heroes.All()) {
			t.Errorf("Ladder Gauntlet offers %d CPU heroes, want %d",
				len(campaign.Missions[14].OpponentHeroes), len(heroes.All()))
		}
	}
	poker := byID["poker"]
	pokerNames := []string{
		"The First Hand", "Finding the Nine", "Your Suit", "The Even Table", "The Bonus Table",
		"Chase the Scissors", "Chase the Paper", "Silent Nines", "The Wall",
		"Free Play", "High Bar", "Consistency", "Doubled Bounty", "Nine Storm", "The Crown",
	}
	if poker.Variant != "poker" || len(poker.Missions) != len(pokerNames) {
		t.Fatalf("Poker campaign listing = %+v", poker)
	}
	for index, mission := range poker.Missions {
		if mission.Name != pokerNames[index] || mission.Variant != "poker" ||
			mission.Target <= 0 {
			t.Errorf("Poker API mission %d = %+v", index+1, mission)
		}
	}
	if len(poker.Missions[0].RecommendedHeroes) != 2 ||
		len(poker.Missions[5].RecommendedHeroes) != 3 ||
		len(poker.Missions[11].RecommendedHeroes) != 3 ||
		len(poker.Missions[14].RecommendedHeroes) != 0 ||
		!strings.Contains(poker.Missions[11].Description, "175 points for 3 consecutive games") {
		t.Fatal("Poker recommendations, consistency par, or final hidden-hint metadata are incorrect")
	}
	abandon := byID["abandon"]
	abandonNames := []string{
		"First Blood", "Mirror", "The Tie", "Count and Peak", "The Bounty",
		"Don't Abandon", "Flip and Find", "Blind Gamble", "The Mirror Match",
		"Snake Eyes", "Two Rounds", "All-In", "Spread", "Tempo", "Abandon",
	}
	if abandon.Variant != "abandon" || len(abandon.Missions) != 15 {
		t.Fatalf("abandon campaign listing = %+v", abandon)
	}
	for index, mission := range abandon.Missions {
		if mission.Variant != "abandon" || mission.Name != abandonNames[index] ||
			mission.PairCount <= 0 || mission.CardCount <= 0 || !mission.Enabled {
			t.Errorf("abandon API mission %d = %+v", index+1, mission)
		}
		if mission.CanPlay != (index == 0) {
			t.Errorf("abandon mission %d canPlay = %v", index+1, mission.CanPlay)
		}
	}
	puzzler := byID["puzzler"]
	puzzleIDs := []string{
		"gp6_balanced", "gp6_hunters_edge", "gp6_bounty_trap", "gp6_narrow_gate",
		"gp6_bodybuilders_wall", "gp8_balanced", "gp8_bounty_climb", "gp8_forgiving",
		"gp8_gamblers_puzzle", "gp8_tight", "gp10_balanced", "gp10_gorilla_threat",
		"gp10_turtle_lead", "gp10_low_score", "gp10_everyone_solves", "gp12_balanced",
		"gp12_dual_threat", "gp12_shift_dominant", "gp12_forgiving", "gp12_inverted",
	}
	if puzzler.Variant != "puzzler" || len(puzzler.Missions) != len(puzzleIDs) {
		t.Fatalf("puzzler campaign listing = %+v", puzzler)
	}
	for index, mission := range puzzler.Missions {
		if mission.VariantID != puzzleIDs[index] || mission.Variant != "puzzler" ||
			mission.Target <= 0 || mission.CardCount <= 0 {
			t.Errorf("puzzler API mission %d = %+v", index+1, mission)
		}
	}
	for _, id := range []string{"warden", "hunter", "treasurer", "ascendant", "anchor", "gambler", "diplomat", "ascetic", "herald", "hater"} {
		if _, exists := byID[id]; exists {
			t.Errorf("hero campaign stub %q is still listed", id)
			continue
		}
	}
}

func TestRummyAndLadderCampaignMissionsLaunchWithValidDecks(t *testing.T) {
	for _, campaignID := range []string{"rummy", "ladder"} {
		campaign, ok := campaigns.ByID(campaignID)
		if !ok {
			t.Fatalf("missing campaign %q", campaignID)
		}
		for _, mission := range campaign.Missions {
			t.Run(fmt.Sprintf("%s/%02d", campaignID, mission.Index), func(t *testing.T) {
				web := campaignTestServer(t)
				for _, hero := range heroes.All() {
					web.playerProfile.UnlockedHeroes = append(web.playerProfile.UnlockedHeroes, hero.ID)
				}
				for index := 1; index < mission.Index; index++ {
					if _, err := campaigns.CompleteMission(web.playerProfile, campaignID, index, 0); err != nil {
						t.Fatal(err)
					}
				}
				heroID := "recruit"
				if len(mission.PlayerHeroRestriction) > 0 {
					heroID = mission.PlayerHeroRestriction[0]
				}
				opponentHeroID := ""
				if campaignID == "ladder" && mission.Index == 15 {
					opponentHeroID = "hunter"
				}
				if err := web.prepareWebMission(actionRequest{
					CampaignID: campaignID, MissionIndex: mission.Index, PlayerHeroID: heroID,
					OpponentHeroID: opponentHeroID,
				}); err != nil {
					t.Fatal(err)
				}
				if web.session.campaign == nil || web.session.variant != campaignID {
					t.Fatalf("campaign launch context = %+v", web.session)
				}
				seen := make(map[game.Card]bool, len(game.NewDeck()))
				add := func(card game.Card) {
					t.Helper()
					if !card.Valid() || seen[card] {
						t.Errorf("invalid or duplicate setup card %s", card)
					}
					seen[card] = true
				}
				if campaignID == "rummy" {
					if web.session.rummy == nil || web.session.rummy.Rules == nil {
						t.Fatal("Rummy campaign state or rules are missing")
					}
					for _, hand := range web.session.rummy.Hands {
						for _, card := range hand {
							add(card)
						}
					}
					for _, card := range web.session.rummy.DrawPile {
						add(card)
					}
					for _, card := range web.session.rummy.FaceUp {
						add(card)
					}
				} else {
					if web.session.ladder == nil {
						t.Fatal("Ladder campaign state is missing")
					}
					if mission.Index == 15 && web.session.opponentHeroID != opponentHeroID {
						t.Fatalf("revealed Ladder CPU hero = %q, want %q",
							web.session.opponentHeroID, opponentHeroID)
					}
					state := web.session.ladder.State()
					if mission.Index == 15 {
						playerHero, _ := heroes.ByID(heroID)
						cpuHero, _ := heroes.ByID(opponentHeroID)
						if got, want := state.SharedStats, ladder.SharedStatsFor(playerHero, cpuHero); got != want {
							t.Errorf("Ladder Gauntlet shared stats = %+v, want matchup stats %+v", got, want)
						}
					}
					for _, card := range state.HandA {
						add(card)
					}
					for _, card := range state.HandB {
						add(card)
					}
					for _, card := range state.Deck {
						add(card)
					}
					add(state.BountyCard)
				}
				if len(seen) != len(game.NewDeck()) {
					t.Errorf("campaign setup contains %d cards, want %d", len(seen), len(game.NewDeck()))
				}
			})
		}
	}
}

func TestSuccessfulSpecialCampaignMissionAwardsMarksOnlyOnce(t *testing.T) {
	web := campaignTestServer(t)
	beforeMarks, beforeEnergy := web.playerProfile.Marks, web.playerProfile.Energy
	body, _ := json.Marshal(map[string]any{
		"campaignId": "puzzler", "missionIndex": 1, "playerHeroId": "recruit",
	})
	response := httptest.NewRecorder()
	web.handleNewGame(response, httptest.NewRequest(http.MethodPost, "/api/new", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("campaign launch status = %d: %s", response.Code, response.Body.String())
	}
	if web.session.variant != "puzzler" || web.session.campaign == nil ||
		web.session.campaign.Variant != "puzzler" ||
		web.session.campaign.VariantID != "gp6_balanced" {
		t.Fatalf("campaign puzzle session context = %+v", web.session)
	}
	snapshotJSON, err := json.Marshal(snapshotSession(web.session))
	if err != nil {
		t.Fatal(err)
	}
	var persisted sessionSnapshot
	if err := json.Unmarshal(snapshotJSON, &persisted); err != nil {
		t.Fatal(err)
	}
	restored := persisted.restore()
	if restored.campaign == nil || restored.campaign.Variant != "puzzler" ||
		restored.campaign.VariantID != "gp6_balanced" {
		t.Fatalf("campaign context was not persisted: %+v", restored.campaign)
	}
	hero, _ := heroes.ByID("recruit")
	web.session.puzzle = newPuzzleSession(puzzleDefinition{
		ID: "gp6_balanced", Cards: []game.Card{
			{Suit: game.Shield, Face: 2}, {Suit: game.Shield, Face: 2},
		},
		Par: 0, Direction: "low",
	}, hero)
	for _, index := range []int{0, 1} {
		if err := web.applyAction(actionRequest{Action: "puzzle-pick", Index: index}); err != nil {
			t.Fatal(err)
		}
	}
	if web.session.phase != "gameover" || web.session.winner != 0 ||
		!missionIsComplete(web.playerProfile.Campaigns["puzzler"], 1) ||
		!campaigns.CanPlayMission(web.playerProfile, "puzzler", 2) {
		t.Fatalf("successful puzzle did not advance campaign: phase=%s progress=%+v",
			web.session.phase, web.playerProfile.Campaigns["puzzler"])
	}
	if web.playerProfile.Marks != beforeMarks+350 || web.playerProfile.Energy != beforeEnergy-1 {
		t.Fatalf("puzzle campaign reward/energy = marks %d→%d, energy %d→%d",
			beforeMarks, web.playerProfile.Marks, beforeEnergy, web.playerProfile.Energy)
	}
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "puzzler", MissionIndex: 1, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	if !web.session.campaign.Replay {
		t.Fatal("completed puzzle mission was not marked as a replay")
	}
	if web.playerProfile.Energy != beforeEnergy-2 {
		t.Fatal("special campaign replay did not charge one Energy")
	}
	web.finishSpecialCampaignMission(true)
	if web.playerProfile.Marks != beforeMarks+350 {
		t.Fatalf("puzzle replay changed Marks to %d, want %d", web.playerProfile.Marks, beforeMarks+350)
	}
}

func TestVariantCampaignCompletionAwardsAchievementAndConsumables(t *testing.T) {
	achievementIDs := map[string]string{
		"abandon": "camp_abandon",
		"ladder":  "camp_ladder",
		"poker":   "camp_poker",
		"rummy":   "camp_rummy",
		"tug-war": "camp_tug_war",
	}
	for campaignID, achievementID := range achievementIDs {
		t.Run(campaignID, func(t *testing.T) {
			web := campaignTestServer(t)
			campaign, ok := campaigns.ByID(campaignID)
			if !ok {
				t.Fatalf("missing campaign %q", campaignID)
			}
			for index := 1; index < len(campaign.Missions); index++ {
				if _, err := campaigns.CompleteMission(web.playerProfile, campaignID, index, 0); err != nil {
					t.Fatal(err)
				}
			}
			marks, crowns := web.playerProfile.Marks, web.playerProfile.Crowns
			web.session = &webSession{
				campaign:            &webCampaignContext{ID: campaignID, Index: len(campaign.Missions)},
				initialAchievements: achievementLevels(web.playerProfile),
			}
			web.finishSpecialCampaignMission(true)
			p := web.playerProfile
			if !p.Achievements[achievementID] || p.Marks != marks+3350 || p.Crowns != crowns+12 ||
				p.UndoMoveTokens != 0 || p.RestartMissionTokens != 0 ||
				p.CampaignSkipTokens != 0 || p.ReplaceCardTokens != 0 {
				t.Fatalf("campaign completion rewards for %s: achievement=%t marks=%d crowns=%d tokens=%d/%d/%d/%d",
					campaignID, p.Achievements[achievementID], p.Marks-marks, p.Crowns-crowns,
					p.UndoMoveTokens, p.RestartMissionTokens, p.CampaignSkipTokens, p.ReplaceCardTokens)
			}
			if len(web.session.achievementUpdates) != 1 ||
				!strings.Contains(web.session.achievementUpdates[0], "3000 Marks") ||
				!strings.Contains(web.session.achievementUpdates[0], "10 Crowns") {
				t.Fatalf("completion achievement update = %v", web.session.achievementUpdates)
			}
		})
	}
}

func TestAbandonCampaignLaunchUsesScriptedMissionAndPersistsCampaignContext(t *testing.T) {
	web := campaignTestServer(t)
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "abandon", MissionIndex: 1, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	if web.session.variant != "abandon" || web.session.abandon == nil ||
		web.session.abandon.RulesVersion != abandonRulesVersion ||
		web.session.abandon.PairCount != 1 || len(web.session.abandon.Board) != 6 ||
		len(web.session.abandon.PersonalPile) != 3 || web.session.abandon.Target != 1 ||
		web.session.abandon.Bounty.Suit != game.Shield || web.session.phase != "abandon-reveal" ||
		web.session.campaign == nil || web.session.campaign.Variant != "abandon" ||
		web.session.campaign.VariantID != "" || !web.session.campaign.HasNextMission {
		t.Fatalf("abandon campaign launch = %+v", web.session)
	}
	restored := snapshotSession(web.session).restore()
	if restored.campaign == nil || restored.campaign.VariantID != "" ||
		restored.abandon.RulesVersion != abandonRulesVersion ||
		len(restored.abandon.PersonalPile) != 3 {
		t.Fatalf("abandon campaign snapshot = %+v", restored)
	}
	marks := web.playerProfile.Marks
	web.session.abandon.Finished, web.session.abandon.Won = true, true
	web.updateAbandonPhase()
	if !missionIsComplete(web.playerProfile.Campaigns["abandon"], 1) ||
		!campaigns.CanPlayMission(web.playerProfile, "abandon", 2) ||
		web.playerProfile.Marks != marks+350 || !web.session.campaign.HasNextMission {
		t.Fatalf("successful Abandon completion did not advance with 350 Marks: %+v",
			web.playerProfile.Campaigns["abandon"])
	}
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "abandon", MissionIndex: 1, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	if !web.session.campaign.Replay {
		t.Fatal("completed Abandon mission was not marked as a replay")
	}
	web.finishSpecialCampaignMission(true)
	if web.playerProfile.Marks != marks+350 {
		t.Fatalf("Abandon replay changed Marks to %d, want %d", web.playerProfile.Marks, marks+350)
	}
}

func TestAbandonCampaignLaunchesEveryMissionWithValidSetup(t *testing.T) {
	campaign, ok := campaigns.ByID("abandon")
	if !ok {
		t.Fatal("Abandon campaign is not registered")
	}
	for _, mission := range campaign.Missions {
		t.Run(mission.Name, func(t *testing.T) {
			web := campaignTestServer(t)
			for index := 1; index < mission.Index; index++ {
				if _, err := campaigns.CompleteMission(web.playerProfile, "abandon", index, 0); err != nil {
					t.Fatal(err)
				}
			}
			if err := web.prepareWebMission(actionRequest{
				CampaignID: "abandon", MissionIndex: mission.Index, PlayerHeroID: "recruit",
			}); err != nil {
				t.Fatal(err)
			}
			if mission.Abandon.Rounds == 1 && mission.Abandon.OpponentAlgo == "" {
				state := web.session.abandon
				if web.session.variant != "abandon" || state == nil ||
					len(state.Board) != mission.Abandon.BoardSize ||
					len(state.PersonalPile) != mission.Abandon.HandSize ||
					state.PairCount != mission.Abandon.PairCount {
					t.Fatalf("solo mission setup = %+v", web.session)
				}
				seen := map[game.Card]bool{state.Bounty: true}
				for _, cards := range [][]game.Card{state.Board, state.PersonalPile, state.DrawPile} {
					for _, card := range cards {
						if seen[card] {
							t.Fatalf("solo mission contains duplicate card %s", card)
						}
						seen[card] = true
					}
				}
				return
			}
			state := web.session.abandonDuel
			if web.session.variant != "abandon-duel" || state == nil ||
				len(state.Board) != mission.Abandon.BoardSize ||
				state.turnLimit() != 2*mission.Abandon.PairCount ||
				state.Setup.OpponentAlgo != mission.Abandon.OpponentAlgo {
				t.Fatalf("duel mission setup = %+v", web.session)
			}
			if err := state.validateState(); err != nil {
				t.Fatalf("duel mission setup is invalid: %v", err)
			}
		})
	}
}

func TestAbandonDuelCampaignMissionsCompleteWithConfiguredRoundCounts(t *testing.T) {
	campaign, _ := campaigns.ByID("abandon")
	for _, mission := range campaign.Missions[8:] {
		t.Run(mission.Name, func(t *testing.T) {
			web := campaignTestServer(t)
			for index := 1; index < mission.Index; index++ {
				if _, err := campaigns.CompleteMission(web.playerProfile, "abandon", index, 0); err != nil {
					t.Fatal(err)
				}
			}
			if err := web.prepareWebMission(actionRequest{
				CampaignID: "abandon", MissionIndex: mission.Index, PlayerHeroID: "recruit",
			}); err != nil {
				t.Fatal(err)
			}
			duel := web.session.abandonDuel
			for !duel.GameOver {
				if duel.ActivePlayer == 1 {
					if err := web.advanceAbandonDuelAI(); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if !duel.FlippedThisTurn {
					hidden := duel.hiddenBoardIndices()
					if len(hidden) > 0 {
						if err := duel.flipBoardCard(hidden[0]); err != nil {
							t.Fatal(err)
						}
					}
				}
				firstSource, firstIndex := firstAvailableAbandonCard(duel, 0, "", -1)
				if firstIndex < 0 {
					t.Fatal("human player has no first card for pairing")
				}
				if _, err := duel.selectPairCard(firstSource, firstIndex, web.rng); err != nil {
					t.Fatal(err)
				}
				secondSource, secondIndex := firstAvailableAbandonCard(duel, 0, firstSource, firstIndex)
				if secondIndex < 0 {
					t.Fatal("human player has no second card for pairing")
				}
				if _, err := duel.selectPairCard(secondSource, secondIndex, web.rng); err != nil {
					t.Fatal(err)
				}
			}
			web.session.winner = duel.Winner
			web.finishAbandonDuel()
			if duel.Round != mission.Abandon.Rounds ||
				duel.RoundPairCounts[0][0] > mission.Abandon.PairCount ||
				duel.RoundPairCounts[0][1] > mission.Abandon.PairCount ||
				web.session.campaign.Result == "" {
				t.Fatalf("campaign duel did not finish as configured: round=%d pairs=%v result=%q",
					duel.Round, duel.RoundPairCounts, web.session.campaign.Result)
			}
			if mission.Index == 15 &&
				(duel.RoundPairCounts[0][0] != mission.Abandon.PairCount ||
					duel.RoundPairCounts[0][1] != mission.Abandon.PairCount) {
				t.Fatalf("final mission completed %v pairs, want %d per player",
					duel.RoundPairCounts[0], mission.Abandon.PairCount)
			}
		})
	}
}

func firstAvailableAbandonCard(duel *abandonDuelGame, player int, excludedSource string, excludedIndex int) (string, int) {
	for index, used := range duel.Players[player].PileUsed {
		if !used && (excludedSource != abandonPileSource || excludedIndex != index) {
			return abandonPileSource, index
		}
	}
	for index, used := range duel.BoardUsed {
		if !used && (excludedSource != abandonBoardSource || excludedIndex != index) {
			return abandonBoardSource, index
		}
	}
	return "", -1
}

func TestTugWarCampaignLaunchesAllMissionsWithLegalCPUActions(t *testing.T) {
	campaign, ok := campaigns.ByID("tug-war")
	if !ok {
		t.Fatal("Tug War campaign is not registered")
	}
	for _, mission := range campaign.Missions {
		t.Run(mission.Name, func(t *testing.T) {
			web := campaignTestServer(t)
			completeTugWarPrerequisites(t, web, mission.Index)
			if err := web.prepareWebMission(actionRequest{
				CampaignID: campaign.ID, MissionIndex: mission.Index, PlayerHeroID: "recruit",
			}); err != nil {
				t.Fatal(err)
			}
			if web.session.variant != "tug-war" || web.session.tugWar == nil ||
				web.session.campaign == nil || web.session.campaign.Index != mission.Index {
				t.Fatalf("Tug War mission launch = %+v", web.session)
			}
			for actionCount := 0; actionCount < 500 && !web.session.tugWar.State.Over; actionCount++ {
				match := web.session.tugWar
				action := match.ChooseCPUAction()
				if match.State.Current == 1 {
					action = match.ChooseCampaignCPUAction(web.rng)
				}
				if err := match.Apply(action); err != nil {
					t.Fatalf("player %d action %d (%+v): %v",
						match.State.Current, actionCount, action, err)
				}
			}
			if !web.session.tugWar.State.Over {
				t.Fatal("Tug War mission did not finish in 500 actions")
			}
		})
	}
}

func TestTugWarCampaignHidesMissionSixInformation(t *testing.T) {
	web := campaignTestServer(t)
	completeTugWarPrerequisites(t, web, 6)
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "tug-war", MissionIndex: 6, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	view := tugWarWebView(web, web.session)
	if view.BountyRevealed || view.BountySuit != nil || view.ValuesRevealed ||
		!view.Piles[0].Hidden || view.Piles[0].Cards[len(view.Piles[0].Cards)-1] != (game.Card{}) {
		t.Fatalf("M6 secret information leaked in web view: %+v", view)
	}
	match := web.session.tugWar
	match.State.Current = 0
	// A blind losing hand play is no longer permitted; it must not consume
	// the card or reveal the pile. Find a legal hand play if one exists.
	if !match.CanPlayCard(0, 0, 0) {
		before := len(match.State.Hands[0])
		if err := match.Apply(tugwar.Action{Kind: tugwar.ActionPlayCard, CardIndex: 0, PileIndex: 0}); err == nil {
			t.Fatal("illegal blind play was accepted")
		}
		if len(match.State.Hands[0]) != before || !match.State.Piles[0].Hidden {
			t.Fatal("illegal blind play consumed a card or revealed a pile")
		}
	}
	played := false
	for cardIndex := range match.State.Hands[0] {
		for pileIndex := range match.State.Piles {
			if !match.CanPlayCard(0, cardIndex, pileIndex) { continue }
			if err := match.Apply(tugwar.Action{Kind: tugwar.ActionPlayCard, CardIndex: cardIndex, PileIndex: pileIndex}); err != nil { t.Fatal(err) }
			view = tugWarWebView(web, web.session)
			if view.Piles[pileIndex].Hidden || view.BountyRevealed || view.BountySuit != nil {
				t.Fatal("legal hand play should reveal its pile, not the bounty")
			}
			played = true
			break
		}
		if played { break }
	}
	match.State.Current = 0
	if err := match.Apply(tugwar.Action{Kind: tugwar.ActionFlipDeck, PileIndex: 1}); err != nil {
		t.Fatal(err)
	}
	view = tugWarWebView(web, web.session)
	if !view.BountyRevealed || view.BountySuit == nil {
		t.Fatal("first deck flip did not reveal the bounty suit")
	}
}

func TestTugWarCampaignSeriesAdvancesAndRestores(t *testing.T) {
	web := campaignTestServer(t)
	completeTugWarPrerequisites(t, web, 14)
	if err := web.prepareWebMission(actionRequest{
		CampaignID: "tug-war", MissionIndex: 14, PlayerHeroID: "recruit",
	}); err != nil {
		t.Fatal(err)
	}
	if web.session.tugWarSeries == nil || web.session.tugWarSeries.Game != 1 ||
		web.session.opponentHeroID != "recruit" ||
		web.session.tugWar.Mission.CPUPolicy != tugwar.CPUPolicyPassive {
		t.Fatalf("Gauntlet game 1 setup = %+v", web.session)
	}
	web.session.tugWar.State.Over, web.session.tugWar.State.Winner = true, 0
	web.updateTugWarSession()
	web.session = snapshotSession(web.session).restore()
	if web.session.tugWarSeries.PlayerWins != 1 {
		t.Fatalf("restored Gauntlet series = %+v", web.session.tugWarSeries)
	}
	if err := web.applyAction(actionRequest{Action: "next-tug-war-game"}); err != nil {
		t.Fatal(err)
	}
	if web.session.tugWarSeries.Game != 2 || web.session.opponentHeroID != "warden" ||
		web.session.tugWar.Mission.CPUPolicy != tugwar.CPUPolicyPushIt ||
		web.session.playerHeroID != "recruit" {
		t.Fatalf("Gauntlet game 2 setup = %+v", web.session)
	}
}

func completeTugWarPrerequisites(t *testing.T, web *webServer, missionIndex int) {
	t.Helper()
	for index := 1; index < missionIndex; index++ {
		if _, err := campaigns.CompleteMission(web.playerProfile, "tug-war", index, 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAbandonWebActionsRequireFlipThenTwoCards(t *testing.T) {
	web := campaignTestServer(t)
	for missionIndex := 1; missionIndex < 4; missionIndex++ {
		if _, err := campaigns.CompleteMission(web.playerProfile, "abandon", missionIndex, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := web.prepareWebMission(actionRequest{CampaignID: "abandon", MissionIndex: 4, PlayerHeroID: "recruit"}); err != nil {
		t.Fatal(err)
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: abandonBoardSource, Index: 0}); err == nil {
		t.Fatal("pairing was allowed before the reveal phase")
	}
	hiddenIndex := -1
	for index, revealed := range web.session.abandon.Revealed {
		if !revealed {
			hiddenIndex = index
			break
		}
	}
	if hiddenIndex < 0 {
		t.Fatal("random board has no face-down card")
	}
	if err := web.applyAction(actionRequest{Action: "abandon-flip", Index: hiddenIndex}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-pair-first" || !web.session.abandon.Revealed[hiddenIndex] {
		t.Fatalf("reveal action did not advance to pairing: phase=%q", web.session.phase)
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: abandonBoardSource, Index: 0}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-pair-second" {
		t.Fatalf("first card selection phase = %q", web.session.phase)
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: abandonBoardSource, Index: 0}); err == nil {
		t.Fatal("same card was allowed twice in one pairing")
	}
	secondSource, secondIndex := abandonPileSource, -1
	for index, revealed := range web.session.abandon.PileRevealed {
		if !revealed && !web.session.abandon.PileUsed[index] {
			secondIndex = index
			break
		}
	}
	if secondIndex < 0 {
		secondSource = abandonBoardSource
		for index, revealed := range web.session.abandon.Revealed {
			if index != 0 && !revealed && !web.session.abandon.Used[index] {
				secondIndex = index
				break
			}
		}
	}
	if secondIndex < 0 {
		t.Fatal("no hidden second card was available")
	}
	if err := web.applyAction(actionRequest{Action: "abandon-reveal-pair-card", Source: secondSource, Index: secondIndex}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-pair-second" || len(web.session.abandon.Pairings) != 0 {
		t.Fatalf("revealing the second card resolved the pair: phase=%q pairs=%d",
			web.session.phase, len(web.session.abandon.Pairings))
	}
	revealedView := web.session.abandon.webView()
	secondPile := revealedView.PersonalPile
	if secondSource == abandonBoardSource {
		secondPile = revealedView.Board
	}
	if secondPile[secondIndex].Card == nil {
		t.Fatal("revealed second card was not included in the Abandon view")
	}
	if err := web.applyAction(actionRequest{Action: "abandon-pair-card", Source: secondSource, Index: secondIndex}); err != nil {
		t.Fatal(err)
	}
	if web.session.phase != "abandon-reveal" || len(web.session.abandon.Pairings) != 1 {
		t.Fatalf("pair action did not advance to next turn: phase=%q pairs=%d",
			web.session.phase, len(web.session.abandon.Pairings))
	}
}

func TestStandalonePuzzlerRequiresCampaignMission(t *testing.T) {
	web := campaignTestServer(t)
	body, _ := json.Marshal(map[string]any{
		"mode": "easy", "variant": "puzzler", "puzzleId": "gp6_balanced",
		"playerHeroId": "recruit",
	})
	response := httptest.NewRecorder()
	web.handleNewGame(response, httptest.NewRequest(http.MethodPost, "/api/new", bytes.NewReader(body)))
	if response.Code != http.StatusBadRequest || web.session != nil {
		t.Fatalf("standalone puzzler status/session = %d / %+v: %s",
			response.Code, web.session, response.Body.String())
	}
}

func TestEveryWebCampaignMissionCompletesWithRestoredState(t *testing.T) {
	for _, campaign := range campaigns.All() {
		for _, mission := range campaign.Missions {
			if mission.Variant != "" || mission.Disabled {
				continue
			}
			t.Run(campaign.ID+"/"+mission.Name, func(t *testing.T) {
				web := campaignTestServer(t)
				for _, prerequisite := range campaigns.All() {
					if prerequisite.ID == campaign.ID {
						break
					}
					for _, previous := range prerequisite.Missions {
						if _, err := campaigns.CompleteMission(web.playerProfile, prerequisite.ID, previous.Index, 0); err != nil {
							t.Fatal(err)
						}
					}
				}
				for _, previous := range campaign.Missions {
					if previous.Index >= mission.Index {
						break
					}
					if _, err := campaigns.CompleteMission(web.playerProfile, campaign.ID, previous.Index, 0); err != nil {
						t.Fatal(err)
					}
				}
				if err := web.prepareWebMission(actionRequest{CampaignID: campaign.ID, MissionIndex: mission.Index}); err != nil {
					t.Fatal(err)
				}
				finishTestMission(t, web)
				if err := web.session.state.Validate(); err != nil {
					t.Fatal(err)
				}
				wantRounds := 3
				if mission.Type == campaigns.SingleRound {
					wantRounds = 1
				}
				if web.session.state.Round != wantRounds {
					t.Fatalf("finished in round %d, want %d", web.session.state.Round, wantRounds)
				}
				if mission.Name == "No Draft Duel" && web.session.battles[0].Attacker == "You" {
					t.Fatal("No Draft Duel did not make the player defend first")
				}
				if mission.Name == "Harsh" && web.session.battles[0].Attacker != "You" {
					t.Fatal("Harsh did not make the player attack first in Round 1")
				}
				if mission.PlayerAttacksFirstAll {
					for round := 0; round < 3; round++ {
						if got := web.session.battles[round*3].Attacker; got != "You" {
							t.Errorf("round %d first attacker = %q, want You", round+1, got)
						}
					}
				}
			})
		}
	}
}
