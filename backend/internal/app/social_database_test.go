package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"prisonersofwar/internal/catalog"
	"prisonersofwar/internal/profile"
)

func TestPostgresSocialGiftingChatAndWagerFlows(t *testing.T) {
	db := integrationDatabase(t)
	t.Setenv("BASE_PATH", "/social-test")
	t.Setenv("COOKIE_SECURE", "false")
	server := httptest.NewServer((&webServer{db: db, defaultMode: "easy"}).routes())
	defer server.Close()
	t.Setenv("PUBLIC_URL", server.URL)
	newClient := func() testAccountClient {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		return testAccountClient{t: t, client: &http.Client{Jar: jar}, base: server.URL, prefix: "/social-test"}
	}
	alice, bob := newClient(), newClient()
	var aliceAccount, bobAccount account
	if err := json.Unmarshal(alice.request("POST", "/auth/signup", credentials{
		Email: "alice-social@example.com", Password: "a-long-social-password",
	}, http.StatusOK), &struct {
		User *account `json:"user"`
	}{User: &aliceAccount}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bob.request("POST", "/auth/signup", credentials{
		Email: "bob-social@example.com", Password: "another-long-password",
	}, http.StatusOK), &struct {
		User *account `json:"user"`
	}{User: &bobAccount}); err != nil {
		t.Fatal(err)
	}

	alice.request("POST", "/social/requests", map[string]string{"handle": bobAccount.PublicHandle}, http.StatusCreated)
	var requests struct {
		IncomingRequests []socialFriendRequest `json:"incomingRequests"`
	}
	if err := json.Unmarshal(bob.request("GET", "/social", nil, http.StatusOK), &requests); err != nil {
		t.Fatal(err)
	}
	if len(requests.IncomingRequests) != 1 {
		t.Fatalf("incoming requests = %d, want 1", len(requests.IncomingRequests))
	}
	bob.request("POST", fmt.Sprintf("/social/requests/%s/accept", requests.IncomingRequests[0].ID), map[string]any{}, http.StatusOK)
	for _, id := range []string{aliceAccount.ID, bobAccount.ID} {
		if _, err := db.Exec(`
			UPDATE player_data SET profile=jsonb_set(profile,'{marks}','100000'::jsonb)
			WHERE user_id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("UPDATE users SET created_at=now()-interval '8 days' WHERE id IN ($1,$2)", aliceAccount.ID, bobAccount.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		UPDATE friendships SET created_at=now()-interval '3 days'
		WHERE user_a=LEAST($1,$2) AND user_b=GREATEST($1,$2)`, aliceAccount.ID, bobAccount.ID); err != nil {
		t.Fatal(err)
	}
	var socialOverview struct {
		Friends []socialPlayer `json:"friends"`
	}
	if err := json.Unmarshal(alice.request("GET", "/social", nil, http.StatusOK), &socialOverview); err != nil {
		t.Fatal(err)
	}
	if len(socialOverview.Friends) != 1 {
		t.Fatalf("friends list = %d, want 1", len(socialOverview.Friends))
	}

	var aliceProfile profile.Profile
	var saved []byte
	if err := db.QueryRow("SELECT profile FROM player_data WHERE user_id=$1", aliceAccount.ID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(saved, &aliceProfile); err != nil {
		t.Fatal(err)
	}
	var chromaID string
	for _, chroma := range catalog.AllChromas() {
		if !aliceProfile.OwnedChromas[chroma.ID] {
			chromaID = chroma.ID
			break
		}
	}
	if chromaID == "" {
		t.Fatal("shop catalog has no chroma available for gift test")
	}
	alice.request("POST", "/shop/gift", map[string]string{
		"action": "buy-chroma", "id": chromaID, "recipient": bobAccount.PublicHandle,
	}, http.StatusOK)
	var bobProfile profile.Profile
	saved = nil
	if err := db.QueryRow("SELECT profile FROM player_data WHERE user_id=$1", bobAccount.ID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(saved, &bobProfile); err != nil {
		t.Fatal(err)
	}
	if !bobProfile.OwnedChromas[chromaID] {
		t.Fatalf("gifted chroma %q was not added to recipient", chromaID)
	}

	alice.request("POST", "/social/messages", map[string]string{"handle": bobAccount.PublicHandle, "body": "Hello"}, http.StatusCreated)
	alice.request("POST", "/social/conversations/delete", map[string]string{"handle": bobAccount.PublicHandle}, http.StatusOK)
	var aliceThread, bobThread struct {
		Messages []socialMessage `json:"messages"`
	}
	if err := json.Unmarshal(alice.request("GET", "/social/conversations/"+bobAccount.PublicHandle, nil, http.StatusOK), &aliceThread); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bob.request("GET", "/social/conversations/"+aliceAccount.PublicHandle, nil, http.StatusOK), &bobThread); err != nil {
		t.Fatal(err)
	}
	if len(aliceThread.Messages) != 0 || len(bobThread.Messages) != 1 {
		t.Fatalf("per-user chat deletion: Alice sees %d messages; Bob sees %d", len(aliceThread.Messages), len(bobThread.Messages))
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	playerIDs := [2]string{aliceAccount.ID, bobAccount.ID}
	if err := reserveFriendWagerTx(context.Background(), tx, playerIDs, 100); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	matchID := randomID()
	if err := createFriendWagerTx(context.Background(), tx, "online", matchID, playerIDs, 100); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := settleFriendWagerTx(context.Background(), tx, matchID, "online", 0, time.Now().UTC()); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	_, net, err := (&webServer{db: db}).friendRecord(context.Background(), aliceAccount.ID, bobAccount.ID)
	if err != nil {
		t.Fatal(err)
	}
	if net != 100 {
		t.Fatalf("winner's net wager = %d Marks, want 100", net)
	}
	if err := json.Unmarshal(alice.request("GET", "/social", nil, http.StatusOK), &socialOverview); err != nil {
		t.Fatal(err)
	}
	if got := socialOverview.Friends[0].NetWagerMarks; got != 100 {
		t.Fatalf("social friend net wager = %d, want 100", got)
	}

	alice.request("POST", "/online-battle", map[string]any{
		"action": "invite", "variant": "main", "recipient": bobAccount.PublicHandle,
		"heroId": "recruit", "wagerMarks": 100,
	}, http.StatusCreated)
	var incoming onlineBattleResponse
	if err := json.Unmarshal(bob.request("GET", "/online-battle", nil, http.StatusOK), &incoming); err != nil {
		t.Fatal(err)
	}
	if len(incoming.Incoming) != 1 || incoming.Incoming[0].WagerMarks != 100 {
		t.Fatalf("incoming wager invitation = %+v", incoming.Incoming)
	}
	var accepted onlineBattleResponse
	if err := json.Unmarshal(bob.request("POST", "/online-battle", map[string]any{
		"action": "accept", "inviteId": incoming.Incoming[0].ID, "heroId": "recruit",
	}, http.StatusCreated), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.Match == nil || accepted.Match.WagerMarks != 100 {
		t.Fatalf("accepted battle wager = %+v", accepted.Match)
	}
	for _, playerID := range []string{aliceAccount.ID, bobAccount.ID} {
		var remaining int
		if err := db.QueryRow(`SELECT (profile->>'energy')::integer FROM player_data WHERE user_id=$1`, playerID).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining != 9 {
			t.Errorf("friend match player %s has %d Energy, want 9", playerID, remaining)
		}
	}
}
