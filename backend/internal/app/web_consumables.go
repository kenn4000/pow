package app

import (
	"encoding/json"
	"errors"
	"fmt"

	"prisonersofwar/internal/campaigns"
	"prisonersofwar/internal/ladder"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
	"prisonersofwar/internal/skills"
)

func shopConsumableOffers(p *profile.Profile) []webShopConsumableOffer {
	if p == nil {
		return nil
	}
	discount := skills.AggregateBonuses(p).ConsumableDiscountPercent
	return []webShopConsumableOffer{
		{ID: "camp-draw-one", Name: "Camp Draw Game", Quantity: 1, Available: p.DrawsAvailable, MarksCost: discountedMarks(1500, discount), CrownsCost: 75, CanBuy: p.DrawsAvailable < lootbox.MaxDrawsStored},
		{ID: "camp-draw-three", Name: "Camp Draw Games", Quantity: 3, Available: p.DrawsAvailable, MarksCost: discountedMarks(4000, discount), CrownsCost: 200, CanBuy: p.DrawsAvailable+3 <= lootbox.MaxDrawsStored},
		{ID: "restart-one", Name: "Restart Mission Token", Quantity: 1, Available: p.RestartMissionTokens, MarksCost: discountedMarks(400, discount), CrownsCost: 20, CanBuy: true},
		{ID: "restart-six", Name: "Restart Mission Tokens", Quantity: 6, Available: p.RestartMissionTokens, MarksCost: discountedMarks(2000, discount), CrownsCost: 100, CanBuy: true},
		{ID: "undo-one", Name: "Undo Last Move Token", Quantity: 1, Available: p.UndoMoveTokens, MarksCost: discountedMarks(250, discount), CrownsCost: 12, CanBuy: true},
		{ID: "undo-ten", Name: "Undo Last Move Tokens", Quantity: 10, Available: p.UndoMoveTokens, MarksCost: discountedMarks(2000, discount), CrownsCost: 100, CanBuy: true},
		{ID: "replace-card-one", Name: "Replace Any Card Token", Quantity: 1, Available: p.ReplaceCardTokens, MarksCost: discountedMarks(180, discount), CrownsCost: 9, CanBuy: true},
		{ID: "replace-card-ten", Name: "Replace Any Card Tokens", Quantity: 10, Available: p.ReplaceCardTokens, MarksCost: discountedMarks(1500, discount), CrownsCost: 75, CanBuy: true},
		{ID: "campaign-skip-one", Name: "Skip a Campaign Mission", Quantity: 1, Available: p.CampaignSkipTokens, MarksCost: discountedMarks(1500, discount), CrownsCost: 75, CanBuy: true},
	}
}

func discountedMarks(cost int64, discountPercent int) int64 {
	return cost * int64(100-discountPercent) / 100
}

func shopChestMarksCost(p *profile.Profile, cost int64) int64 {
	return discountedMarks(cost, skills.AggregateBonuses(p).ChestDiscountPercent)
}

func buyShopConsumable(p *profile.Profile, id, payment string) (string, error) {
	if p == nil {
		return "", errors.New("profile is required")
	}
	var offer webShopConsumableOffer
	for _, candidate := range shopConsumableOffers(p) {
		if candidate.ID == id {
			offer = candidate
			break
		}
	}
	if offer.ID == "" {
		return "", fmt.Errorf("unknown consumable %q", id)
	}
	if !offer.CanBuy {
		return "", errors.New("Camp Draw storage is full")
	}
	cost := offer.MarksCost
	if payment == "crowns" {
		cost = offer.CrownsCost
	} else if payment != "marks" {
		return "", fmt.Errorf("unsupported payment %q", payment)
	}
	if payment == "marks" && p.Marks < cost {
		return "", fmt.Errorf("need %d Marks; you have %d", cost, p.Marks)
	}
	if payment == "crowns" && p.Crowns < cost {
		return "", fmt.Errorf("need %d Crowns; you have %d", cost, p.Crowns)
	}
	switch id {
	case "camp-draw-one", "camp-draw-three":
		p.DrawsAvailable += offer.Quantity
	case "restart-one", "restart-six":
		p.RestartMissionTokens += offer.Quantity
	case "undo-one", "undo-ten":
		p.UndoMoveTokens += offer.Quantity
	case "replace-card-one", "replace-card-ten":
		p.ReplaceCardTokens += offer.Quantity
	case "campaign-skip-one":
		p.CampaignSkipTokens += offer.Quantity
	}
	if payment == "marks" {
		p.Marks -= cost
	} else {
		p.Crowns -= cost
	}
	currency := "Marks"
	if payment == "crowns" {
		currency = "Crowns"
	}
	return fmt.Sprintf("Purchased %d %s for %d %s.", offer.Quantity, offer.Name, cost, currency), nil
}

func skipCampaignMission(p *profile.Profile, campaignID string, missionIndex int) (string, error) {
	if p == nil {
		return "", errors.New("profile is required")
	}
	if p.CampaignSkipTokens < 1 {
		return "", errors.New("you do not have a Campaign Mission Skip token")
	}
	if !campaigns.CanPlayMission(p, campaignID, missionIndex) {
		return "", errors.New("complete the previous mission and unlock this campaign first")
	}
	campaign, ok := campaigns.ByID(campaignID)
	if !ok {
		return "", fmt.Errorf("unknown campaign %q", campaignID)
	}
	_, ok = findCampaignMission(campaign, missionIndex)
	if !ok {
		return "", fmt.Errorf("mission %d does not exist in %s", missionIndex, campaign.Name)
	}
	if missionIsComplete(p.Campaigns[campaignID], missionIndex) {
		return "", errors.New("completed missions cannot be skipped")
	}
	outcome, err := campaigns.SkipMission(p, campaignID, missionIndex)
	if err != nil {
		return "", err
	}
	if outcome.AlreadyCompleted {
		return "", errors.New("completed missions cannot be skipped")
	}
	p.CampaignSkipTokens--
	message := fmt.Sprintf("Skipped %s · Mission %d. No mission rewards were granted.", campaign.Name, missionIndex)
	if outcome.CampaignCompleted {
		message += " Campaign complete; completion rewards were not granted."
	}
	return message, nil
}

func canUseWebMatchTokens(session *webSession, onlineViewer bool) bool {
	if session == nil || onlineViewer {
		return false
	}
	switch session.mode {
	case "easy", "normal", "hard":
		return true
	default:
		return false
	}
}

func isAutomaticWebAction(action string) bool {
	return action == "draft-ai-pick" || action == "poker-duel-cpu-turn"
}

func isConsumableWebAction(action string) bool {
	switch action {
	case "use-restart-token", "use-undo-token", "use-replace-token",
		"use-campaign-skip-token", "quick-buy-token":
		return true
	default:
		return false
	}
}

func (s *webServer) captureRestartSnapshot() error {
	if !canUseWebMatchTokens(s.session, s.onlineViewer) {
		return nil
	}
	snapshot, err := cloneWebSession(s.session)
	if err != nil {
		return fmt.Errorf("capture initial game state: %w", err)
	}
	s.session.restartSnapshot = snapshot
	s.session.undoSnapshot = nil
	return nil
}

func (s *webServer) useRestartMissionToken() error {
	session := s.session
	if !canUseWebMatchTokens(session, s.onlineViewer) || session.phase == "gameover" {
		return errors.New("Restart Mission tokens can only be used during a campaign or vs-CPU game")
	}
	if session.restartSnapshot == nil {
		return errors.New("the initial game state is unavailable")
	}
	p := s.playerProfile
	if p == nil || p.RestartMissionTokens < 1 {
		return errors.New("you do not have a Restart Mission token")
	}
	restarted, err := cloneWebSession(session.restartSnapshot)
	if err != nil {
		return fmt.Errorf("restore initial game state: %w", err)
	}
	restarted.restartSnapshot = session.restartSnapshot
	restarted.message = "Mission restarted."
	if restarted.campaign == nil {
		restarted.message = "Match restarted."
	}
	p.RestartMissionTokens--
	if err := p.Save(); err != nil {
		p.RestartMissionTokens++
		return fmt.Errorf("save Restart Mission token use: %w", err)
	}
	s.session = restarted
	return nil
}

func (s *webServer) useUndoMoveToken() error {
	session := s.session
	if !canUseWebMatchTokens(session, s.onlineViewer) || session.phase == "gameover" {
		return errors.New("Undo Last Move tokens can only be used during a campaign or vs-CPU game")
	}
	if session.undoSnapshot == nil {
		return errors.New("there is no previous move to undo")
	}
	p := s.playerProfile
	if p == nil || p.UndoMoveTokens < 1 {
		return errors.New("you do not have an Undo Last Move token")
	}
	restored, err := cloneWebSession(session.undoSnapshot)
	if err != nil {
		return fmt.Errorf("restore previous game state: %w", err)
	}
	restored.restartSnapshot = session.restartSnapshot
	restored.message = "Last move undone."
	p.UndoMoveTokens--
	if err := p.Save(); err != nil {
		p.UndoMoveTokens++
		return fmt.Errorf("save Undo Last Move token use: %w", err)
	}
	s.session = restored
	return nil
}

func cloneWebSession(source *webSession) (*webSession, error) {
	if source == nil {
		return nil, errors.New("game state is unavailable")
	}
	cloned := *source
	var err error
	if cloned.state, err = cloneWebValue(source.state); err != nil {
		return nil, err
	}
	if cloned.puzzle, err = cloneWebPointer(source.puzzle); err != nil {
		return nil, err
	}
	if cloned.abandon, err = cloneWebPointer(source.abandon); err != nil {
		return nil, err
	}
	if cloned.abandonDuel, err = cloneWebPointer(source.abandonDuel); err != nil {
		return nil, err
	}
	if cloned.rummy, err = cloneWebPointer(source.rummy); err != nil {
		return nil, err
	}
	if source.ladder != nil {
		state, cloneErr := cloneWebValue(source.ladder.State())
		if cloneErr != nil {
			return nil, cloneErr
		}
		cloned.ladder = ladder.Restore(state)
	}
	if cloned.poker, err = cloneWebPointer(source.poker); err != nil {
		return nil, err
	}
	if cloned.pokerDuel, err = cloneWebPointer(source.pokerDuel); err != nil {
		return nil, err
	}
	if cloned.tugWar, err = cloneWebPointer(source.tugWar); err != nil {
		return nil, err
	}
	if cloned.slots, err = cloneWebValue(source.slots); err != nil {
		return nil, err
	}
	if cloned.campaign, err = cloneWebPointer(source.campaign); err != nil {
		return nil, err
	}
	if cloned.payout, err = cloneWebPointer(source.payout); err != nil {
		return nil, err
	}
	if cloned.battles, err = cloneWebValue(source.battles); err != nil {
		return nil, err
	}
	if cloned.initialAchievements, err = cloneWebValue(source.initialAchievements); err != nil {
		return nil, err
	}
	if cloned.achievementUpdates, err = cloneWebValue(source.achievementUpdates); err != nil {
		return nil, err
	}
	if cloned.rankedSeries, err = cloneWebPointer(source.rankedSeries); err != nil {
		return nil, err
	}
	cloned.restartSnapshot = nil
	cloned.undoSnapshot = nil
	return &cloned, nil
}

func cloneWebPointer[T any](source *T) (*T, error) {
	if source == nil {
		return nil, nil
	}
	return cloneWebValue(source)
}

func cloneWebValue[T any](source T) (T, error) {
	var cloned T
	data, err := json.Marshal(source)
	if err != nil {
		return cloned, fmt.Errorf("encode game snapshot: %w", err)
	}
	if err := json.Unmarshal(data, &cloned); err != nil {
		return cloned, fmt.Errorf("decode game snapshot: %w", err)
	}
	return cloned, nil
}
