package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

func TestDevelopmentPickerListsEveryAccountAndUsesRealPermissions(t *testing.T) {
	db := integrationDatabase(t)
	t.Setenv("BASE_PATH", "")
	t.Setenv("COOKIE_SECURE", "false")
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var admin, player account
	for index := 0; index < 103; index++ {
		role := "user"
		if index == 0 {
			role = "admin"
		}
		user, err := insertAccount(context.Background(), tx, fmt.Sprintf("picker-%03d@example.com", index), dummyPasswordHash, role)
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			admin = user
		}
		if index == 102 {
			player = user
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	web := &webServer{db: db, defaultMode: "easy"}
	server := httptest.NewServer(web.routes())
	defer server.Close()
	t.Setenv("PUBLIC_URL", server.URL)
	t.Setenv("DEVELOPMENT_LOGIN", "true")
	jar, _ := cookiejar.New(nil)
	client := testAccountClient{t: t, client: &http.Client{Jar: jar}, base: server.URL}
	var page struct {
		Users []account `json:"users"`
		Next  string    `json:"next"`
	}
	if err := json.Unmarshal(client.request("GET", "/auth/users", nil, 200), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Users) != 100 || page.Next == "" {
		t.Fatalf("first picker page: %d users, cursor %q", len(page.Users), page.Next)
	}
	if err := json.Unmarshal(client.request("GET", "/auth/users?after="+page.Next, nil, 200), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Users) != 3 || page.Next != "" {
		t.Fatalf("last picker page: %d users, cursor %q", len(page.Users), page.Next)
	}
	client.request("POST", "/auth/assume", map[string]string{"userId": player.ID}, 200)
	var response struct {
		User account `json:"user"`
	}
	json.Unmarshal(client.request("GET", "/auth/me", nil, 200), &response)
	if response.User.ID != player.ID || response.User.Role != "user" {
		t.Fatalf("wrong assumed account: %+v", response.User)
	}
	client.request("GET", "/admin/users", nil, 403)
	client.request("GET", "/cards", nil, 200)
	client.request("POST", "/new", map[string]string{"mode": "easy"}, 200)
	client.request("POST", "/auth/assume", map[string]string{"userId": admin.ID}, 200)
	client.request("GET", "/admin/users?q=picker-102", nil, 200)
	var state webView
	json.Unmarshal(client.request("GET", "/state", nil, 200), &state)
	if state.Started {
		t.Fatal("switching accounts leaked the previous player's game")
	}
	client.request("POST", "/auth/assume", map[string]string{"userId": "missing"}, 404)
}
