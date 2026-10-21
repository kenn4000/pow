package game

// MatchGame is a completed game summary relative to Player 1. It is separate
// from GameState so progression systems can inspect results without owning the
// game loop or changing rule resolution.
type MatchGame struct {
	Won             bool   `json:"won"`
	PlayerSeat      Seat   `json:"player_seat"`
	PlayerScore     int    `json:"player_score"`
	OpponentScore   int    `json:"opponent_score"`
	PlayerJail      []Card `json:"player_jail"`
	OpponentJail    []Card `json:"opponent_jail"`
	BountySuit      Suit   `json:"bounty_suit"`
	Clashes         int    `json:"clashes"`
	ScoreAfterR2    int    `json:"score_after_r2"`
	OpponentAfterR2 int    `json:"opponent_after_r2"`
	SuitDoublingWin bool   `json:"suit_doubling_win"`
	RoundSweeps     int    `json:"round_sweeps"`
}

// MatchResult summarizes one completed match. Scores are cumulative for
// best-of-three matches, while Games retains each game's result and seat.
type MatchResult struct {
	Won           bool        `json:"won"`
	Tied          bool        `json:"tied"`
	PlayerSeat    Seat        `json:"player_seat"`
	PlayerScore   int         `json:"player_score"`
	OpponentScore int         `json:"opponent_score"`
	Games         []MatchGame `json:"games"`
}
