package profile

import (
	"errors"
	"fmt"
	"time"
)

const MatchHistoryLimit = 50

var heroCrownsCosts = map[string]int64{
	"warden":    300,
	"hunter":    300,
	"anchor":    300,
	"ascetic":   300,
	"ascendant": 400,
	"treasurer": 400,
	"diplomat":  400,
	"hater":     500,
	"gambler":   500,
	"herald":    500,
}

func HeroCrownsCost(heroID string) (int64, bool) {
	cost, ok := heroCrownsCosts[heroID]
	return cost, ok
}

type MatchRecord struct {
	GameID              string         `json:"game_id,omitempty"`
	At                  time.Time      `json:"at"`
	Mode                int            `json:"mode"`
	PlayerSeat          string         `json:"player_seat"`
	PlayerScore         int            `json:"player_score"`
	OppScore            int            `json:"opp_score"`
	Won                 bool           `json:"won"`
	BountySuit          string         `json:"bounty_suit"`
	PlayerPrisoners     int            `json:"player_prisoners"`
	OppPrisoners        int            `json:"opp_prisoners"`
	PlayerPrisonerSuits map[string]int `json:"player_prisoner_suits"`
	Clashes             int            `json:"clashes"`
	MarksEarned         int64          `json:"marks_earned"`
}

func (p *Profile) RecordMatch(record MatchRecord) {
	if p == nil {
		return
	}
	if record.PlayerPrisonerSuits != nil {
		suitCounts := make(map[string]int, len(record.PlayerPrisonerSuits))
		for suit, count := range record.PlayerPrisonerSuits {
			suitCounts[suit] = count
		}
		record.PlayerPrisonerSuits = suitCounts
	}
	p.MatchHistory = append([]MatchRecord{record}, p.MatchHistory...)
	if len(p.MatchHistory) > MatchHistoryLimit {
		p.MatchHistory = p.MatchHistory[:MatchHistoryLimit]
	}
}

func (p *Profile) ApplyLogin(now time.Time) {
	if p == nil {
		return
	}
	p.ApplyEnergyTick(now)
	today := dateOnly(now)
	if p.LastLoginDate.IsZero() {
		p.LoginStreakCurrent = 1
	} else {
		last := dateOnly(p.LastLoginDate)
		switch {
		case sameDate(last, today):
			return
		case sameDate(last.AddDate(0, 0, 1), today):
			p.LoginStreakCurrent++
		default:
			p.LoginStreakCurrent = 1
		}
	}
	if p.LoginStreakCurrent > p.LoginStreakBest {
		p.LoginStreakBest = p.LoginStreakCurrent
	}
	p.LastLoginDate = today
	if p.SkillNodes["daily_energy"] {
		p.Energy += 2
		if p.Energy > p.MaxEnergy {
			p.Energy = p.MaxEnergy
		}
	}
}

func dateOnly(value time.Time) time.Time {
	local := value.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func sameDate(first, second time.Time) bool {
	y1, m1, d1 := first.Date()
	y2, m2, d2 := second.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func (p *Profile) GrantBox(boxType string, count int) error {
	if p == nil {
		return errors.New("profile is required")
	}
	if !validBoxType(boxType) {
		return fmt.Errorf("invalid box type %q", boxType)
	}
	if count < 0 {
		return errors.New("box count cannot be negative")
	}
	if p.PendingBoxes == nil {
		p.PendingBoxes = make(map[string]int)
	}
	p.PendingBoxes[boxType] += count
	return nil
}

func (p *Profile) BuyHeroWithCrowns(heroID string) error {
	if p == nil {
		return errors.New("profile is required")
	}
	cost, ok := HeroCrownsCost(heroID)
	if !ok {
		return fmt.Errorf("hero %q cannot be purchased", heroID)
	}
	if containsHero(p.UnlockedHeroes, heroID) {
		return fmt.Errorf("hero %q is already unlocked", heroID)
	}
	if p.Crowns < cost {
		return fmt.Errorf("need %d Crowns; you have %d", cost, p.Crowns)
	}
	p.Crowns -= cost
	p.UnlockedHeroes = append(p.UnlockedHeroes, heroID)
	return nil
}

func (p *Profile) TotalPendingBoxes() int {
	if p == nil {
		return 0
	}
	total := 0
	for _, boxType := range [...]string{"wooden", "silver", "gold", "diamond"} {
		if count := p.PendingBoxes[boxType]; count > 0 {
			total += count
		}
	}
	return total
}

func validBoxType(boxType string) bool {
	switch boxType {
	case "wooden", "silver", "gold", "diamond":
		return true
	default:
		return false
	}
}
