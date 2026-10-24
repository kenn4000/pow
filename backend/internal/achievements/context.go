package achievements

import (
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

type EventKind int

const (
	EventMatchEnd EventKind = iota
	EventCampDrawOpened
	EventPurchase
	EventProfileLoad
	EventLogin
	EventHeroAcquired
	EventCampaignCompleted
)

type Context struct {
	Profile             *profile.Profile
	Event               EventKind
	Now                 time.Time
	Result              *game.MatchResult
	Mode                int
	PlayerSeat          string
	PlayerPrisoners     int
	OppPrisoners        int
	PlayerPrisonerSuits map[string]int
	OppPrisonerSuits    map[string]int
	BountyCount         int
	MaxFace             int
	Clashes             int
	ScoreAfterR2        int
	OppScoreAfterR2     int
	RoundSweeps         int
	MatchWon            bool
	MatchTied           bool
	IsBestOfThree       bool
	UnlockedHeroCount   int
	AcquisitionSource   string
}

func NewMatchContext(p *profile.Profile, result game.MatchResult, mode int, seat string) Context {
	ctx := Context{
		Profile:             p,
		Event:               EventMatchEnd,
		Now:                 time.Now(),
		Result:              &result,
		Mode:                mode,
		PlayerSeat:          seat,
		PlayerPrisonerSuits: make(map[string]int),
		OppPrisonerSuits:    make(map[string]int),
		MatchWon:            result.Won,
		MatchTied:           result.Tied || result.PlayerScore == result.OpponentScore,
		IsBestOfThree:       len(result.Games) > 1,
	}
	if p != nil {
		ctx.UnlockedHeroCount = len(p.UnlockedHeroes)
	}
	for _, matchGame := range result.Games {
		ctx.PlayerPrisoners += len(matchGame.PlayerJail)
		ctx.OppPrisoners += len(matchGame.OpponentJail)
		ctx.Clashes += matchGame.Clashes
		ctx.RoundSweeps += matchGame.RoundSweeps
		for _, card := range matchGame.PlayerJail {
			role := SuitRoleID(card.Suit)
			ctx.PlayerPrisonerSuits[role]++
			if card.Face > ctx.MaxFace {
				ctx.MaxFace = card.Face
			}
			if card.Suit == matchGame.BountySuit {
				ctx.BountyCount++
			}
		}
		for _, card := range matchGame.OpponentJail {
			ctx.OppPrisonerSuits[SuitRoleID(card.Suit)]++
		}
		ctx.ScoreAfterR2 = matchGame.ScoreAfterR2
		ctx.OppScoreAfterR2 = matchGame.OpponentAfterR2
	}
	return ctx
}

func SuitRoleID(suit game.Suit) string {
	switch suit {
	case game.Blade:
		return "B"
	case game.Fang:
		return "F"
	case game.Shield:
		return "S"
	default:
		return ""
	}
}

func SuitRoleName(suit game.Suit) string {
	switch suit {
	case game.Blade:
		return "Scissors"
	case game.Fang:
		return "Paper"
	case game.Shield:
		return "Rock"
	default:
		return "Unknown"
	}
}
