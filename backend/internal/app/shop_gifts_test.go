package app

import (
	"testing"
	"time"

	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/profile"
)

func TestShopGiftEligibilityRequiresFriendshipAndAccountAge(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	accountCreated := now.Add(-giftAccountAge)
	friendshipCreated := now.Add(-giftFriendshipAge)

	if eligible, reason := shopGiftEligibility(accountCreated, friendshipCreated, now); !eligible || reason != "" {
		t.Fatalf("exact eligibility boundaries = %v, %q; want eligible", eligible, reason)
	}
	if eligible, reason := shopGiftEligibility(accountCreated, friendshipCreated.Add(time.Nanosecond), now); eligible || reason == "" {
		t.Fatalf("recent friendship = %v, %q; want ineligible with reason", eligible, reason)
	}
	if eligible, reason := shopGiftEligibility(accountCreated.Add(time.Nanosecond), friendshipCreated, now); eligible || reason == "" {
		t.Fatalf("recent account = %v, %q; want ineligible with reason", eligible, reason)
	}
}

func TestGrantShopGiftTransfersSupportedInventory(t *testing.T) {
	now := time.Now().UTC()
	before, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	chromaID := ""
	for _, chroma := range catalog.AllChromas() {
		if !before.OwnedChromas[chroma.ID] && !recipient.OwnedChromas[chroma.ID] {
			chromaID = chroma.ID
			break
		}
	}
	if chromaID == "" {
		t.Fatal("shop catalog has no chroma available for gift test")
	}
	after, err := cloneProfile(before)
	if err != nil {
		t.Fatal(err)
	}
	after.OwnedChromas[chromaID] = true
	if err := grantShopGift(recipient, before, after, actionRequest{Action: "buy-chroma", ID: chromaID}, now); err != nil {
		t.Fatal(err)
	}
	if !recipient.OwnedChromas[chromaID] {
		t.Fatal("chroma gift was not added to recipient")
	}

	if err := grantShopGift(recipient, before, after, actionRequest{Action: "buy-box", BoxType: "silver"}, now); err != nil {
		t.Fatal(err)
	}
	if recipient.PendingBoxes["silver"] != 1 {
		t.Fatalf("silver box gift count = %d, want 1", recipient.PendingBoxes["silver"])
	}

	recipient.Energy = 0
	if err := grantShopGift(recipient, before, after, actionRequest{Action: "buy-energy", Amount: 1}, now); err != nil {
		t.Fatal(err)
	}
	if recipient.Energy != 1 {
		t.Fatalf("energy gift amount = %d, want 1", recipient.Energy)
	}

	if err := grantShopGift(recipient, before, after, actionRequest{Action: "buy-consumable", ID: "restart-six"}, now); err != nil {
		t.Fatal(err)
	}
	if recipient.RestartMissionTokens != 6 {
		t.Fatalf("restart token gift count = %d, want 6", recipient.RestartMissionTokens)
	}
	if err := grantShopGift(recipient, before, after, actionRequest{Action: "buy-skill-points", ID: "skill-point-ten"}, now); err != nil {
		t.Fatal(err)
	}
	if recipient.SkillPoints != 10 {
		t.Fatalf("Skill Point gift count = %d, want 10", recipient.SkillPoints)
	}
}
