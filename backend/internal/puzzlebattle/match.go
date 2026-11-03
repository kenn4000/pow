package puzzlebattle

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/modifiers"
)

const (
	PhaseWaiting        = "waiting"
	PhaseCountdown      = "countdown"
	PhaseRoundOne       = "round1"
	PhaseRoundTwo       = "round2"
	PhaseComplete       = "complete"
	RoundStatusPending  = "pending"
	RoundStatusActive   = "active"
	RoundStatusComplete = "complete"
	RoundStatusSkipped  = "skipped"
)

const (
	ReadyPromptDelay = 20 * time.Second
	ReadyTimeout     = 45 * time.Second
	CountdownLength  = 3 * time.Second
	MatchLength      = 120 * time.Second
	HeartbeatTimeout = 30 * time.Second
	TieTolerance     = 100 * time.Millisecond
)

var (
	ErrInvalidPlayer = errors.New("player index must be 0 or 1")
	ErrMatchComplete = errors.New("match is complete")
)

type Pair struct {
	FirstIndex  int `json:"firstIndex"`
	SecondIndex int `json:"secondIndex"`
}

type PlayerRound struct {
	Pairs           []Pair      `json:"pairs"`
	PendingIndex    int         `json:"pendingIndex"`
	Submitted       bool        `json:"submitted"`
	AutoSubmitted   bool        `json:"autoSubmitted"`
	SubmittedAt     time.Time   `json:"submittedAt,omitempty"`
	Score           int         `json:"score"`
	Jail            []game.Card `json:"jail"`
	CapturedByRound [3][3]int   `json:"capturedByRound"`
}

type Round struct {
	Number      int            `json:"number"`
	Status      string         `json:"status"`
	Controller  int            `json:"controller"`
	Board       []game.Card    `json:"board"`
	Players     [2]PlayerRound `json:"players"`
	StartedAt   time.Time      `json:"startedAt,omitempty"`
	CompletedAt time.Time      `json:"completedAt,omitempty"`
}

type PlayerRoundResult struct {
	Pairs         []Pair       `json:"pairs"`
	PairResults   []PairResult `json:"pairResults"`
	Submitted     bool         `json:"submitted"`
	AutoSubmitted bool         `json:"autoSubmitted"`
	SubmittedAt   time.Time    `json:"submittedAt,omitempty"`
	Score         int          `json:"score"`
	Jail          []game.Card  `json:"jail"`
}

type PairResult struct {
	FirstIndex    int        `json:"firstIndex"`
	SecondIndex   int        `json:"secondIndex"`
	AttackerIndex int        `json:"attackerIndex"`
	DefenderIndex int        `json:"defenderIndex"`
	AttackerValue int        `json:"attackerValue"`
	DefenderValue int        `json:"defenderValue"`
	WinnerPlayer  int        `json:"winnerPlayer"`
	CapturedCard  *game.Card `json:"capturedCard,omitempty"`
}

type RoundResult struct {
	Number     int                  `json:"number"`
	Controller int                  `json:"controller"`
	Board      []game.Card          `json:"board"`
	Players    [2]PlayerRoundResult `json:"players"`
}

type Result struct {
	Winner      int           `json:"winner"`
	Draw        bool          `json:"draw"`
	Reason      string        `json:"reason"`
	Scores      [2]int        `json:"scores"`
	Rounds      []RoundResult `json:"rounds"`
	CompletedAt time.Time     `json:"completedAt"`
}

// Match is the persisted domain state. RNG output is materialized at creation,
// so restoring this value does not depend on serializing a random generator.
type Match struct {
	ID              string             `json:"id"`
	PlayerIDs       [2]string          `json:"playerIds"`
	PlayerHeroIDs   [2]string          `json:"playerHeroIds"`
	PlayerDraft     [2]int             `json:"playerDraft"`
	SharedHeroID    string             `json:"sharedHeroId"`
	Modifiers       game.GameModifiers `json:"modifiers"`
	BountySuit      game.Suit          `json:"bountySuit"`
	Controllers     [2]int             `json:"controllers"`
	Rounds          [2]Round           `json:"rounds"`
	Ready           [2]bool            `json:"ready"`
	LastHeartbeatAt [2]time.Time       `json:"lastHeartbeatAt"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
	ReadyPromptAt   time.Time          `json:"readyPromptAt"`
	AutoReadyAt     time.Time          `json:"autoReadyAt"`
	Status          string             `json:"status"`
	CountdownEndsAt *time.Time         `json:"countdownEndsAt,omitempty"`
	Deadline        *time.Time         `json:"deadline,omitempty"`
	Result          *Result            `json:"result,omitempty"`
}

type View struct {
	MatchID           string             `json:"matchId"`
	Status            string             `json:"status"`
	Player            int                `json:"player"`
	PlayerIDs         [2]string          `json:"playerIds"`
	PlayerHeroIDs     [2]string          `json:"playerHeroIds"`
	SharedHeroID      string             `json:"sharedHeroId"`
	Modifiers         game.GameModifiers `json:"modifiers"`
	BountySuit        game.Suit          `json:"bountySuit"`
	Ready             [2]bool            `json:"ready"`
	ReadyPrompt       bool               `json:"readyPrompt"`
	ReadyPromptAt     time.Time          `json:"readyPromptAt"`
	AutoReadyAt       time.Time          `json:"autoReadyAt"`
	CountdownEndsAt   *time.Time         `json:"countdownEndsAt,omitempty"`
	Deadline          *time.Time         `json:"deadline,omitempty"`
	RemainingMillis   int64              `json:"remainingMillis"`
	RoundNumber       int                `json:"roundNumber"`
	RoundStatus       string             `json:"roundStatus"`
	Controller        int                `json:"controller"`
	Board             []game.Card        `json:"board,omitempty"`
	RequiredPairs     int                `json:"requiredPairs"`
	OwnPairs          []Pair             `json:"ownPairs,omitempty"`
	PendingIndex      int                `json:"pendingIndex"`
	OwnSubmitted      bool               `json:"ownSubmitted"`
	OpponentSubmitted bool               `json:"opponentSubmitted"`
	PlayerSubmitted   [2]bool            `json:"playerSubmitted"`
	ReadyStatus       [2]string          `json:"readyStatus"`
	RoundOneScores    *[2]int            `json:"roundOneScores,omitempty"`
	Result            *Result            `json:"result,omitempty"`
}

// New creates a match and materializes all random choices so Match can be
// encoded and restored without keeping the generator alive.
func New(id string, playerIDs [2]string, playerHeroIDs [2]string, playerHeroes [2]heroes.Hero, rng *rand.Rand, now time.Time) (*Match, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("match id is required")
	}
	if rng == nil {
		return nil, errors.New("random source is required")
	}
	if now.IsZero() {
		return nil, errors.New("creation time is required")
	}
	if strings.TrimSpace(playerIDs[0]) == "" || strings.TrimSpace(playerIDs[1]) == "" || playerIDs[0] == playerIDs[1] {
		return nil, errors.New("two distinct player ids are required")
	}

	var draft [2]int
	for i := range playerIDs {
		if strings.TrimSpace(playerHeroIDs[i]) == "" || playerHeroes[i].ID != playerHeroIDs[i] {
			return nil, fmt.Errorf("player %d hero id does not match the selected hero", i)
		}
		hero, ok := heroes.ByID(playerHeroIDs[i])
		if !ok {
			return nil, fmt.Errorf("unknown player hero %q", playerHeroIDs[i])
		}
		if playerHeroes[i].Draft != hero.Draft {
			return nil, fmt.Errorf("player %d hero has an invalid draft modifier", i)
		}
		// Personal heroes affect only draft size; all other personal modifiers
		// are deliberately ignored in Puzzle Battle.
		draft[i] = playerHeroes[i].Draft
	}

	allHeroes := heroes.All()
	sharedHero := allHeroes[rng.Intn(len(allHeroes))]
	controller := rng.Intn(2)
	bounty := game.Suit(rng.Intn(3))
	scoringModifiers := game.GameModifiers{
		Bounty:    clamp(1+sharedHero.Bounty, -3, 3),
		Shift:     sharedHero.Shift,
		Favored:   sharedHero.Favored,
		Reckoning: modifiers.DefaultReckoning,
	}
	if err := scoringModifiers.Validate(); err != nil {
		return nil, fmt.Errorf("shared hero has invalid scoring modifiers: %w", err)
	}

	match := &Match{
		ID:            id,
		PlayerIDs:     playerIDs,
		PlayerHeroIDs: playerHeroIDs,
		PlayerDraft:   draft,
		SharedHeroID:  sharedHero.ID,
		Modifiers:     scoringModifiers,
		BountySuit:    bounty,
		Controllers:   [2]int{controller, 1 - controller},
		CreatedAt:     now,
		UpdatedAt:     now,
		ReadyPromptAt: now.Add(ReadyPromptDelay),
		AutoReadyAt:   now.Add(ReadyTimeout),
		Status:        PhaseWaiting,
	}
	for i := range match.LastHeartbeatAt {
		match.LastHeartbeatAt[i] = now
	}
	for i := range match.Rounds {
		number := i + 1
		boardSize := 7 + draft[match.Controllers[i]]
		if i == 1 {
			boardSize = 9 + draft[match.Controllers[i]]
		}
		if boardSize < 2*(2+i+1) {
			return nil, fmt.Errorf("round %d board size %d cannot support the required pairs", number, boardSize)
		}
		deck := game.NewDeck()
		game.Shuffle(deck, rng)
		match.Rounds[i] = Round{
			Number:     number,
			Status:     RoundStatusPending,
			Controller: match.Controllers[i],
			Board:      append([]game.Card(nil), deck[:boardSize]...),
		}
		for player := range match.Rounds[i].Players {
			match.Rounds[i].Players[player].PendingIndex = -1
			match.Rounds[i].Players[player].Pairs = []Pair{}
			match.Rounds[i].Players[player].Jail = []game.Card{}
		}
	}
	return match, nil
}

func (m *Match) SetReady(player int, now time.Time) error {
	if err := validatePlayer(player); err != nil {
		return err
	}
	if err := m.Advance(now); err != nil {
		return err
	}
	if m.Status != PhaseWaiting {
		return errors.New("ready phase is not active")
	}
	m.Ready[player] = true
	if m.Ready[0] && m.Ready[1] {
		m.beginCountdown(now)
	}
	return nil
}

func (m *Match) TogglePair(player, index int, now time.Time) error {
	if err := validatePlayer(player); err != nil {
		return err
	}
	if err := m.Advance(now); err != nil {
		return err
	}
	round, err := m.activeRound()
	if err != nil {
		return err
	}
	if round.Players[player].Submitted {
		return errors.New("player has already submitted this round")
	}
	if index < 0 || index >= len(round.Board) {
		return fmt.Errorf("card index %d is outside the board", index)
	}
	state := &round.Players[player]
	for i, pair := range state.Pairs {
		if pair.FirstIndex == index || pair.SecondIndex == index {
			state.Pairs = append(state.Pairs[:i], state.Pairs[i+1:]...)
			return nil
		}
	}
	if state.PendingIndex == index {
		state.PendingIndex = -1
		return nil
	}
	if state.PendingIndex < 0 {
		state.PendingIndex = index
		return nil
	}
	state.Pairs = append(state.Pairs, Pair{FirstIndex: state.PendingIndex, SecondIndex: index})
	state.PendingIndex = -1
	return nil
}

func (m *Match) Submit(player int, now time.Time) error {
	if err := validatePlayer(player); err != nil {
		return err
	}
	if err := m.Advance(now); err != nil {
		return err
	}
	round, err := m.activeRound()
	if err != nil {
		return err
	}
	state := &round.Players[player]
	if state.Submitted {
		return errors.New("player has already submitted this round")
	}
	required := requiredPairs(round.Number)
	if len(state.Pairs) != required || state.PendingIndex >= 0 {
		return fmt.Errorf("round %d requires exactly %d complete pairs", round.Number, required)
	}
	m.finalizePlayerRound(round, player, now, false)
	m.progressAfterSubmission(round, now)
	m.UpdatedAt = now
	return nil
}

func (m *Match) Heartbeat(player int, now time.Time) error {
	if err := validatePlayer(player); err != nil {
		return err
	}
	if err := m.validateNow(now); err != nil {
		return err
	}
	if m.Status == PhaseComplete {
		return ErrMatchComplete
	}
	last := m.LastHeartbeatAt[player]
	if now.Before(last) {
		return errors.New("heartbeat time cannot move backwards")
	}
	if now.Sub(last) > HeartbeatTimeout {
		if err := m.Advance(now); err != nil {
			return err
		}
		return errors.New("heartbeat arrived after the disconnect timeout")
	}
	m.LastHeartbeatAt[player] = now
	return m.Advance(now)
}

// Advance applies timer-driven transitions and disconnect forfeits.
func (m *Match) Advance(now time.Time) error {
	if err := m.validateNow(now); err != nil {
		return err
	}
	if m.Status == PhaseComplete {
		return nil
	}
	m.UpdatedAt = now
	disconnected := [2]bool{}
	for player := range m.LastHeartbeatAt {
		disconnected[player] = now.Sub(m.LastHeartbeatAt[player]) > HeartbeatTimeout
	}
	if disconnected[0] && disconnected[1] {
		m.finishDoubleForfeit(now)
		return nil
	}
	for player, isDisconnected := range disconnected {
		if isDisconnected {
			m.finishForfeit(1-player, now)
			return nil
		}
	}
	if m.Status == PhaseWaiting && !now.Before(m.AutoReadyAt) {
		m.Ready = [2]bool{true, true}
		m.beginCountdown(m.AutoReadyAt)
	}
	if m.Status == PhaseCountdown && m.CountdownEndsAt != nil && !now.Before(*m.CountdownEndsAt) {
		m.beginMatch(*m.CountdownEndsAt)
	}
	if m.Deadline != nil && !now.Before(*m.Deadline) {
		if m.Status == PhaseRoundOne {
			m.finishRoundOneTimeout(now)
		} else if m.Status == PhaseRoundTwo {
			m.finishRoundTwoTimeout(now)
		}
	}
	return nil
}

func (m *Match) ViewFor(player int, now time.Time) (View, error) {
	if err := validatePlayer(player); err != nil {
		return View{}, err
	}
	if err := m.Advance(now); err != nil {
		return View{}, err
	}
	view := View{
		MatchID:         m.ID,
		Status:          m.Status,
		Player:          player,
		PlayerIDs:       m.PlayerIDs,
		PlayerHeroIDs:   m.PlayerHeroIDs,
		SharedHeroID:    m.SharedHeroID,
		Modifiers:       m.Modifiers,
		BountySuit:      m.BountySuit,
		Ready:           m.Ready,
		ReadyPrompt:     m.Status == PhaseWaiting && !now.Before(m.ReadyPromptAt),
		ReadyPromptAt:   m.ReadyPromptAt,
		AutoReadyAt:     m.AutoReadyAt,
		CountdownEndsAt: copyTime(m.CountdownEndsAt),
		Deadline:        copyTime(m.Deadline),
	}
	for i := range m.Ready {
		view.ReadyStatus[i] = "waiting"
		if m.Ready[i] {
			view.ReadyStatus[i] = "ready"
		}
	}
	if view.Deadline != nil && now.Before(*view.Deadline) {
		view.RemainingMillis = view.Deadline.Sub(now).Milliseconds()
	}
	if m.Status == PhaseRoundOne || m.Status == PhaseRoundTwo {
		roundIndex := 0
		if m.Status == PhaseRoundTwo {
			roundIndex = 1
		}
		round := m.Rounds[roundIndex]
		own := round.Players[player]
		view.RoundNumber = round.Number
		view.RoundStatus = round.Status
		view.Controller = round.Controller
		view.Board = append([]game.Card(nil), round.Board...)
		view.RequiredPairs = requiredPairs(round.Number)
		view.OwnPairs = append([]Pair(nil), own.Pairs...)
		view.PendingIndex = own.PendingIndex
		view.OwnSubmitted = own.Submitted
		view.OpponentSubmitted = round.Players[1-player].Submitted
		view.PlayerSubmitted = [2]bool{round.Players[0].Submitted, round.Players[1].Submitted}
	}
	if m.Status == PhaseRoundTwo || m.Status == PhaseComplete {
		scores := [2]int{m.Rounds[0].Players[0].Score, m.Rounds[0].Players[1].Score}
		view.RoundOneScores = &scores
	}
	if m.Status == PhaseComplete {
		view.Result = cloneResult(m.Result)
	}
	return view, nil
}

func (m *Match) validateNow(now time.Time) error {
	if m == nil {
		return errors.New("match is required")
	}
	if now.IsZero() {
		return errors.New("current time is required")
	}
	if m.CreatedAt.IsZero() || now.Before(m.CreatedAt) {
		return errors.New("current time cannot precede match creation")
	}
	if !m.UpdatedAt.IsZero() && now.Before(m.UpdatedAt) {
		return errors.New("current time cannot move backwards")
	}
	return nil
}

func (m *Match) activeRound() (*Round, error) {
	if m == nil {
		return nil, errors.New("match is required")
	}
	if m.Status == PhaseComplete {
		return nil, ErrMatchComplete
	}
	switch m.Status {
	case PhaseRoundOne:
		return &m.Rounds[0], nil
	case PhaseRoundTwo:
		return &m.Rounds[1], nil
	default:
		return nil, errors.New("round is not active")
	}
}

func (m *Match) beginCountdown(now time.Time) {
	if m.Status != PhaseWaiting {
		return
	}
	end := now.Add(CountdownLength)
	m.CountdownEndsAt = &end
	m.Status = PhaseCountdown
}

func (m *Match) beginMatch(now time.Time) {
	m.Status = PhaseRoundOne
	m.Rounds[0].Status = RoundStatusActive
	m.Rounds[0].StartedAt = now
	deadline := now.Add(MatchLength)
	m.Deadline = &deadline
}

func (m *Match) beginRoundTwo(now time.Time) {
	m.Rounds[0].Status = RoundStatusComplete
	m.Rounds[0].CompletedAt = now
	m.Status = PhaseRoundTwo
	m.Rounds[1].Status = RoundStatusActive
	m.Rounds[1].StartedAt = now
}

func (m *Match) finalizePlayerRound(round *Round, player int, now time.Time, automatic bool) {
	state := &round.Players[player]
	if state.Submitted {
		return
	}
	state.Submitted = true
	state.AutoSubmitted = automatic
	state.SubmittedAt = now
	state.Score, state.Jail, state.CapturedByRound = m.scorePairs(round, player, state.Pairs)
}

func (m *Match) progressAfterSubmission(round *Round, now time.Time) {
	if !round.Players[0].Submitted || !round.Players[1].Submitted {
		return
	}
	if round.Number == 1 {
		m.beginRoundTwo(now)
		return
	}
	round.Status = RoundStatusComplete
	round.CompletedAt = now
	m.finish("completed", now)
}

func (m *Match) finishRoundOneTimeout(now time.Time) {
	round := &m.Rounds[0]
	for player := range round.Players {
		if !round.Players[player].Submitted {
			round.Players[player].Score = 0
			round.Players[player].Jail = []game.Card{}
			round.Players[player].CapturedByRound = [3][3]int{}
		}
	}
	round.Status = RoundStatusSkipped
	round.CompletedAt = now
	m.Rounds[1].Status = RoundStatusSkipped
	m.finish("timeoutRoundOne", now)
}

func (m *Match) finishRoundTwoTimeout(now time.Time) {
	round := &m.Rounds[1]
	for player := range round.Players {
		m.finalizePlayerRound(round, player, now, true)
	}
	round.Status = RoundStatusComplete
	round.CompletedAt = now
	m.finish("timeoutRoundTwo", now)
}

func (m *Match) finishForfeit(winner int, now time.Time) {
	m.finish("forfeit", now)
	if m.Result != nil {
		m.Result.Winner = winner
		m.Result.Draw = false
	}
}

func (m *Match) finishDoubleForfeit(now time.Time) {
	m.finish("doubleForfeit", now)
	if m.Result != nil {
		m.Result.Winner = -1
		m.Result.Draw = true
	}
}

func (m *Match) finish(reason string, now time.Time) {
	scores := [2]int{}
	for roundIndex := range m.Rounds {
		for player := range m.Rounds[roundIndex].Players {
			scores[player] += m.Rounds[roundIndex].Players[player].Score
		}
	}
	winner, draw := chooseWinner(scores, m.Rounds)
	if reason == "forfeit" {
		winner, draw = -1, false
	}
	results := make([]RoundResult, 0, 2)
	for _, round := range m.Rounds {
		if round.Status == RoundStatusPending {
			continue
		}
		roundResult := RoundResult{
			Number:     round.Number,
			Controller: round.Controller,
			Board:      append([]game.Card(nil), round.Board...),
		}
		for player, state := range round.Players {
			pairResults := m.resolvePairs(&round, player, state.Pairs)
			roundResult.Players[player] = PlayerRoundResult{
				Pairs:         append([]Pair(nil), state.Pairs...),
				PairResults:   pairResults,
				Submitted:     state.Submitted,
				AutoSubmitted: state.AutoSubmitted,
				SubmittedAt:   state.SubmittedAt,
				Score:         state.Score,
				Jail:          append([]game.Card(nil), state.Jail...),
			}
		}
		results = append(results, roundResult)
	}
	m.Status = PhaseComplete
	m.Result = &Result{
		Winner:      winner,
		Draw:        draw,
		Reason:      reason,
		Scores:      scores,
		Rounds:      results,
		CompletedAt: now,
	}
}

func (m *Match) scorePairs(round *Round, player int, pairs []Pair) (int, []game.Card, [3][3]int) {
	var jail []game.Card
	var captured [3][3]int
	for _, result := range m.resolvePairs(round, player, pairs) {
		if result.CapturedCard == nil || result.WinnerPlayer != player {
			continue
		}
		card := *result.CapturedCard
		jail = append(jail, card)
		captured[0][int(card.Suit)]++
	}
	breakdown := game.ScoreWithModifiers(jail, captured, m.BountySuit, m.Modifiers)
	return breakdown.Total, jail, captured
}

func (m *Match) resolvePairs(round *Round, player int, pairs []Pair) []PairResult {
	results := make([]PairResult, 0, len(pairs))
	for _, pair := range pairs {
		attackerIndex, defenderIndex := pair.FirstIndex, pair.SecondIndex
		if player != round.Controller {
			attackerIndex, defenderIndex = pair.SecondIndex, pair.FirstIndex
		}
		if attackerIndex < 0 || defenderIndex < 0 || attackerIndex >= len(round.Board) || defenderIndex >= len(round.Board) {
			continue
		}
		outcome := game.ResolveWithModifiers(round.Board[attackerIndex], round.Board[defenderIndex], m.Modifiers)
		winnerPlayer := -1
		var captured *game.Card
		switch outcome.Winner {
		case 0:
			winnerPlayer = round.Controller
			card := outcome.Captured
			captured = &card
		case 1:
			winnerPlayer = 1 - round.Controller
			card := outcome.Captured
			captured = &card
		}
		results = append(results, PairResult{
			FirstIndex: pair.FirstIndex, SecondIndex: pair.SecondIndex,
			AttackerIndex: attackerIndex, DefenderIndex: defenderIndex,
			AttackerValue: outcome.AttackerValue, DefenderValue: outcome.DefenderValue,
			WinnerPlayer: winnerPlayer, CapturedCard: captured,
		})
	}
	return results
}

func chooseWinner(scores [2]int, rounds [2]Round) (int, bool) {
	if scores[0] > scores[1] {
		return 0, false
	}
	if scores[1] > scores[0] {
		return 1, false
	}
	first, second := rounds[1].Players[0].SubmittedAt, rounds[1].Players[1].SubmittedAt
	if first.IsZero() || second.IsZero() {
		return -1, true
	}
	delta := first.Sub(second)
	if delta < 0 {
		delta = -delta
	}
	if delta <= TieTolerance {
		return -1, true
	}
	if first.Before(second) {
		return 0, false
	}
	return 1, false
}

func requiredPairs(round int) int {
	if round == 1 {
		return 3
	}
	return 4
}

func validatePlayer(player int) error {
	if player < 0 || player > 1 {
		return ErrInvalidPlayer
	}
	return nil
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func cloneResult(result *Result) *Result {
	if result == nil {
		return nil
	}
	clone := *result
	clone.Rounds = make([]RoundResult, len(result.Rounds))
	for i, round := range result.Rounds {
		clone.Rounds[i] = round
		clone.Rounds[i].Board = append([]game.Card(nil), round.Board...)
		for player := range round.Players {
			state := round.Players[player]
			clone.Rounds[i].Players[player] = state
			clone.Rounds[i].Players[player].Pairs = append([]Pair(nil), state.Pairs...)
			clone.Rounds[i].Players[player].PairResults = append([]PairResult(nil), state.PairResults...)
			clone.Rounds[i].Players[player].Jail = append([]game.Card(nil), state.Jail...)
		}
	}
	return &clone
}
