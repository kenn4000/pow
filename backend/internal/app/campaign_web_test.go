package app

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/profile"
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

func TestWebCampaignCompletionReplayAndSnapshots(t *testing.T) {
	web := campaignTestServer(t)
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 2}); err == nil {
		t.Fatal("locked mission was allowed")
	}
	energy := web.playerProfile.Energy
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
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
	if web.session.payout.TotalMarks != 100 {
		t.Fatalf("mission payout = %d", web.session.payout.TotalMarks)
	}
	marks, games, remaining := web.playerProfile.Marks, web.playerProfile.TotalGames, web.playerProfile.Energy
	if err := web.prepareWebMission(actionRequest{CampaignID: "basic", MissionIndex: 1}); err != nil {
		t.Fatal(err)
	}
	if !web.session.campaign.Replay {
		t.Fatal("completed mission was not marked as practice")
	}
	finishTestMission(t, web)
	if web.playerProfile.Marks != marks || web.playerProfile.TotalGames != games || web.playerProfile.Energy != remaining {
		t.Fatal("practice changed rewards, statistics or energy")
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

func TestEveryWebCampaignMissionCompletesWithRestoredState(t *testing.T) {
	for _, campaign := range campaigns.All() {
		for _, mission := range campaign.Missions {
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
