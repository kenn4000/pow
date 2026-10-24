package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"prisonersofwar/storage"
)

func integrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	for admin.PingContext(ctx) != nil {
		if ctx.Err() != nil {
			t.Fatal("PostgreSQL did not become available")
		}
		time.Sleep(time.Second)
	}
	schema := "test_" + randomID()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := storage.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	return db
}

type testAccountClient struct {
	t      *testing.T
	client *http.Client
	base   string
	prefix string
}

func (c testAccountClient) request(method, path string, body any, status int) []byte {
	c.t.Helper()
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		input = bytes.NewReader(data)
	}
	r, err := http.NewRequest(method, c.base+c.prefix+"/api"+path, input)
	if err != nil {
		c.t.Fatal(err)
	}
	r.Header.Set("Origin", c.base)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(r)
	if err != nil {
		c.t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	if response.StatusCode != status {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, response.StatusCode, status, data)
	}
	return data
}

func TestPostgresApplication(t *testing.T) {
	db := integrationDatabase(t)
	t.Setenv("BASE_PATH", "/nested/prisoner-of-war")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "a-long-admin-password")
	app := &webServer{db: db, defaultMode: "easy"}
	if err := app.bootstrapAdmin(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.routes())
	defer server.Close()
	t.Setenv("PUBLIC_URL", server.URL)
	newClient := func() testAccountClient {
		jar, _ := cookiejar.New(nil)
		return testAccountClient{t: t, client: &http.Client{Jar: jar}, base: server.URL, prefix: "/nested/prisoner-of-war"}
	}
	a, b, admin := newClient(), newClient(), newClient()
	a.request("GET", "/state", nil, 401)
	a.request("GET", "/admin/users", nil, 401)
	admin.request("POST", "/auth/login", credentials{"admin@example.com", "a-long-admin-password"}, 200)
	var adminMe struct {
		User account `json:"user"`
	}
	json.Unmarshal(admin.request("GET", "/auth/me", nil, 200), &adminMe)
	admin.request("PATCH", "/admin/users/"+adminMe.User.ID+"/role", map[string]string{"role": "user"}, 409)
	data := a.request("POST", "/auth/signup", map[string]string{"email": "Alice@Example.com", "password": "a-long-user-password", "role": "admin"}, 200)
	var alice struct {
		User account `json:"user"`
	}
	if err := json.Unmarshal(data, &alice); err != nil {
		t.Fatal(err)
	}
	if alice.User.Email != "alice@example.com" || alice.User.Role != "user" {
		t.Fatalf("signup privileges: %+v", alice)
	}
	a.request("GET", "/admin/users", nil, 403)
	a.request("PATCH", "/admin/users/"+alice.User.ID+"/role", map[string]string{"role": "admin"}, 403)
	b.request("POST", "/auth/signup", credentials{"bob@example.com", "a-long-other-password"}, 200)
	newClient().request("POST", "/auth/signup", credentials{"alice@example.com", "a-long-user-password"}, 409)
	newClient().request("POST", "/auth/login", credentials{"alice@example.com", "wrong-long-password"}, 401)
	admin.request("GET", "/admin/users", nil, 200)
	admin.request("PATCH", "/admin/users/"+alice.User.ID+"/role", map[string]string{"role": "root"}, 400)
	admin.request("PATCH", "/admin/users/"+alice.User.ID+"/role", map[string]string{"role": "admin"}, 200)
	a.request("GET", "/admin/users", nil, 200)
	a.request("PATCH", "/admin/users/"+adminMe.User.ID+"/role", map[string]string{"role": "user"}, 200)
	admin.request("GET", "/admin/users", nil, 403)
	a.request("PATCH", "/admin/users/"+alice.User.ID+"/role", map[string]string{"role": "user"}, 409)
	var before webView
	json.Unmarshal(a.request("POST", "/new", map[string]string{"mode": "easy"}, 200), &before)
	var other webView
	json.Unmarshal(b.request("GET", "/state", nil, 200), &other)
	if other.Started {
		t.Fatal("another account sees Alice's game")
	}
	var history struct {
		Games []struct {
			ID string `json:"id"`
		} `json:"games"`
	}
	json.Unmarshal(a.request("GET", "/games", nil, 200), &history)
	if len(history.Games) != 1 {
		t.Fatalf("game list: %+v", history)
	}
	gameID := history.Games[0].ID
	b.request("GET", "/games/"+gameID, nil, 404)
	var stored []byte
	if err := db.QueryRow("SELECT state FROM player_data WHERE user_id=$1", alice.User.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	a.request("POST", "/action", map[string]string{"action": "not-an-action"}, 400)
	var unchanged []byte
	db.QueryRow("SELECT state FROM player_data WHERE user_id=$1", alice.User.ID).Scan(&unchanged)
	if !bytes.Equal(stored, unchanged) {
		t.Fatal("failed action changed stored state")
	}
	restarted := httptest.NewServer((&webServer{db: db, defaultMode: "easy"}).routes())
	defer restarted.Close()
	a.base = restarted.URL
	t.Setenv("PUBLIC_URL", restarted.URL)
	var resumed webView
	json.Unmarshal(a.request("GET", "/state", nil, 200), &resumed)
	if !resumed.Started || resumed.Phase != before.Phase || fmt.Sprint(resumed.Players[0].Hand) != fmt.Sprint(before.Players[0].Hand) {
		t.Fatal("game did not resume after restart")
	}
	view := resumed
	for step := 0; step < 80 && view.Phase != "gameover"; step++ {
		body := map[string]any{}
		switch view.Phase {
		case "commit":
			body = map[string]any{"action": "commit", "cards": []int{0, 1, 2}}
		case "attack", "defend":
			body = map[string]any{"action": view.Phase, "index": 0}
		case "result":
			body["action"] = "continue"
		case "seat":
			body = map[string]any{"action": "seat", "seat": "X"}
		default:
			t.Fatalf("unexpected phase %s", view.Phase)
		}
		json.Unmarshal(a.request("POST", "/action", body, 200), &view)
	}
	if view.Phase != "gameover" {
		t.Fatal("game did not complete")
	}
	var status string
	var eventCount int
	if err := db.QueryRow("SELECT status FROM games WHERE id=$1", gameID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("game status: %s", status)
	}
	db.QueryRow("SELECT count(*) FROM game_events WHERE game_id=$1", gameID).Scan(&eventCount)
	if eventCount < 10 {
		t.Fatalf("only %d events saved", eventCount)
	}
	a.request("GET", "/games/"+gameID, nil, 200)
	var profilePage webHistoryPage
	json.Unmarshal(a.request("GET", "/history", nil, 200), &profilePage)
	if profilePage.TotalGames != 1 || len(profilePage.Matches) != 1 {
		t.Fatalf("history not recorded: %+v", profilePage)
	}
	a.request("POST", "/new", map[string]string{"mode": "easy"}, 200)
	a.request("POST", "/new", map[string]string{"mode": "easy"}, 200)
	var abandoned int
	db.QueryRow("SELECT count(*) FROM games WHERE user_id=$1 AND status='abandoned'", alice.User.ID).Scan(&abandoned)
	if abandoned != 1 {
		t.Fatalf("abandoned count %d", abandoned)
	}
	if _, err := db.Exec(`UPDATE player_data SET profile=jsonb_set(profile,'{camp_draws_available}','2') WHERE user_id=$1`, alice.User.ID); err != nil {
		t.Fatal(err)
	}
	a.request("POST", "/camp-draw", map[string]string{"action": "start-draw"}, 200)
	a.request("POST", "/camp-draw", map[string]any{"action": "play-card", "index": 0}, 200)
	secondRestart := httptest.NewServer((&webServer{db: db}).routes())
	defer secondRestart.Close()
	a.base = secondRestart.URL
	t.Setenv("PUBLIC_URL", secondRestart.URL)
	var draw webCampDrawPage
	json.Unmarshal(a.request("GET", "/camp-draw", nil, 200), &draw)
	if !draw.Started || draw.Prompt == nil || draw.Prompt.ExchangeIndex != 1 {
		t.Fatalf("Camp Draw did not resume: %+v", draw)
	}
	a.request("POST", "/camp-draw", map[string]any{"action": "play-card", "index": 0}, 200)
	json.Unmarshal(a.request("POST", "/camp-draw", map[string]any{"action": "play-card", "index": 0}, 200), &draw)
	if draw.Result == nil || draw.DrawsAvailable != 1 {
		t.Fatalf("Camp Draw completion not persisted: %+v", draw)
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); a.request("GET", "/state", nil, 200) }()
	}
	wg.Wait()
	request, _ := http.NewRequest("POST", a.base+a.prefix+"/api/auth/logout", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response, err := a.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
	a.request("POST", "/auth/logout", nil, 200)
	a.request("GET", "/state", nil, 401)
}

func TestPostgresRollsBackBeforeResponding(t *testing.T) {
	db := integrationDatabase(t)
	t.Setenv("BASE_PATH", "/cards")
	t.Setenv("COOKIE_SECURE", "false")
	server := httptest.NewServer((&webServer{db: db}).routes())
	defer server.Close()
	t.Setenv("PUBLIC_URL", server.URL)
	jar, _ := cookiejar.New(nil)
	client := testAccountClient{t: t, client: &http.Client{Jar: jar}, base: server.URL, prefix: "/cards"}
	client.request("POST", "/auth/signup", credentials{"atomic@example.com", "a-long-user-password"}, 200)
	if _, err := db.Exec(`CREATE FUNCTION reject_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'deliberate test write failure'; END $$; CREATE TRIGGER reject_write BEFORE UPDATE ON player_data FOR EACH ROW EXECUTE FUNCTION reject_write()`); err != nil {
		t.Fatal(err)
	}
	client.request("POST", "/new", map[string]string{"mode": "easy"}, 500)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM games").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed transaction left a game behind")
	}
	if err := db.QueryRow("SELECT count(*) FROM game_events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed transaction left events behind")
	}
	if _, err := db.Exec("DROP TRIGGER reject_write ON player_data; DROP FUNCTION reject_write()"); err != nil {
		t.Fatal(err)
	}
	var view webView
	json.Unmarshal(client.request("GET", "/state", nil, 200), &view)
	if view.Started {
		t.Fatal("failed transaction left active state behind")
	}
}
