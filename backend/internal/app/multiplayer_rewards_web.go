package app

import (
	"fmt"
	"time"

	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
)

func (s *webServer) awardLocalMultiplayerPayout(score int) {
	session := s.session
	if session == nil || session.phase != "gameover" || session.mode == "hvh" || s.onlineViewer || s.playerProfile == nil {
		return
	}
	won, draw := session.winner == 0, session.winner < 0
	payout, err := awardWebMultiplayerPayout(
		s.playerProfile, session.variant, score, won, draw, won, session.mode,
	)
	if err != nil {
		session.profileError = "Match reward calculation failed: " + err.Error()
		return
	}
	session.payout = &payout
	p := s.playerProfile
	if won {
		p.TotalWins++
		p.CurrentStreak++
		if p.CurrentStreak > p.BestStreak {
			p.BestStreak = p.CurrentStreak
		}
		p.WinsSinceCampDraw++
		if p.WinsSinceCampDraw >= lootbox.RequiredWins(p) {
			if p.DrawsAvailable < lootbox.MaxDrawsStored {
				p.DrawsAvailable++
			}
			p.WinsSinceCampDraw = 0
		}
	} else if draw {
		p.TotalDraws++
		p.CurrentStreak = 0
	} else {
		p.TotalLosses++
		p.CurrentStreak = 0
	}
	p.TotalGames++
	p.RecordMatch(profile.MatchRecord{
		At: time.Now().UTC(), Mode: economy.ModeID(multiplayerEconomyMode(session.mode)),
		PlayerScore: payout.PlayerScore, Won: won, MarksEarned: payout.TotalMarks,
	})
	if err := p.Save(); err != nil {
		session.profileError = fmt.Sprintf("Profile save failed: %v", err)
	}
}

func multiplayerEconomyMode(mode string) economy.Mode {
	switch mode {
	case "easy":
		return economy.ModeCPUEasy
	case "normal":
		return economy.ModeCPUNormal
	case "hard":
		return economy.ModeCPUHard
	default:
		return economy.ModeFriendCasual
	}
}
