package campaigns

import "prisonersofwar/internal/game"

type MissionType int

const (
	SingleRound MissionType = iota
	ThreeRound
)

type AILevel int

const (
	AIEasy AILevel = iota
	AINormal
	AIGreedy
)

type Mission struct {
	Index                 int
	Name                  string
	Flavor                string
	Hint                  string
	Type                  MissionType
	DraftMode             int
	PlayerSeat            *game.Seat
	PlayerDraftsFirst     bool
	PlayerAttacksFirstAll bool
	PlayerHand            []game.Card
	CPUHoleCards          []game.Card
	CPULeads              []game.Card
	CPUPool               []game.Card
	CPUAttacksAll         bool
	CPUMirrorIndex        int
	BountyOverride        *int
	ShiftOverride         *int
	FavoredOverride       *int
	ReckoningOverride     *[3]int
	BountySuitOverride    *string
	AILevel               AILevel
	AIHero                string
	PlayerHeroRestriction []string
	RewardMarks           int64
}

type CampaignRewards struct {
	Marks           int64
	Heroes          []string
	HeroBoxes       int
	SkillPoints     int
	Badge           string
	UnlocksCampaign string
}

type Campaign struct {
	ID          string
	Name        string
	Description string
	Missions    []Mission
	Completion  CampaignRewards
	Requires    string
}

const (
	BasicMissionMarks     int64 = 100
	BasicCompleteMarks    int64 = 1000
	AdvancedMissionMarks  int64 = 400
	AdvancedCompleteMarks int64 = 2000
)

var definitions = []Campaign{basicCampaign(), advancedCampaign()}

func All() []Campaign {
	result := make([]Campaign, len(definitions))
	for index, campaign := range definitions {
		result[index] = cloneCampaign(campaign)
	}
	return result
}

func ByID(id string) (Campaign, bool) {
	for _, campaign := range definitions {
		if campaign.ID == id {
			return cloneCampaign(campaign), true
		}
	}
	return Campaign{}, false
}

func Find(id string) (Campaign, bool) {
	return ByID(id)
}

func cloneCampaign(campaign Campaign) Campaign {
	campaign.Missions = append([]Mission(nil), campaign.Missions...)
	for index := range campaign.Missions {
		mission := &campaign.Missions[index]
		mission.PlayerHand = append([]game.Card(nil), mission.PlayerHand...)
		mission.CPUHoleCards = append([]game.Card(nil), mission.CPUHoleCards...)
		mission.CPULeads = append([]game.Card(nil), mission.CPULeads...)
		mission.CPUPool = append([]game.Card(nil), mission.CPUPool...)
		mission.PlayerHeroRestriction = append([]string(nil), mission.PlayerHeroRestriction...)
		if mission.PlayerSeat != nil {
			seat := *mission.PlayerSeat
			mission.PlayerSeat = &seat
		}
		if mission.BountyOverride != nil {
			value := *mission.BountyOverride
			mission.BountyOverride = &value
		}
		if mission.ShiftOverride != nil {
			value := *mission.ShiftOverride
			mission.ShiftOverride = &value
		}
		if mission.FavoredOverride != nil {
			value := *mission.FavoredOverride
			mission.FavoredOverride = &value
		}
		if mission.ReckoningOverride != nil {
			value := *mission.ReckoningOverride
			mission.ReckoningOverride = &value
		}
		if mission.BountySuitOverride != nil {
			value := *mission.BountySuitOverride
			mission.BountySuitOverride = &value
		}
	}
	campaign.Completion.Heroes = append([]string(nil), campaign.Completion.Heroes...)
	return campaign
}

func intValue(value int) *int {
	return &value
}

func weightsValue(value [3]int) *[3]int {
	return &value
}

func stringValue(value string) *string {
	return &value
}

func seatValue(value game.Seat) *game.Seat {
	return &value
}

func card(suit game.Suit, face int) game.Card {
	return game.Card{Suit: suit, Face: face}
}
