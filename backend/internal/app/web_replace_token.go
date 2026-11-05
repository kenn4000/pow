package app

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/game"
	"prisonersofwar/internal/ladder"
	"prisonersofwar/internal/profile"
)

type webReplaceOption struct {
	Source string `json:"source"`
	Index  int    `json:"index"`
	Label  string `json:"label"`
}

func replacementDeck(excluded []game.Card, rng *rand.Rand) []game.Card {
	used := make(map[game.Card]struct{}, len(excluded))
	for _, card := range excluded {
		used[card] = struct{}{}
	}
	deck := make([]game.Card, 0, len(game.NewDeck()))
	for _, card := range game.NewDeck() {
		if _, exists := used[card]; !exists {
			deck = append(deck, card)
		}
	}
	if rng != nil {
		rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	}
	return deck
}

func drawRandomCard(pile *[]game.Card, rng *rand.Rand) (game.Card, error) {
	if pile == nil || len(*pile) == 0 {
		return game.Card{}, errors.New("there are no cards left to draw")
	}
	if rng == nil {
		return game.Card{}, errors.New("random source is unavailable")
	}
	index := rng.Intn(len(*pile))
	card := (*pile)[index]
	*pile = append((*pile)[:index], (*pile)[index+1:]...)
	return card, nil
}

func (s *webServer) replaceCardOptions() []webReplaceOption {
	if !canUseWebMatchTokens(s.session, s.onlineViewer) || s.session.phase == "gameover" {
		return nil
	}
	session := s.session
	options := make([]webReplaceOption, 0)
	add := func(source, label string, index int) {
		options = append(options, webReplaceOption{Source: source, Index: index, Label: label})
	}
	addVisible := func(source, name string, cards []game.Card) {
		for index, card := range cards {
			add(source, fmt.Sprintf("%s %d · %s", name, index+1, card), index)
		}
	}

	switch session.variant {
	case "", "main", "fog":
		if len(session.state.Deck) == 0 || isAI(session.mode, 0) {
			return nil
		}
		switch session.phase {
		case "commit":
			if session.actor == 0 {
				addVisible("hand", "Hand card", session.state.P1.Hand)
			}
		case "attack":
			if session.attacker == 0 {
				addVisible("pool", "Pool card", session.state.P1.Pool)
			}
		case "defend":
			if session.actor == 0 {
				addVisible("pool", "Pool card", session.state.P1.Pool)
			}
		}
	case "puzzler":
		puzzle := session.puzzle
		if puzzle == nil || len(puzzle.DrawPile) == 0 || session.phase != "puzzle-pick" ||
			puzzle.complete() || isAI(session.mode, 0) {
			return nil
		}
		for index, card := range puzzle.Definition.Cards {
			if !puzzle.Used[index] && puzzle.Selected != index {
				add("puzzle", fmt.Sprintf("Puzzle card %d · %s", index+1, card), index)
			}
		}
	case "abandon":
		state := session.abandon
		if state == nil || len(state.DrawPile) == 0 || state.Finished || isAI(session.mode, 0) ||
			(session.phase != "abandon-reveal" && session.phase != "abandon-pair-first" &&
				session.phase != "abandon-pair-second") {
			return nil
		}
		for index := range state.Board {
			if !state.Used[index] && !(state.SelectedSource == abandonBoardSource && state.SelectedIndex == index) {
				label := fmt.Sprintf("Board card %d", index+1)
				if state.Revealed[index] {
					label += " · " + state.Board[index].String()
				} else {
					label += " · face down"
				}
				add(abandonBoardSource, label, index)
			}
		}
		for index := range state.PersonalPile {
			if !state.PileUsed[index] && !(state.SelectedSource == abandonPileSource && state.SelectedIndex == index) {
				label := fmt.Sprintf("Personal pile card %d", index+1)
				if state.PileRevealed[index] {
					label += " · " + state.PersonalPile[index].String()
				} else {
					label += " · face down"
				}
				add(abandonPileSource, label, index)
			}
		}
	case "abandon-duel":
		duel := session.abandonDuel
		if duel == nil || len(duel.DrawPile) == 0 || duel.GameOver || duel.ActivePlayer != 0 ||
			session.actor != 0 || isAI(session.mode, 0) ||
			(session.phase != "abandon-duel-reveal" && session.phase != "abandon-duel-pair-first" &&
				session.phase != "abandon-duel-pair-second") {
			return nil
		}
		for index := range duel.Board {
			if !duel.BoardUsed[index] && !(duel.SelectedSource == abandonBoardSource && duel.SelectedIndex == index) {
				label := fmt.Sprintf("Shared board card %d", index+1)
				if duel.BoardRevealed[index] {
					label += " · " + duel.Board[index].String()
				} else {
					label += " · face down"
				}
				add(abandonBoardSource, label, index)
			}
		}
		pile := duel.Players[0]
		for index := range pile.PersonalPile {
			if !pile.PileUsed[index] && !(duel.SelectedSource == abandonPileSource && duel.SelectedIndex == index) {
				label := fmt.Sprintf("Personal pile card %d", index+1)
				if pile.PileRevealed[index] {
					label += " · " + pile.PersonalPile[index].String()
				} else {
					label += " · face down"
				}
				add(abandonPileSource, label, index)
			}
		}
	case "rummy":
		rummy := session.rummy
		if rummy == nil || len(rummy.DrawPile) == 0 || session.actor != 0 || isAI(session.mode, 0) ||
			(session.phase != "rummy-opening" && session.phase != "rummy-turn" &&
				session.phase != "rummy-discard") {
			return nil
		}
		addVisible("hand", "Hand card", rummy.Hands[0])
	case "ladder":
		if session.ladder == nil || session.phase != "ladder-play" || session.actor != 0 ||
			isAI(session.mode, 0) {
			return nil
		}
		state := session.ladder.State()
		if state.Current == ladder.PlayerA && len(state.Deck) > 0 {
			addVisible("hand", "Hand card", state.HandA)
		}
	case "poker":
		if session.poker == nil || len(session.poker.DrawPile) == 0 ||
			session.phase != "poker-play" || isAI(session.mode, 0) {
			return nil
		}
		addVisible("hand", "Hand card", session.poker.Hand)
	case "poker-duel":
		duel := session.pokerDuel
		if duel == nil || duel.GameOver || len(duel.Players[0].DrawPile) == 0 ||
			session.phase != "poker-duel-play" || duel.ActivePlayer != 0 ||
			session.actor != 0 || isAI(session.mode, 0) {
			return nil
		}
		addVisible("hand", "Hand card", duel.Players[0].Hand)
	case "tug-war":
		match := session.tugWar
		if match == nil || match.State.Over || len(match.State.Deck) == 0 ||
			session.phase != "tug-war-play" || session.actor != 0 ||
			match.State.Current != 0 || isAI(session.mode, 0) {
			return nil
		}
		addVisible("hand", "Hand card", match.State.Hands[0])
	}
	return options
}

func (s *webServer) useReplaceCardToken(source string, index int) error {
	session := s.session
	if !canUseWebMatchTokens(session, s.onlineViewer) {
		return errors.New("Replace Any Card tokens can only be used in campaigns and vs-CPU games")
	}
	p := s.playerProfile
	if p == nil || p.ReplaceCardTokens < 1 {
		return errors.New("you do not have a Replace Any Card token")
	}
	var selected *webReplaceOption
	options := s.replaceCardOptions()
	for optionIndex := range options {
		option := options[optionIndex]
		if option.Source == source && option.Index == index {
			selected = &option
			break
		}
	}
	if selected == nil {
		return errors.New("that card cannot be replaced right now")
	}
	previous, err := cloneWebSession(session)
	if err != nil {
		return fmt.Errorf("save state before card replacement: %w", err)
	}
	previous.restartSnapshot, previous.undoSnapshot = session.restartSnapshot, session.undoSnapshot
	if err := s.applyCardReplacement(source, index); err != nil {
		return err
	}
	session = s.session
	session.message = "Replaced a card with a random draw."
	p.ReplaceCardTokens--
	if err := p.Save(); err != nil {
		p.ReplaceCardTokens++
		s.session = previous
		return fmt.Errorf("save Replace Any Card token use: %w", err)
	}
	return nil
}

func (s *webServer) applyCardReplacement(source string, index int) error {
	session := s.session
	switch session.variant {
	case "", "main", "fog":
		var cards []game.Card
		switch source {
		case "hand":
			cards = session.state.P1.Hand
		case "pool":
			cards = session.state.P1.Pool
		default:
			return errors.New("invalid card location")
		}
		if index < 0 || index >= len(cards) {
			return errors.New("card is no longer available")
		}
		drawn, err := drawRandomCard(&session.state.Deck, s.rng)
		if err != nil {
			return err
		}
		discard := cards[index]
		cards[index] = drawn
		session.state.Discard = append(session.state.Discard, discard)
		if source == "hand" {
			session.state.P1.Hand = cards
		} else {
			session.state.P1.Pool = cards
		}
	case "puzzler":
		puzzle := session.puzzle
		if source != "puzzle" || index < 0 || index >= len(puzzle.Definition.Cards) ||
			puzzle.Used[index] || puzzle.Selected == index {
			return errors.New("that puzzle card is no longer available")
		}
		drawn, err := drawRandomCard(&puzzle.DrawPile, s.rng)
		if err != nil {
			return err
		}
		puzzle.Discard = append(puzzle.Discard, puzzle.Definition.Cards[index])
		puzzle.Definition.Cards[index] = drawn
	case "abandon":
		state := session.abandon
		var cards []game.Card
		var revealed *[]bool
		switch source {
		case abandonBoardSource:
			cards, revealed = state.Board, &state.Revealed
		case abandonPileSource:
			cards, revealed = state.PersonalPile, &state.PileRevealed
		default:
			return errors.New("invalid card location")
		}
		if index < 0 || index >= len(cards) || state.Finished {
			return errors.New("that Abandon card is no longer available")
		}
		drawn, err := drawRandomCard(&state.DrawPile, s.rng)
		if err != nil {
			return err
		}
		state.Discard = append(state.Discard, cards[index])
		cards[index] = drawn
		(*revealed)[index] = false
		if source == abandonBoardSource {
			state.Board = cards
		} else {
			state.PersonalPile = cards
		}
	case "abandon-duel":
		duel := session.abandonDuel
		if duel == nil || duel.GameOver {
			return errors.New("Abandon Duel is complete")
		}
		switch source {
		case abandonBoardSource:
			if index < 0 || index >= len(duel.Board) || duel.BoardUsed[index] {
				return errors.New("that board card is no longer available")
			}
			drawn, err := drawRandomCard(&duel.DrawPile, s.rng)
			if err != nil {
				return err
			}
			duel.Discard = append(duel.Discard, duel.Board[index])
			duel.Board[index] = drawn
			duel.BoardRevealed[index] = false
		case abandonPileSource:
			if index < 0 || index >= len(duel.Players[0].PersonalPile) ||
				duel.Players[0].PileUsed[index] {
				return errors.New("that personal pile card is no longer available")
			}
			drawn, err := drawRandomCard(&duel.DrawPile, s.rng)
			if err != nil {
				return err
			}
			duel.Discard = append(duel.Discard, duel.Players[0].PersonalPile[index])
			for player := range duel.Players {
				duel.Players[player].PersonalPile[index] = drawn
				duel.Players[player].PileRevealed[index] = false
			}
		default:
			return errors.New("invalid card location")
		}
	case "rummy":
		rummy := session.rummy
		if source != "hand" || index < 0 || index >= len(rummy.Hands[0]) {
			return errors.New("that Rummy card is no longer available")
		}
		drawn, err := drawRandomCard(&rummy.DrawPile, s.rng)
		if err != nil {
			return err
		}
		old := rummy.Hands[0][index]
		rummy.Hands[0][index] = drawn
		rummy.FaceUp = append(rummy.FaceUp, old)
	case "ladder":
		if source != "hand" || session.ladder == nil {
			return errors.New("invalid Ladder card location")
		}
		return session.ladder.ReplaceCard(ladder.PlayerA, index, s.rng)
	case "poker":
		poker := session.poker
		if source != "hand" || index < 0 || index >= len(poker.Hand) {
			return errors.New("that Poker card is no longer available")
		}
		drawn, err := drawRandomCard(&poker.DrawPile, s.rng)
		if err != nil {
			return err
		}
		old := poker.Hand[index]
		poker.Hand[index] = drawn
		poker.DiscardPile = append(poker.DiscardPile, old)
	case "poker-duel":
		duel := session.pokerDuel
		if duel == nil || duel.GameOver || duel.ActivePlayer != 0 ||
			session.phase != "poker-duel-play" {
			return errors.New("Poker Duel is not accepting a replacement")
		}
		if err := duel.validateAction(time.Now()); err != nil {
			return err
		}
		player := &duel.Players[0]
		if source != "hand" || index < 0 || index >= len(player.Hand) {
			return errors.New("that Poker Duel card is no longer available")
		}
		drawn, err := drawRandomCard(&player.DrawPile, s.rng)
		if err != nil {
			return err
		}
		old := player.Hand[index]
		player.Hand[index] = drawn
		player.DiscardPile = append(player.DiscardPile, old)
	case "tug-war":
		match := session.tugWar
		if match == nil || match.State.Over || match.State.Current != 0 ||
			session.phase != "tug-war-play" {
			return errors.New("Tug War is not accepting a replacement")
		}
		if source != "hand" || index < 0 || index >= len(match.State.Hands[0]) {
			return errors.New("that Tug War card is no longer available")
		}
		drawn, err := drawRandomCard(&match.State.Deck, s.rng)
		if err != nil {
			return err
		}
		old := match.State.Hands[0][index]
		match.State.Hands[0][index] = drawn
		match.State.Discard = append(match.State.Discard, old)
	default:
		return errors.New("this game does not support card replacement")
	}
	return nil
}

func (s *webServer) quickBuyToken(id string) error {
	if !canUseWebMatchTokens(s.session, s.onlineViewer) {
		return errors.New("quick token purchases are only available in campaigns and vs-CPU games")
	}
	switch id {
	case "restart-one", "restart-six", "undo-one", "undo-ten", "replace-card-one", "replace-card-ten":
	case "campaign-skip-one":
		if s.session.campaign == nil || s.session.campaign.Replay {
			return errors.New("Campaign Skip tokens can only be purchased during a campaign mission")
		}
	default:
		return errors.New("this token cannot be purchased from the in-game bar")
	}
	p := s.playerProfile
	if p == nil {
		return errors.New("profile is unavailable")
	}
	before := *p
	message, err := buyShopConsumable(p, id, "crowns")
	if err != nil {
		return err
	}
	if err := p.Save(); err != nil {
		*p = before
		return fmt.Errorf("save token purchase: %w", err)
	}
	s.session.message = message
	return nil
}

func (s *webServer) useCampaignSkipToken() error {
	session := s.session
	p := s.playerProfile
	if !canUseWebMatchTokens(session, s.onlineViewer) || session.campaign == nil ||
		session.campaign.Replay || p == nil ||
		missionIsComplete(p.Campaigns[session.campaign.ID], session.campaign.Index) {
		return errors.New("Campaign Skip tokens can only be used on an uncompleted campaign mission")
	}
	if p.CampaignSkipTokens < 1 {
		return errors.New("you do not have a Campaign Skip token")
	}
	previous, err := cloneWebSession(session)
	if err != nil {
		return fmt.Errorf("save mission state before skipping: %w", err)
	}
	previous.restartSnapshot, previous.undoSnapshot = session.restartSnapshot, session.undoSnapshot
	campaignID, missionIndex := session.campaign.ID, session.campaign.Index
	oldTokens := p.CampaignSkipTokens
	oldProgress, hadProgress := p.Campaigns[campaignID]
	var progressCopy profile.CampaignProgress
	if hadProgress {
		progressCopy = *oldProgress
		progressCopy.CompletedMissionIDs = append([]int(nil), oldProgress.CompletedMissionIDs...)
	}
	message, err := skipCampaignMission(p, campaignID, missionIndex)
	if err != nil {
		return err
	}
	if err := p.Save(); err != nil {
		p.CampaignSkipTokens = oldTokens
		if hadProgress {
			*oldProgress = progressCopy
		} else {
			delete(p.Campaigns, campaignID)
		}
		s.session = previous
		return fmt.Errorf("save Campaign Skip token use: %w", err)
	}
	session.phase, session.winner = "gameover", -1
	session.campaign.Result = message
	session.campaign.HasNextMission = campaigns.CanPlayMission(p, campaignID, missionIndex+1)
	session.message = message
	return nil
}
