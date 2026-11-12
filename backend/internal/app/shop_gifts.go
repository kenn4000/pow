package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"prisonersofwar/internal/achievements"
	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/energy"
	"prisonersofwar/internal/heroes"
	"prisonersofwar/internal/lootbox"
	"prisonersofwar/internal/profile"
)

const (
	giftFriendshipAge = 48 * time.Hour
	giftAccountAge    = 7 * 24 * time.Hour
)

type webGiftRecipient struct {
	Handle            string    `json:"handle"`
	FriendshipCreated time.Time `json:"friendshipCreated"`
	AccountCreated    time.Time `json:"accountCreated"`
	Eligible          bool      `json:"eligible"`
	Reason            string    `json:"reason,omitempty"`
}

func shopGiftEligibility(accountCreated, friendshipCreated, now time.Time) (bool, string) {
	if friendshipCreated.After(now.Add(-giftFriendshipAge)) {
		return false, "Friendship must be at least 48 hours old."
	}
	if accountCreated.After(now.Add(-giftAccountAge)) {
		return false, "Receiving account must be at least 7 days old."
	}
	return true, ""
}

func (s *webServer) shopGiftRecipients(ctx context.Context, userID string) ([]webGiftRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.public_handle,u.created_at,f.created_at
		FROM friendships f
		JOIN users u ON u.id=CASE WHEN f.user_a=$1 THEN f.user_b ELSE f.user_a END
		WHERE f.user_a=$1 OR f.user_b=$1
		ORDER BY lower(u.public_handle)`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	recipients := make([]webGiftRecipient, 0)
	for rows.Next() {
		var item webGiftRecipient
		if err := rows.Scan(&item.Handle, &item.AccountCreated, &item.FriendshipCreated); err != nil {
			return nil, err
		}
		item.Eligible, item.Reason = shopGiftEligibility(item.AccountCreated, item.FriendshipCreated, now)
		recipients = append(recipients, item)
	}
	return recipients, rows.Err()
}

func (s *webServer) handleShopGift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	var request actionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	request.Recipient = strings.TrimSpace(request.Recipient)
	if request.Recipient == "" {
		writeError(w, http.StatusBadRequest, errors.New("choose a friend to receive this gift"))
		return
	}
	if !giftableShopAction(request.Action) {
		writeError(w, http.StatusBadRequest, errors.New("this shop item cannot be gifted"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()

	var recipientID string
	var accountCreated, friendshipCreated time.Time
	err = tx.QueryRowContext(r.Context(), `
		SELECT u.id,u.created_at,f.created_at
		FROM friendships f
		JOIN users u ON u.id=CASE WHEN f.user_a=$1 THEN f.user_b ELSE f.user_a END
		WHERE (f.user_a=$1 OR f.user_b=$1) AND lower(u.public_handle)=lower($2)
		FOR UPDATE OF f,u`, user.ID, request.Recipient).
		Scan(&recipientID, &accountCreated, &friendshipCreated)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusForbidden, errors.New("gifts can only be sent to friends"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	now := time.Now().UTC()
	eligible, reason := shopGiftEligibility(accountCreated, friendshipCreated, now)
	if !eligible {
		writeError(w, http.StatusForbidden, errors.New(strings.TrimSuffix(reason, ".")))
		return
	}

	ids := []string{user.ID, recipientID}
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	profiles := make(map[string]*profile.Profile, 2)
	for _, id := range ids {
		var data []byte
		if err := tx.QueryRowContext(r.Context(),
			"SELECT profile FROM player_data WHERE user_id=$1 FOR UPDATE", id).Scan(&data); err != nil {
			databaseError(w, err)
			return
		}
		p := &profile.Profile{}
		if err := json.Unmarshal(data, p); err != nil {
			databaseError(w, fmt.Errorf("decode gift profile: %w", err))
			return
		}
		profiles[id] = p
	}
	payer, recipient := profiles[user.ID], profiles[recipientID]
	payerAfter, err := cloneProfile(payer)
	if err != nil {
		databaseError(w, err)
		return
	}
	if _, err := s.applyShopAction(payerAfter, request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := grantShopGift(recipient, payer, payerAfter, request, now); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	payer.Marks = payerAfter.Marks
	payer.Crowns = payerAfter.Crowns
	payer.TotalChromasPurchased = payerAfter.TotalChromasPurchased
	payer.RareChromasPurchased = payerAfter.RareChromasPurchased
	payer.SuperRareChromasPurchased = payerAfter.SuperRareChromasPurchased
	evaluateAchievementEvent(payer, achievements.EventPurchase, "shop")
	payer.UpdatedAt = now
	recipient.UpdatedAt = now
	for _, id := range []string{user.ID, recipientID} {
		data, err := json.Marshal(profiles[id])
		if err != nil {
			databaseError(w, fmt.Errorf("encode gift profile: %w", err))
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			UPDATE player_data SET profile=$1,updated_at=$2,version=version+1 WHERE user_id=$3`,
			data, now, id); err != nil {
			databaseError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Gift purchased for " + request.Recipient + ".",
	})
}

func giftableShopAction(action string) bool {
	switch action {
	case "buy-chroma", "buy-chroma-family", "buy-box", "buy-energy",
		"buy-consumable", "buy-skill-points", "buy-hero":
		return true
	default:
		return false
	}
}

func cloneProfile(source *profile.Profile) (*profile.Profile, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	clone := &profile.Profile{}
	if err := json.Unmarshal(data, clone); err != nil {
		return nil, err
	}
	return clone, nil
}

func grantShopGift(recipient, before, after *profile.Profile, request actionRequest, now time.Time) error {
	switch request.Action {
	case "buy-chroma":
		if recipient.OwnedChromas[request.ID] {
			return errors.New("the recipient already owns this chroma")
		}
		if !after.OwnedChromas[request.ID] {
			return errors.New("chroma purchase did not grant the item")
		}
		if recipient.OwnedChromas == nil {
			recipient.OwnedChromas = map[string]bool{}
		}
		if recipient.SeenChromas == nil {
			recipient.SeenChromas = map[string]bool{}
		}
		recipient.OwnedChromas[request.ID] = true
		recipient.SeenChromas[request.ID] = false
	case "buy-chroma-family":
		if recipient.OwnedChromas == nil {
			recipient.OwnedChromas = map[string]bool{}
		}
		if recipient.SeenChromas == nil {
			recipient.SeenChromas = map[string]bool{}
		}
		for _, chroma := range catalog.AllChromas() {
			if shopChromaFamilyKey(chroma) != strings.ToLower(request.ID) ||
				before.OwnedChromas[chroma.ID] || !after.OwnedChromas[chroma.ID] {
				continue
			}
			if recipient.OwnedChromas[chroma.ID] {
				return fmt.Errorf("the recipient already owns %s", chroma.Name)
			}
			recipient.OwnedChromas[chroma.ID] = true
			recipient.SeenChromas[chroma.ID] = false
		}
	case "buy-box":
		if request.BoxType == "hero" {
			recipient.HeroBoxesAvailable++
		} else {
			if recipient.PendingBoxes == nil {
				recipient.PendingBoxes = map[string]int{}
			}
			recipient.PendingBoxes[request.BoxType]++
		}
	case "buy-energy":
		if err := energy.Grant(recipient, request.Amount, now); err != nil {
			return err
		}
	case "buy-consumable":
		offer, ok := consumableOffer(request.ID, before)
		if !ok {
			return fmt.Errorf("unknown consumable %q", request.ID)
		}
		switch request.ID {
		case "camp-draw-one", "camp-draw-three":
			if recipient.DrawsAvailable+offer.Quantity > lootbox.MaxDrawsStored {
				return errors.New("the recipient does not have enough Camp Draw storage")
			}
			recipient.DrawsAvailable += offer.Quantity
		case "restart-one", "restart-six":
			recipient.RestartMissionTokens += offer.Quantity
		case "undo-one", "undo-ten":
			recipient.UndoMoveTokens += offer.Quantity
		case "replace-card-one", "replace-card-ten":
			recipient.ReplaceCardTokens += offer.Quantity
		case "campaign-skip-one":
			recipient.CampaignSkipTokens += offer.Quantity
		default:
			return fmt.Errorf("unknown consumable %q", request.ID)
		}
	case "buy-skill-points":
		offer, ok := skillPointOffer(request.ID)
		if !ok {
			return fmt.Errorf("unknown Skill Point offer %q", request.ID)
		}
		recipient.SkillPoints += offer.Quantity
	case "buy-hero":
		if containsHeroID(recipient.UnlockedHeroes, request.ID) {
			return errors.New("the recipient already owns this hero")
		}
		if _, ok := heroes.ByID(request.ID); !ok {
			return fmt.Errorf("unknown hero %q", request.ID)
		}
		recipient.UnlockedHeroes = append(recipient.UnlockedHeroes, request.ID)
	}
	return nil
}

func consumableOffer(id string, p *profile.Profile) (webShopConsumableOffer, bool) {
	for _, offer := range shopConsumableOffers(p) {
		if offer.ID == id {
			return offer, true
		}
	}
	return webShopConsumableOffer{}, false
}

func skillPointOffer(id string) (webShopSkillPointOffer, bool) {
	for _, offer := range shopSkillPointOffers() {
		if offer.ID == id {
			return offer, true
		}
	}
	return webShopSkillPointOffer{}, false
}

func shopChromaFamilyKey(chroma catalog.Chroma) string {
	parts := strings.Split(strings.ReplaceAll(chroma.Skin, `\`, "/"), "/")
	for index, part := range parts {
		if part == "shop" && index+1 < len(parts) {
			return strings.ToLower(parts[index+1])
		}
	}
	return ""
}
