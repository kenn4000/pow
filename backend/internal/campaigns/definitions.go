package campaigns

import (
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/tugwar"
)

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
	Variant               string
	VariantID             string
	Status                string
	Disabled              bool
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
	RewardCrowns          int64
	Target                int
	AbandonPairs          int
	AbandonBoardCards     int
	Abandon               *AbandonCampaignRules
	PokerRules            *game.PokerMissionRules
	TugWar                *tugwar.CampaignMission
	Rummy                 *RummyCampaignRules
	Ladder                *LadderCampaignRules
}

type AbandonCampaignRules struct {
	Board        []game.Card
	PlayerHand   []game.Card
	CPUHand      []game.Card
	BoardSize    int
	HandSize     int
	PairCount    int
	Rounds       int
	BountySuits  [2]int
	MirrorHands  bool
	Snake        bool
	Goal         string
	Target       int
	TargetCard   *game.Card
	MinCaptured  int
	MaxAbandoned int
	OpponentAlgo string
}

type LadderCampaignRules struct {
	WildFace    int
	BountySuit  game.Suit
	PlayerHand  []game.Card
	CPUHand     []game.Card
	PlayerSize  int
	CPUSize     int
	SharedStats *LadderSharedStats
	CPUPolicy   string
	OpenHand    bool
	RandomHands bool
}

type LadderSharedStats struct {
	Bounty int
	Draft  int
	Shift  int
	Favor  int
}

type RummyCampaignRules struct {
	Parameters      game.RummyParameters
	PlayerHand      []game.Card
	CPUHand         []game.Card
	DrawPileTop     []game.Card
	InitialFaceUp   []game.Card
	TurnLimit       int
	Goal            string
	Target          int
	CPUPolicy       string
	MatchHands      int
	ScriptedDiscard *game.Card
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
	Variant     string
	Status      string
	Missions    []Mission
	Completion  CampaignRewards
	Requires    string
}

const (
	BasicMissionMarks     int64 = 350
	BasicCompleteMarks    int64 = 1000
	AdvancedMissionMarks  int64 = 350
	AdvancedCompleteMarks int64 = 2000
)

var definitions = func() []Campaign {
	result := []Campaign{
		basicCampaign(), advancedCampaign(), abandonCampaign(), puzzlerCampaign(),
		pokerCampaign(), tugWarCampaign(), ladderCampaign(), rummyCampaign(),
	}
	// All campaigns use the same reward tiers, keyed by mission number.
	for campaignIndex := range result {
		for missionIndex := range result[campaignIndex].Missions {
			mission := &result[campaignIndex].Missions[missionIndex]
			switch {
			case mission.Index <= 5:
				mission.RewardMarks, mission.RewardCrowns = 350, 0
			case mission.Index <= 10:
				mission.RewardMarks, mission.RewardCrowns = 300, 1
			default:
				mission.RewardMarks, mission.RewardCrowns = 350, 2
			}
		}
	}
	return result
}()

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
		if mission.Abandon != nil {
			rules := *mission.Abandon
			rules.Board = append([]game.Card(nil), rules.Board...)
			rules.PlayerHand = append([]game.Card(nil), rules.PlayerHand...)
			rules.CPUHand = append([]game.Card(nil), rules.CPUHand...)
			if rules.TargetCard != nil {
				card := *rules.TargetCard
				rules.TargetCard = &card
			}
			mission.Abandon = &rules
		}
		if mission.TugWar != nil {
			mission.TugWar = tugwar.CloneCampaignMission(*mission.TugWar)
		}
		if mission.Rummy != nil {
			rules := *mission.Rummy
			rules.PlayerHand = append([]game.Card(nil), rules.PlayerHand...)
			rules.CPUHand = append([]game.Card(nil), rules.CPUHand...)
			rules.DrawPileTop = append([]game.Card(nil), rules.DrawPileTop...)
			rules.InitialFaceUp = append([]game.Card(nil), rules.InitialFaceUp...)
			if rules.ScriptedDiscard != nil {
				card := *rules.ScriptedDiscard
				rules.ScriptedDiscard = &card
			}
			mission.Rummy = &rules
		}
		if mission.Ladder != nil {
			rules := *mission.Ladder
			rules.PlayerHand = append([]game.Card(nil), rules.PlayerHand...)
			rules.CPUHand = append([]game.Card(nil), rules.CPUHand...)
			if rules.SharedStats != nil {
				stats := *rules.SharedStats
				rules.SharedStats = &stats
			}
			mission.Ladder = &rules
		}
		if mission.PokerRules != nil {
			rules := *mission.PokerRules
			rules.StartingHand = append([]game.Card(nil), rules.StartingHand...)
			rules.RecommendedHeroes = append([]string(nil), rules.RecommendedHeroes...)
			if rules.ForcedBountySuit != nil {
				suit := *rules.ForcedBountySuit
				rules.ForcedBountySuit = &suit
			}
			if rules.Ghost != nil {
				ghost := *rules.Ghost
				ghost.StartingHand = append([]game.Card(nil), ghost.StartingHand...)
				rules.Ghost = &ghost
			}
			mission.PokerRules = &rules
		}
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
