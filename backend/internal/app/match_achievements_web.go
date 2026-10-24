package app

import (
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/economy"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/profile"
)

// webMatchModes maps browser match modes to the economy modes used for
// achievement statistics, matching the terminal interface's classification.
var webMatchModes = map[string]economy.Mode{
	"hvh":    economy.ModeFriendCasual,
	"easy":   economy.ModeCPUEasy,
	"normal": economy.ModeCPUNormal,
	"hard":   economy.ModeCPUHard,
}

// webMatchResult describes the finished browser match from Player 1's
// perspective, in the form used by achievement statistics and evaluation.
func (s *webServer) webMatchResult(playerScore, opponentScore int, won, tied bool) game.MatchResult {
	state := s.session.state
	afterR2 := s.scoresAfterRound(2)
	return game.MatchResult{
		Won: won, Tied: tied, PlayerSeat: state.P1.Seat, PlayerScore: playerScore, OpponentScore: opponentScore,
		Games: []game.MatchGame{{
			Won: won, PlayerSeat: state.P1.Seat, PlayerScore: playerScore, OpponentScore: opponentScore,
			PlayerJail: state.P1.Jail, OpponentJail: state.P2.Jail,
			PlayerCapturedByRound: state.P1.CapturedByRound, OpponentCapturedByRound: state.P2.CapturedByRound,
			Modifiers: state.Modifiers, BountySuit: state.Bounty.Suit, Clashes: state.Clashes,
			ScoreAfterR2: afterR2[0], OpponentAfterR2: afterR2[1],
		}},
	}
}

// scoresAfterRound returns the cumulative [Player 1, Player 2] scores recorded
// after the last resolved exchange of the given round, or zeros if none.
func (s *webServer) scoresAfterRound(round int) [2]int {
	scores := [2]int{}
	for _, battle := range s.session.battles {
		if battle.Round <= round {
			scores = battle.Scores
		}
	}
	return scores
}

// creditWebMatchAchievements updates seat, streak, comeback and other
// achievement statistics for a finished browser match and awards any
// achievements it completes. It also counts the match in TotalGames.
func (s *webServer) creditWebMatchAchievements(p *profile.Profile, result game.MatchResult) {
	mode, ok := webMatchModes[s.session.mode]
	if !ok {
		mode = economy.ModeCPUEasy
	}
	updateAchievementStats(p, result, mode)
	ctx := achievements.NewMatchContext(p, result, economy.ModeID(mode), result.PlayerSeat.String())
	ctx.Now = time.Now()
	for _, event := range achievements.EvaluateAll(ctx) {
		applyUnlockEvent(p, event)
	}
}
