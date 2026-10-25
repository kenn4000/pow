package campaigns

import (
	"errors"
	"fmt"
	"math/rand"

	"prisonersofwar/internal/profile"
)

type MissionOutcome struct {
	MissionMarks       int64
	CampaignCompleted  bool
	AlreadyCompleted   bool
	CompletionMarks    int64
	HeroesUnlocked     []string
	HeroBoxesAwarded   int
	SkillPointsAwarded int
	BadgeAwarded       string
}

func IsUnlocked(p *profile.Profile, campaignID string) bool {
	if p == nil {
		return false
	}
	campaign, ok := ByID(campaignID)
	if !ok {
		return false
	}
	if campaign.Requires == "" {
		return true
	}
	progress := p.Campaigns[campaign.Requires]
	return progress != nil && progress.Completed
}

func Start(p *profile.Profile, campaignID string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	campaign, ok := Find(campaignID)
	if !ok {
		return fmt.Errorf("unknown campaign %q", campaignID)
	}
	if !IsUnlocked(p, campaignID) {
		return fmt.Errorf("%s is locked", campaign.Name)
	}
	progress := ensureProgress(p, campaign)
	progress.Started = true
	return nil
}

func StartCampaign(p *profile.Profile, campaignID string) error {
	return Start(p, campaignID)
}

func CanPlayMission(p *profile.Profile, campaignID string, missionIndex int) bool {
	if p == nil || !IsUnlocked(p, campaignID) {
		return false
	}
	campaign, ok := ByID(campaignID)
	if !ok {
		return false
	}
	progress := ensureProgress(p, campaign)
	if missionIndex < 1 {
		return false
	}
	for _, mission := range campaign.Missions {
		if mission.Index == missionIndex {
			if missionIndex <= progress.MissionsCompleted {
				return true
			}
			return missionIndex == progress.MissionsCompleted+1 &&
				(!progress.Completed || progress.MissionsCompleted < len(campaign.Missions))
		}
	}
	return false
}

func IsCampaignComplete(p *profile.Profile, campaignID string) bool {
	if p == nil {
		return false
	}
	progress := p.Campaigns[campaignID]
	return progress != nil && progress.Completed
}

func PlayNextMission(p *profile.Profile, campaignID string, ui MissionUI, rng *rand.Rand) (MissionResult, error) {
	if err := Start(p, campaignID); err != nil {
		return MissionResult{}, err
	}
	campaign, _ := ByID(campaignID)
	progress := ensureProgress(p, campaign)
	nextIndex := progress.MissionsCompleted + 1
	mission, ok := findMission(campaign, nextIndex)
	if !ok {
		return MissionResult{}, fmt.Errorf("%s has no remaining missions", campaign.Name)
	}
	result, err := RunMission(p, mission, rng, ui)
	if err != nil {
		return MissionResult{}, err
	}
	if result.Won {
		result.Progression, err = CompleteMission(p, campaignID, mission.Index, 0)
		if err != nil {
			return MissionResult{}, err
		}
	}
	return result, nil
}

func CompleteMission(p *profile.Profile, campaignID string, missionIndex int, campaignBonusPercent int) (MissionOutcome, error) {
	if p == nil {
		return MissionOutcome{}, errors.New("profile is required")
	}
	if campaignBonusPercent < 0 {
		return MissionOutcome{}, errors.New("campaign bonus cannot be negative")
	}
	campaign, ok := Find(campaignID)
	if !ok {
		return MissionOutcome{}, fmt.Errorf("unknown campaign %q", campaignID)
	}
	if !IsUnlocked(p, campaignID) {
		return MissionOutcome{}, fmt.Errorf("%s is locked", campaign.Name)
	}
	mission, ok := findMission(campaign, missionIndex)
	if !ok {
		return MissionOutcome{}, fmt.Errorf("mission %d does not exist in %s", missionIndex, campaign.Name)
	}
	progress := ensureProgress(p, campaign)
	for _, completed := range progress.CompletedMissionIDs {
		if completed == missionIndex {
			return MissionOutcome{AlreadyCompleted: true}, nil
		}
	}
	if missionIndex > progress.MissionsCompleted+1 {
		return MissionOutcome{}, fmt.Errorf("mission %d is locked; complete the previous mission first", missionIndex)
	}
	progress.Started = true

	missionMarks := mission.RewardMarks * int64(100+campaignBonusPercent) / 100
	progress.CompletedMissionIDs = append(progress.CompletedMissionIDs, missionIndex)
	progress.MissionsCompleted = len(progress.CompletedMissionIDs)
	p.Marks += missionMarks
	outcome := MissionOutcome{MissionMarks: missionMarks}
	if progress.MissionsCompleted != len(campaign.Missions) {
		return outcome, nil
	}

	progress.Completed = true
	outcome.CampaignCompleted = true
	if !progress.CompletionRewardsAwarded {
		p.Marks += campaign.Completion.Marks
		outcome.CompletionMarks = campaign.Completion.Marks
		progress.CompletionRewardsAwarded = true
		for _, heroID := range campaign.Completion.Heroes {
			if !contains(p.UnlockedHeroes, heroID) {
				p.UnlockedHeroes = append(p.UnlockedHeroes, heroID)
				outcome.HeroesUnlocked = append(outcome.HeroesUnlocked, heroID)
			}
		}
		p.HeroBoxesAvailable += campaign.Completion.HeroBoxes
		outcome.HeroBoxesAwarded = campaign.Completion.HeroBoxes
		if campaign.Completion.Badge != "" && !contains(p.Badges, campaign.Completion.Badge) {
			outcome.BadgeAwarded = campaign.Completion.Badge
			p.Badges = append(p.Badges, campaign.Completion.Badge)
		}
	}
	if campaign.Completion.SkillPoints > 0 && !progress.CompletionSkillPointAwarded {
		p.SkillPoints += campaign.Completion.SkillPoints
		progress.CompletionSkillPointAwarded = true
		outcome.SkillPointsAwarded = campaign.Completion.SkillPoints
	}
	return outcome, nil
}

func findMission(campaign Campaign, index int) (Mission, bool) {
	for _, mission := range campaign.Missions {
		if mission.Index == index {
			return mission, true
		}
	}
	return Mission{}, false
}

func ensureProgress(p *profile.Profile, campaign Campaign) *profile.CampaignProgress {
	if p.Campaigns == nil {
		p.Campaigns = make(map[string]*profile.CampaignProgress)
	}
	progress := p.Campaigns[campaign.ID]
	if progress == nil {
		progress = &profile.CampaignProgress{
			CampaignID: campaign.ID, TotalMissions: len(campaign.Missions),
			CompletedMissionIDs: []int{},
		}
		p.Campaigns[campaign.ID] = progress
	}
	previousTotal := progress.TotalMissions
	progress.CampaignID = campaign.ID
	progress.TotalMissions = len(campaign.Missions)
	if progress.CompletedMissionIDs == nil {
		progress.CompletedMissionIDs = []int{}
	}
	progress.MissionsCompleted = len(progress.CompletedMissionIDs)
	if progress.Completed && previousTotal > 0 && previousTotal < len(campaign.Missions) {
		progress.CompletionRewardsAwarded = true
		progress.Completed = false
	}
	return progress
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
