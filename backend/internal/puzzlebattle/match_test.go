package puzzlebattle

import (
	"encoding/json"
	"errors"
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
)

func testMatch(t *testing.T, seed int64, now time.Time) *Match {
	t.Helper()
	first, _ := heroes.ByID("recruit")
	second, _ := heroes.ByID("anchor")
	match, err := New("battle-1", [2]string{"alice", "bob"}, [2]string{"recruit", "anchor"}, [2]heroes.Hero{first, second}, rand.New(rand.NewSource(seed)), now)
	if err != nil {
		t.Fatal(err)
	}
	return match
}

func startMatch(t *testing.T, match *Match, now time.Time) time.Time {
	t.Helper()
	if err := match.SetReady(0, now); err != nil {
		t.Fatal(err)
	}
	if err := match.SetReady(1, now); err != nil {
		t.Fatal(err)
	}
	start := now.Add(CountdownLength)
	if err := match.Advance(start); err != nil {
		t.Fatal(err)
	}
	return start
}

func sendHeartbeats(t *testing.T, match *Match, now time.Time) {
	t.Helper()
	for player := 0; player < 2; player++ {
		if err := match.Heartbeat(player, now); err != nil {
			t.Fatal(err)
		}
	}
}

func keepAliveUntil(t *testing.T, match *Match, start, end time.Time) {
	t.Helper()
	for beat := start.Add(25 * time.Second); beat.Before(end); beat = beat.Add(25 * time.Second) {
		sendHeartbeats(t, match, beat)
	}
}

func TestNewAndReadinessCountdown(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	match := testMatch(t, 11, now)
	if match.Status != PhaseWaiting || match.Controllers[0] == match.Controllers[1] {
		t.Fatalf("unexpected initial state: status=%q controllers=%v", match.Status, match.Controllers)
	}
	if len(match.Rounds[0].Board) != 7+match.PlayerDraft[match.Controllers[0]] {
		t.Fatalf("round one board size = %d, want 7 + controller draft", len(match.Rounds[0].Board))
	}
	if len(match.Rounds[1].Board) != 9+match.PlayerDraft[match.Controllers[1]] {
		t.Fatalf("round two board size = %d, want 9 + other draft", len(match.Rounds[1].Board))
	}
	if match.Modifiers.Reckoning != [3]int{1, 1, 1} {
		t.Fatalf("unexpected reckoning modifiers: %v", match.Modifiers)
	}

	if err := match.SetReady(0, now); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseWaiting {
		t.Fatalf("one ready player started match: %s", match.Status)
	}
	view, err := match.ViewFor(0, now.Add(ReadyPromptDelay))
	if err != nil {
		t.Fatal(err)
	}
	if !view.ReadyPrompt || view.Status != PhaseWaiting {
		t.Fatalf("ready prompt not shown at 20s: %+v", view)
	}
	if err := match.SetReady(1, now.Add(ReadyPromptDelay)); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseCountdown || match.CountdownEndsAt == nil || !match.CountdownEndsAt.Equal(now.Add(ReadyPromptDelay+CountdownLength)) {
		t.Fatalf("unexpected countdown state: %+v", match)
	}
	if err := match.Advance(*match.CountdownEndsAt); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseRoundOne || match.Deadline == nil || !match.Deadline.Equal(now.Add(ReadyPromptDelay+CountdownLength+MatchLength)) {
		t.Fatalf("match did not start with the expected global clock: %+v", match)
	}
	if match.Rounds[1].StartedAt != (time.Time{}) {
		t.Fatal("round two started early")
	}
}

func TestStartImmediatelyStartsRoundOne(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	match := testMatch(t, 13, now)
	if err := match.StartImmediately(now); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseRoundOne || match.Rounds[0].Status != RoundStatusActive ||
		!match.Rounds[0].StartedAt.Equal(now) || match.Deadline == nil {
		t.Fatalf("immediate start did not initialize round one: %+v", match)
	}
	if !match.Ready[0] || !match.Ready[1] {
		t.Fatalf("immediate start did not ready both players: %v", match.Ready)
	}
	if err := match.StartImmediately(now); err == nil {
		t.Fatal("immediate start accepted a match that had already started")
	}
}

func TestTogglePairSubmitAndPrivateView(t *testing.T) {
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	match := testMatch(t, 17, now)
	start := startMatch(t, match, now)
	round := &match.Rounds[0]
	if err := match.TogglePair(0, 0, start); err != nil {
		t.Fatal(err)
	}
	if err := match.TogglePair(0, 1, start); err != nil {
		t.Fatal(err)
	}
	if err := match.TogglePair(0, 1, start); err != nil {
		t.Fatal(err)
	}
	if len(round.Players[0].Pairs) != 0 || round.Players[0].PendingIndex != -1 {
		t.Fatal("clicking a paired card should unpair both cards")
	}
	if err := match.TogglePair(0, 0, start); err != nil {
		t.Fatal(err)
	}
	if err := match.TogglePair(0, 2, start); err != nil {
		t.Fatal(err)
	}
	if err := match.TogglePair(0, 2, start); err != nil {
		t.Fatal(err)
	}
	if round.Players[0].PendingIndex != -1 || len(round.Players[0].Pairs) != 0 {
		t.Fatal("clicking a pending card should cancel its selection")
	}

	playerPairs := [2][]Pair{
		{{FirstIndex: 0, SecondIndex: 1}, {FirstIndex: 2, SecondIndex: 3}, {FirstIndex: 4, SecondIndex: 5}},
		{{FirstIndex: 0, SecondIndex: 2}, {FirstIndex: 1, SecondIndex: 3}, {FirstIndex: 4, SecondIndex: 5}},
	}
	for player := 0; player < 2; player++ {
		for _, pair := range playerPairs[player] {
			a, b := pair.FirstIndex, pair.SecondIndex
			if err := match.TogglePair(player, a, start); err != nil {
				t.Fatal(err)
			}
			if err := match.TogglePair(player, b, start); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := match.Submit(0, start); err != nil {
		t.Fatal(err)
	}
	view, err := match.ViewFor(1, start)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.OwnPairs) != 3 || view.OpponentSubmitted != true {
		t.Fatalf("own state or opponent submitted status missing: %+v", view)
	}
	if view.RoundOneScores != nil || view.Result != nil {
		t.Fatalf("round-one score leaked before round two: %+v", view)
	}
	if view.RoundOneJails[0] != nil || view.RoundOneJails[1] != nil {
		t.Fatalf("round-one jail leaked before both players submitted: %+v", view.RoundOneJails)
	}
	if len(view.OwnPairs) != len(round.Players[1].Pairs) || view.OwnPairs[0] != round.Players[1].Pairs[0] {
		t.Fatal("view does not contain only the requesting player's pairs")
	}
	if err := match.Submit(1, start); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseRoundTwo {
		t.Fatalf("round two did not start after both submissions: %s", match.Status)
	}
	view, err = match.ViewFor(0, start)
	if err != nil {
		t.Fatal(err)
	}
	if view.RoundOneScores == nil {
		t.Fatal("round-one scores should be revealed at the start of round two")
	}
	for player := range view.RoundOneJails {
		if len(view.RoundOneJails[player]) != len(round.Players[player].Jail) {
			t.Fatalf("round-one jail %d has %d cards, want %d", player,
				len(view.RoundOneJails[player]), len(round.Players[player].Jail))
		}
		for index, card := range view.RoundOneJails[player] {
			if card != round.Players[player].Jail[index] {
				t.Fatalf("round-one jail %d card %d = %v, want %v", player, index, card, round.Players[player].Jail[index])
			}
		}
	}
	if view.RoundNumber != 2 || view.RequiredPairs != 4 || view.Deadline == nil {
		t.Fatalf("round two view is incomplete: %+v", view)
	}
	if view.RemainingMillis != int64(MatchLength/time.Millisecond) {
		t.Fatalf("round two should use the existing global clock, got %dms", view.RemainingMillis)
	}
}

func TestRoundTwoJailScoringUsesRoundTwoReckoning(t *testing.T) {
	match := testMatch(t, 29, time.Now())
	match.Modifiers.Reckoning = [3]int{1, 1, 5}
	round := &Round{
		Number: 2, Controller: 0,
		Board: []game.Card{
			{Suit: game.Shield, Face: 9},
			{Suit: game.Shield, Face: 1},
		},
	}
	score, jail, captured := match.scorePairs(round, 0, []Pair{{FirstIndex: 0, SecondIndex: 1}})
	if len(jail) != 1 || captured[1][int(game.Shield)] != 1 || captured[0][int(game.Shield)] != 0 {
		t.Fatalf("round-two capture tally = %v / %v, want one Shield captured in round two", jail, captured)
	}
	want := game.ScoreWithModifiers(jail, captured, match.BountySuit, match.Modifiers).Total
	if score != want {
		t.Fatalf("round-two score = %d, want %d", score, want)
	}
}

func TestSubmitRequiresAllPairsAndInputValidation(t *testing.T) {
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	match := testMatch(t, 19, now)
	start := startMatch(t, match, now)
	if err := match.Submit(0, start); err == nil {
		t.Fatal("submit accepted fewer than the required pairs")
	}
	if err := match.TogglePair(2, 0, start); !errors.Is(err, ErrInvalidPlayer) {
		t.Fatalf("invalid player error = %v", err)
	}
	if err := match.TogglePair(0, len(match.Rounds[0].Board), start); err == nil {
		t.Fatal("toggle accepted an out-of-range board index")
	}
	if _, err := match.ViewFor(-1, start); !errors.Is(err, ErrInvalidPlayer) {
		t.Fatalf("view invalid player error = %v", err)
	}
}

func TestSubmitPairLocksEachPairAndSubmitsOnTheLastPair(t *testing.T) {
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	match := testMatch(t, 23, now)
	start := startMatch(t, match, now)
	pairs := [2][]Pair{
		{{FirstIndex: 0, SecondIndex: 1}, {FirstIndex: 2, SecondIndex: 3}, {FirstIndex: 4, SecondIndex: 5}},
		{{FirstIndex: 0, SecondIndex: 2}, {FirstIndex: 1, SecondIndex: 3}, {FirstIndex: 4, SecondIndex: 5}},
	}

	if err := match.SubmitPair(0, 0, 1, start); err != nil {
		t.Fatal(err)
	}
	if err := match.SubmitPair(0, 0, 6, start); err == nil {
		t.Fatal("a card from an already submitted pair was accepted again")
	}
	if match.Rounds[0].Players[0].Submitted {
		t.Fatal("player submitted before completing all required pairs")
	}

	for _, pair := range pairs[0][1:] {
		if err := match.SubmitPair(0, pair.FirstIndex, pair.SecondIndex, start); err != nil {
			t.Fatal(err)
		}
	}
	if !match.Rounds[0].Players[0].Submitted || match.Status != PhaseRoundOne {
		t.Fatalf("last pair did not submit the player's round: status=%q state=%+v", match.Status, match.Rounds[0].Players[0])
	}

	for _, pair := range pairs[1] {
		if err := match.SubmitPair(1, pair.FirstIndex, pair.SecondIndex, start); err != nil {
			t.Fatal(err)
		}
	}
	if match.Status != PhaseRoundTwo {
		t.Fatalf("round two did not start after both players submitted their last pair: %q", match.Status)
	}
}

func TestRoundOneTimeoutSkipsRoundTwo(t *testing.T) {
	now := time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
	match := testMatch(t, 23, now)
	start := startMatch(t, match, now)
	round := &match.Rounds[0]
	for pairIndex := 0; pairIndex < 3; pairIndex++ {
		if err := match.TogglePair(0, pairIndex*2, start); err != nil {
			t.Fatal(err)
		}
		if err := match.TogglePair(0, pairIndex*2+1, start); err != nil {
			t.Fatal(err)
		}
	}
	if err := match.TogglePair(1, 0, start); err != nil {
		t.Fatal(err)
	}
	if err := match.TogglePair(1, 1, start); err != nil {
		t.Fatal(err)
	}
	if err := match.Submit(0, start); err != nil {
		t.Fatal(err)
	}
	deadline := *match.Deadline
	keepAliveUntil(t, match, start, deadline)
	if err := match.Advance(deadline); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseComplete || match.Result.Reason != "timeoutRoundOne" {
		t.Fatalf("round-one timeout was not finalized: %+v", match.Result)
	}
	if match.Rounds[0].Players[0].Score == 0 && len(round.Players[0].Pairs) != 0 {
		t.Fatal("submitted player's current pairs were not scored")
	}
	if match.Rounds[0].Players[1].Score != 0 || match.Rounds[1].Status != RoundStatusSkipped {
		t.Fatalf("missing player should score zero and round two should be skipped: %+v", match.Rounds)
	}
	if len(match.Result.Rounds) != 2 || match.Result.Rounds[0].Players[1].Submitted {
		t.Fatalf("full timeout results are missing: %+v", match.Result)
	}
	if len(match.Result.Rounds[0].Players[1].PairResults) != 1 || match.Rounds[0].Players[1].Score != 0 {
		t.Fatalf("unsubmitted current pairs should be resolved for the result but score zero: %+v", match.Result.Rounds[0].Players[1])
	}
}

func TestRoundTwoTimeoutAutoSubmitsAndTieTolerance(t *testing.T) {
	now := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	match := testMatch(t, 29, now)
	start := startMatch(t, match, now)
	for player := 0; player < 2; player++ {
		for pairIndex := 0; pairIndex < 3; pairIndex++ {
			if err := match.TogglePair(player, pairIndex*2, start); err != nil {
				t.Fatal(err)
			}
			if err := match.TogglePair(player, pairIndex*2+1, start); err != nil {
				t.Fatal(err)
			}
		}
		if err := match.Submit(player, start); err != nil {
			t.Fatal(err)
		}
	}
	secondStart := start
	for player := 0; player < 2; player++ {
		for pairIndex := 0; pairIndex < 4; pairIndex++ {
			if err := match.TogglePair(player, pairIndex*2, secondStart); err != nil {
				t.Fatal(err)
			}
			if err := match.TogglePair(player, pairIndex*2+1, secondStart); err != nil {
				t.Fatal(err)
			}
		}
	}
	// With the same board and pairing order, scores tie; the second submission
	// is within the inclusive 100ms draw window.
	if err := match.Submit(0, secondStart.Add(1*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := match.Submit(1, secondStart.Add(101*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseComplete {
		t.Fatalf("round two did not complete: %s", match.Status)
	}
	if match.Result.Scores[0] != match.Result.Scores[1] {
		t.Fatalf("identical pairings should tie: %v", match.Result.Scores)
	}
	if !match.Result.Draw || match.Result.Winner != -1 {
		t.Fatalf("100ms submit difference should draw: %+v", match.Result)
	}
	view, err := match.ViewFor(0, secondStart.Add(101*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if view.Result == nil || len(view.Result.Rounds) != 2 {
		t.Fatalf("completed view omitted full results: %+v", view)
	}
}

func TestDisconnectForfeitAndHeartbeatReconnect(t *testing.T) {
	now := time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC)
	match := testMatch(t, 31, now)
	if err := match.Advance(now.Add(HeartbeatTimeout)); err != nil {
		t.Fatal(err)
	}
	if match.Status == PhaseComplete {
		t.Fatal("player should not forfeit at exactly 30 seconds")
	}
	if err := match.Heartbeat(1, now.Add(HeartbeatTimeout)); err != nil {
		t.Fatal(err)
	}
	if err := match.Heartbeat(0, now.Add(HeartbeatTimeout+time.Nanosecond)); err == nil {
		t.Fatal("late reconnect should not revive a player after the threshold")
	}
	if match.Status != PhaseComplete || match.Result.Reason != "forfeit" || match.Result.Winner != 1 {
		t.Fatalf("disconnect did not forfeit to the connected player: %+v", match.Result)
	}
}

func TestAutoReadyAndJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 7, 8, 9, 10, 11, 0, time.UTC)
	match := testMatch(t, 37, now)
	// Keep both players connected while readiness times out.
	if err := match.Heartbeat(0, now.Add(HeartbeatTimeout)); err != nil {
		t.Fatal(err)
	}
	if err := match.Heartbeat(1, now.Add(HeartbeatTimeout)); err != nil {
		t.Fatal(err)
	}
	if err := match.Advance(now.Add(ReadyTimeout)); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseCountdown || !match.Ready[0] || !match.Ready[1] {
		t.Fatalf("players were not auto-readied: %+v", match)
	}
	encoded, err := json.Marshal(match)
	if err != nil {
		t.Fatal(err)
	}
	var restored Match
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ID != match.ID || restored.SharedHeroID != match.SharedHeroID || restored.Rounds[0].Board[0] != match.Rounds[0].Board[0] {
		t.Fatalf("JSON round trip lost match state: %+v", restored)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["playerIds"]; !ok {
		t.Fatal("state JSON does not use lowerCamel field names")
	}
}

func TestTimeoutUsesCurrentPairsForRoundTwo(t *testing.T) {
	now := time.Date(2026, 8, 9, 10, 11, 12, 0, time.UTC)
	match := testMatch(t, 41, now)
	start := startMatch(t, match, now)
	for player := 0; player < 2; player++ {
		for pairIndex := 0; pairIndex < 3; pairIndex++ {
			if err := match.TogglePair(player, pairIndex*2, start); err != nil {
				t.Fatal(err)
			}
			if err := match.TogglePair(player, pairIndex*2+1, start); err != nil {
				t.Fatal(err)
			}
		}
		if err := match.Submit(player, start); err != nil {
			t.Fatal(err)
		}
	}
	secondStart := start
	// Timeout accepts current pairs even if the player did not submit.
	if err := match.TogglePair(0, 0, secondStart); err != nil {
		t.Fatal(err)
	}
	if err := match.TogglePair(0, 1, secondStart); err != nil {
		t.Fatal(err)
	}
	secondDeadline := *match.Deadline
	keepAliveUntil(t, match, secondStart, secondDeadline)
	if err := match.Advance(secondDeadline); err != nil {
		t.Fatal(err)
	}
	if match.Status != PhaseComplete || match.Result.Reason != "timeoutRoundTwo" {
		t.Fatalf("round two timeout was not finalized: %+v", match.Result)
	}
	if !match.Rounds[1].Players[0].AutoSubmitted || !match.Rounds[1].Players[1].AutoSubmitted {
		t.Fatalf("round-two timeout should auto-submit both players: %+v", match.Rounds[1].Players)
	}
	if len(match.Rounds[1].Players[0].Pairs) != 1 || len(match.Result.Rounds[1].Players[0].PairResults) != 1 {
		t.Fatalf("round-two timeout should resolve current complete pairs: %+v", match.Rounds[1].Players[0])
	}
	if len(match.Result.Rounds) != 2 || match.Rounds[1].Players[1].Score != 0 {
		t.Fatalf("round-two timeout should retain both round results: %+v", match.Result.Rounds)
	}
}

func TestBountyAndSharedHeroUseIndependentRandomRoster(t *testing.T) {
	now := time.Date(2026, 9, 10, 11, 12, 13, 0, time.UTC)
	match := testMatch(t, 43, now)
	shared, ok := heroes.ByID(match.SharedHeroID)
	if !ok {
		t.Fatalf("shared hero %q is not in the complete roster", match.SharedHeroID)
	}
	wantBounty := 1 + shared.Bounty
	if wantBounty < -3 {
		wantBounty = -3
	}
	if wantBounty > 3 {
		wantBounty = 3
	}
	if match.Modifiers.Bounty != wantBounty || match.Modifiers.Shift != shared.Shift || match.Modifiers.Favored != shared.Favored {
		t.Fatalf("scoring modifiers do not match the shared hero: hero=%+v mods=%+v", shared, match.Modifiers)
	}
	if !match.BountySuit.Valid() {
		t.Fatalf("invalid random bounty suit: %v", match.BountySuit)
	}
	if match.PlayerDraft[0] != -1 || match.PlayerDraft[1] != 1 {
		t.Fatalf("personal heroes should supply their canonical Draft modifiers: %v", match.PlayerDraft)
	}
}

func TestTimelyHeartbeatAndTieBreakBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 13, 14, 0, time.UTC)
	match := testMatch(t, 47, now)
	atThreshold := now.Add(HeartbeatTimeout)
	if err := match.Heartbeat(0, atThreshold); err != nil {
		t.Fatalf("heartbeat at the disconnect boundary should resume: %v", err)
	}
	if err := match.Heartbeat(1, atThreshold); err != nil {
		t.Fatalf("second heartbeat at the boundary should remain active: %v", err)
	}
	if match.Status == PhaseComplete {
		t.Fatal("timely reconnect should not forfeit")
	}

	rounds := [2]Round{}
	rounds[1].Players[0].SubmittedAt = now
	rounds[1].Players[1].SubmittedAt = now.Add(TieTolerance + time.Nanosecond)
	winner, draw := chooseWinner([2]int{12, 12}, rounds)
	if winner != 0 || draw {
		t.Fatalf("submission more than 100ms earlier should win the score tie: winner=%d draw=%v", winner, draw)
	}
}
