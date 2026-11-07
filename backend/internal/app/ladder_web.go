package app

import (
	"errors"
	"fmt"
	"time"

	"prisonersofwar/internal/game"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/ladder"
)

type webLadderState struct {
	SharedStats              ladder.SharedStats `json:"sharedStats"`
	WildFaceRaw              int                `json:"wildFaceRaw"`
	BountyCard               game.Card          `json:"bountyCard"`
	InitiativeSuit           game.Suit          `json:"initiativeSuit"`
	FallbackNoInitiativeSuit bool               `json:"fallbackNoInitiativeSuit"`
	PlayedCards              []game.Card        `json:"playedCards"`
	ChainCard                *game.Card         `json:"chainCard,omitempty"`
	ChainValue               *int               `json:"chainValue,omitempty"`
	ChainSuit                *game.Suit         `json:"chainSuit,omitempty"`
	ChainIsWild              bool               `json:"chainIsWild"`
	CurrentPlayer            int                `json:"currentPlayer"`
	OpeningPending           bool               `json:"openingPending"`
	OpeningChoice            bool               `json:"openingChoice"`
	Hand                     []webCard          `json:"hand"`
	HandCount                int                `json:"handCount"`
	OpponentHandCount        int                `json:"opponentHandCount"`
	CanPass                  bool               `json:"canPass"`
	FinalHands               [][]game.Card      `json:"finalHands,omitempty"`
}

func (s *webServer) startWebLadder(mode, playerHeroID, opponentHeroID string, playerHero, opponentHero heroes.Hero) error {
	deck := game.NewDeck()
	game.Shuffle(deck, s.rng)
	match, err := ladder.NewLadderGameWithDeferredOpening(playerHero, opponentHero, deck, 0)
	if err != nil {
		return fmt.Errorf("start Ladder game: %w", err)
	}
	state := match.State()
	actor := int(state.Current)
	phase := "ladder-play"
	message := "Choose a legal card to play."
	var readyAt time.Time
	if mode == "hvh" {
		phase = "handoff"
		message = fmt.Sprintf("Pass the screen to Player %d for their private hand.", actor+1)
	} else if state.OpeningPending && isAI(mode, actor) {
		phase = "ladder-thinking"
		message = "The CPU is thinking."
		readyAt = time.Now().Add(time.Second)
	} else if state.OpeningPending {
		phase = "ladder-opening-reveal"
		message = "Take a moment to review your full hand. The opening card will be played automatically."
		readyAt = time.Now().Add(3 * time.Second)
	}
	s.session = &webSession{
		mode: mode, variant: "ladder", ladder: match, phase: phase,
		actor: actor, handoffTarget: actor, handoffPhase: "ladder-opening-reveal", winner: -1,
		ladderReadyAt: readyAt,
		playerHeroID:  playerHeroID, opponentHeroID: opponentHeroID, message: message,
		initialAchievements: achievementLevels(s.playerProfile),
	}
	return nil
}

func ladderWebView(session *webSession, viewer int, online bool) *webLadderState {
	if session.ladder == nil {
		return nil
	}
	state := session.ladder.State()
	currentPlayer := session.actor
	if online {
		currentPlayer = viewer
	}
	current := ladder.PlayerID(currentPlayer)
	hand := state.HandA
	opponentHand := state.HandB
	if current == ladder.PlayerB {
		hand, opponentHand = state.HandB, state.HandA
	}
	view := &webLadderState{
		SharedStats: state.SharedStats, WildFaceRaw: state.WildFaceRaw,
		BountyCard: state.BountyCard, InitiativeSuit: state.InitiativeSuit,
		FallbackNoInitiativeSuit: state.FallbackNoInitiativeSuit,
		PlayedCards:              state.PlayedCards,
		ChainCard:                state.ChainCard, ChainValue: state.ChainValue, ChainSuit: state.ChainSuit,
		ChainIsWild: state.ChainIsWild, CurrentPlayer: int(state.Current),
		OpeningPending: state.OpeningPending,
		OpeningChoice:  state.OpeningPending && state.OpeningCard == nil,
		Hand:           []webCard{}, HandCount: len(hand), OpponentHandCount: len(opponentHand),
	}
	if session.phase == "gameover" {
		view.FinalHands = [][]game.Card{
			append([]game.Card{}, state.HandA...),
			append([]game.Card{}, state.HandB...),
		}
		return view
	}
	humanTurn := session.phase == "ladder-play" && !isAI(session.mode, session.actor) &&
		(!online || viewer == session.actor)
	openingReveal := session.phase == "ladder-opening-reveal" &&
		!isAI(session.mode, session.actor) && (!online || viewer == session.actor)
	openingChoice := session.phase == "ladder-opening-choice" &&
		!isAI(session.mode, session.actor) && (!online || viewer == session.actor)
	legal := session.ladder.LegalPlays(current)
	legalCards := make(map[game.Card]struct{}, len(legal))
	for _, card := range legal {
		legalCards[card] = struct{}{}
	}
	view.CanPass = humanTurn && len(legal) == 0
	if session.phase != "handoff" && (humanTurn || openingReveal || openingChoice || online && session.phase == "ladder-play") {
		view.Hand = make([]webCard, len(hand))
		for index, card := range hand {
			_, selectable := legalCards[card]
			view.Hand[index] = webCard{
				Card: card, Index: index, Selectable: openingChoice || selectable && humanTurn,
			}
		}
	}
	return view
}

func (s *webServer) applyLadderAction(request actionRequest) error {
	session := s.session
	if session.ladder == nil || isAI(session.mode, session.actor) {
		return errors.New("it is not your Ladder turn")
	}
	player := ladder.PlayerID(session.actor)
	if session.phase == "ladder-opening-choice" {
		if request.Action != "ladder-open" {
			return errors.New("choose a card to open the new Ladder")
		}
		state := session.ladder.State()
		hand := state.HandA
		if player == ladder.PlayerB {
			hand = state.HandB
		}
		if request.Index < 0 || request.Index >= len(hand) {
			return errors.New("choose a card from your hand")
		}
		if err := session.ladder.OpenStack(player, hand[request.Index]); err != nil {
			return err
		}
		s.finishLadderAction(player)
		return nil
	}
	if session.phase != "ladder-play" {
		return errors.New("it is not your Ladder turn")
	}
	switch request.Action {
	case "ladder-play":
		state := session.ladder.State()
		hand := state.HandA
		if player == ladder.PlayerB {
			hand = state.HandB
		}
		if request.Index < 0 || request.Index >= len(hand) {
			return errors.New("choose a card from your hand")
		}
		if err := session.ladder.Play(player, hand[request.Index]); err != nil {
			return err
		}
	case "ladder-pass":
		if err := session.ladder.PassTurn(player); err != nil {
			return err
		}
	default:
		return errors.New("unknown Ladder action")
	}
	s.finishLadderAction(player)
	return nil
}

func (s *webServer) finishLadderAction(previous ladder.PlayerID) {
	session := s.session
	session.ladderReadyAt = time.Time{}
	state := session.ladder.State()
	if state.Winner != nil {
		session.phase = "gameover"
		session.winner = int(*state.Winner)
		session.actor = int(*state.Winner)
		session.message = fmt.Sprintf("Player %d wins the Ladder game.", session.winner+1)
		if session.mode != "hvh" {
			if session.winner == 0 {
				session.message = "You win the Ladder game."
			} else {
				session.message = "The opponent wins the Ladder game."
			}
		}
		s.awardLocalMultiplayerPayout(0)
		return
	}
	session.actor = int(state.Current)
	if state.OpeningPending && state.OpeningCard == nil {
		opener := ladder.PlayerA
		if previous == ladder.PlayerA {
			opener = ladder.PlayerB
		}
		if session.mode == "hvh" {
			session.phase = "handoff"
			session.handoffTarget = int(state.Current)
			session.handoffPhase = "ladder-opening-choice"
		} else if isAI(session.mode, session.actor) {
			session.phase = "ladder-thinking"
		} else {
			session.phase = "ladder-opening-choice"
		}
		if session.mode != "hvh" && opener == ladder.PlayerA {
			session.message = fmt.Sprintf("%s passed. Choose a card to open the next ladder.",
				s.ladderPlayerName(previous))
		} else {
			session.message = fmt.Sprintf("%s passed. %s will choose a card to open the next ladder.",
				s.ladderPlayerName(previous), s.ladderPlayerName(opener))
		}
		return
	}
	session.phase = "ladder-play"
	session.handoffPhase = ""
	if session.mode == "hvh" && state.Current != previous {
		session.phase = "handoff"
		session.handoffTarget = int(state.Current)
		session.handoffPhase = "ladder-play"
		session.message = fmt.Sprintf("Pass the screen to Player %d for their private hand.", session.actor+1)
		return
	}
	if session.mode == "hvh" {
		session.message = fmt.Sprintf("Player %d, choose a legal card to play.", session.actor+1)
	} else if session.actor == 0 {
		session.message = "Your turn. Choose a legal card to play."
	} else {
		session.message = "The CPU is choosing a card."
	}
}

func (s *webServer) ladderPlayerName(player ladder.PlayerID) string {
	if s.session.mode == "hvh" {
		return fmt.Sprintf("Player %d", int(player)+1)
	}
	return s.playerName(int(player))
}

func (s *webServer) advanceLadderAI() error {
	session := s.session
	if session == nil || session.variant != "ladder" || session.ladder == nil {
		return nil
	}
	if session.phase == "gameover" {
		return nil
	}
	state := session.ladder.State()
	if state.OpeningPending {
		if session.phase == "handoff" {
			return nil
		}
		if state.OpeningCard != nil {
			if session.ladderReadyAt.IsZero() {
				return errors.New("Ladder opening card has no scheduled play time")
			}
			if time.Now().Before(session.ladderReadyAt) {
				return nil
			}
			player := state.OpeningPlayer
			if err := session.ladder.PlayOpeningCard(); err != nil {
				return fmt.Errorf("play Ladder opening card: %w", err)
			}
			s.finishLadderAction(player)
			return s.advanceLadderAI()
		}
		if !isAI(session.mode, session.actor) {
			return nil
		}
		if session.ladderReadyAt.IsZero() {
			session.phase = "ladder-thinking"
			session.ladderReadyAt = time.Now().Add(time.Second)
			if session.message == "" {
				session.message = "The CPU is choosing a card to open the new ladder."
			}
			return nil
		}
		if time.Now().Before(session.ladderReadyAt) {
			return nil
		}
		player := ladder.PlayerID(session.actor)
		card := s.chooseLadderCPUOpening(state)
		if err := session.ladder.OpenStack(player, card); err != nil {
			return fmt.Errorf("open Ladder stack: %w", err)
		}
		s.finishLadderAction(player)
		return s.advanceLadderAI()
	}
	if !isAI(session.mode, session.actor) {
		return nil
	}
	if session.ladderReadyAt.IsZero() {
		session.phase = "ladder-thinking"
		session.ladderReadyAt = time.Now().Add(time.Second)
		session.message = "The CPU is thinking."
		return nil
	}
	if time.Now().Before(session.ladderReadyAt) {
		return nil
	}
	session.ladderReadyAt = time.Time{}
	player := ladder.PlayerID(session.actor)
	legal := session.ladder.LegalPlays(player)
	passed := len(legal) == 0
	if passed {
		if err := session.ladder.PassTurn(player); err != nil {
			return fmt.Errorf("Ladder CPU pass: %w", err)
		}
	} else {
		card := s.chooseLadderCPUPlay(legal)
		if err := session.ladder.Play(player, card); err != nil {
			return fmt.Errorf("Ladder CPU play: %w", err)
		}
	}
	s.finishLadderAction(player)
	return s.advanceLadderAI()
}

func (s *webServer) chooseLadderCPUOpening(state ladder.LadderState) game.Card {
	hand := state.HandB
	opponentHand := state.HandA
	if state.Current == ladder.PlayerA {
		hand, opponentHand = state.HandA, state.HandB
	}
	if s.session.mode == "easy" {
		return hand[s.rng.Intn(len(hand))]
	}
	best := hand[0]
	bestCount := len(opponentHand) + 1
	bestWild := true
	for _, candidate := range hand {
		wild := candidate.Face == state.WildFaceRaw
		count := ladderOpponentLegalCount(candidate, state, opponentHand)
		if count < bestCount ||
			count == bestCount && bestWild && !wild ||
			count == bestCount && bestWild == wild && candidate.Face < best.Face {
			best, bestCount, bestWild = candidate, count, wild
		}
	}
	return best
}

func (s *webServer) chooseLadderCPUPlay(legal []game.Card) game.Card {
	if s.session.mode == "easy" {
		return legal[s.rng.Intn(len(legal))]
	}
	if s.session.mode != "hard" {
		return ladder.GreedyPolicy().ChoosePlay(s.session.ladder, legal)
	}
	state := s.session.ladder.State()
	opponentHand := state.HandA
	if state.Current == ladder.PlayerA {
		opponentHand = state.HandB
	}
	best := legal[0]
	bestCount := len(opponentHand) + 1
	bestWild := true
	for _, candidate := range legal {
		wild := candidate.Face == state.WildFaceRaw
		count := ladderOpponentLegalCount(candidate, state, opponentHand)
		if count < bestCount ||
			count == bestCount && bestWild && !wild ||
			count == bestCount && bestWild == wild && candidate.Face < best.Face {
			best, bestCount, bestWild = candidate, count, wild
		}
	}
	return best
}

func ladderOpponentLegalCount(candidate game.Card, state ladder.LadderState, hand []game.Card) int {
	count := 0
	if candidate.Face == state.WildFaceRaw {
		for _, card := range hand {
			if card.Face == state.WildFaceRaw && card.Suit.Beats(candidate.Suit) {
				count++
			}
		}
		return count
	}
	value := candidate.Face + state.SharedShift
	chainSuit := candidate.Suit
	for _, card := range hand {
		if card.Face == state.WildFaceRaw ||
			ladder.Effective(card, &chainSuit, state.SharedShift, state.SharedFavor) >= value {
			count++
		}
	}
	return count
}
