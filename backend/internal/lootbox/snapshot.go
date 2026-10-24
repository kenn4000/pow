package lootbox

import "prisonersofwar/internal/game"

type SessionSnapshot struct {
	StartingHand  []game.Card      `json:"startingHand"`
	PlayerPool    []game.Card      `json:"playerPool"`
	CPUPool       []game.Card      `json:"cpuPool"`
	ExchangeIndex int              `json:"exchangeIndex"`
	PlayerWins    int              `json:"playerWins"`
	Exchanges     []ExchangeResult `json:"exchanges"`
	Finished      bool             `json:"finished"`
}

func (s *CampDrawSession) Snapshot() *SessionSnapshot {
	if s == nil {
		return nil
	}
	return &SessionSnapshot{s.startingHand, s.playerPool, s.cpuPool, s.exchangeIndex, s.playerWins, s.exchanges, s.finished}
}

func RestoreSession(s *SessionSnapshot) *CampDrawSession {
	if s == nil {
		return nil
	}
	return &CampDrawSession{s.StartingHand, s.PlayerPool, s.CPUPool, s.ExchangeIndex, s.PlayerWins, s.Exchanges, s.Finished}
}
