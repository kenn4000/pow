package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPostgresPasswordChanges(t *testing.T) {
	db := integrationDatabase(t)
	t.Setenv("BASE_PATH", "/cards")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "password-admin@example.com")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "initial-admin-password")
	app := &webServer{db: db}
	if err := app.bootstrapAdmin(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.routes())
	defer server.Close()
	t.Setenv("PUBLIC_URL", server.URL)
	newClient := func() testAccountClient {
		jar, _ := cookiejar.New(nil)
		return testAccountClient{t: t, client: &http.Client{Jar: jar}, base: server.URL, prefix: "/cards"}
	}
	a, b, other := newClient(), newClient(), newClient()
	oldPassword, newPassword := "initial-admin-password", "updated-admin-password"
	a.request("POST", "/auth/login", credentials{"password-admin@example.com", oldPassword}, 200)
	b.request("POST", "/auth/login", credentials{"password-admin@example.com", oldPassword}, 200)
	other.request("POST", "/auth/signup", credentials{"other@example.com", "unrelated-user-password"}, 200)
	newClient().request("POST", "/account/password", passwordChange{oldPassword, newPassword}, 401)
	a.request("POST", "/account/password", passwordChange{oldPassword, "short"}, 400)
	a.request("POST", "/account/password", passwordChange{oldPassword, oldPassword}, 400)
	a.request("POST", "/account/password", passwordChange{"incorrect-old-password", newPassword}, 400)
	a.request("GET", "/auth/me", nil, 200)
	a.request("POST", "/new", map[string]string{"mode": "easy"}, 200)
	if _, err := db.Exec(`CREATE FUNCTION reject_session() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'deliberate session write failure'; END $$; CREATE TRIGGER reject_session BEFORE INSERT ON auth_sessions FOR EACH ROW EXECUTE FUNCTION reject_session()`); err != nil {
		t.Fatal(err)
	}
	a.request("POST", "/account/password", passwordChange{oldPassword, newPassword}, 500)
	a.request("GET", "/auth/me", nil, 200)
	b.request("GET", "/auth/me", nil, 200)
	var hash []byte
	if err := db.QueryRow("SELECT password_hash FROM users WHERE email='password-admin@example.com'").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(oldPassword)) != nil {
		t.Fatal("failed password change was not rolled back")
	}
	if _, err := db.Exec("DROP TRIGGER reject_session ON auth_sessions; DROP FUNCTION reject_session()"); err != nil {
		t.Fatal(err)
	}
	location, _ := url.Parse(server.URL + "/cards/")
	oldCookie := a.client.Jar.Cookies(location)[0].Value
	a.request("POST", "/account/password", passwordChange{oldPassword, newPassword}, 200)
	if a.client.Jar.Cookies(location)[0].Value == oldCookie {
		t.Fatal("session token was not rotated")
	}
	var me struct {
		User account `json:"user"`
	}
	json.Unmarshal(a.request("GET", "/auth/me", nil, 200), &me)
	if me.User.Role != "admin" {
		t.Fatal("password change modified the account role")
	}
	var view webView
	json.Unmarshal(a.request("GET", "/state", nil, 200), &view)
	if !view.Started {
		t.Fatal("password change lost active game")
	}
	b.request("GET", "/auth/me", nil, 401)
	other.request("GET", "/auth/me", nil, 200)
	newClient().request("POST", "/auth/login", credentials{"password-admin@example.com", oldPassword}, 401)
	b.request("POST", "/auth/login", credentials{"password-admin@example.com", newPassword}, 200)
	if err := app.bootstrapAdmin(context.Background()); err != nil {
		t.Fatal(err)
	}
	newClient().request("POST", "/auth/login", credentials{"password-admin@example.com", oldPassword}, 401)
	concurrent := newClient()
	var wg sync.WaitGroup
	var loginStatus int
	var loginErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := `{"email":"password-admin@example.com","password":"updated-admin-password"}`
		r, err := http.NewRequest("POST", server.URL+"/cards/api/auth/login", strings.NewReader(data))
		if err != nil {
			loginErr = err
			return
		}
		r.Header.Set("Origin", server.URL)
		response, err := concurrent.client.Do(r)
		if err != nil {
			loginErr = err
			return
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		loginStatus = response.StatusCode
	}()
	a.request("POST", "/account/password", passwordChange{newPassword, "third-admin-password"}, 200)
	wg.Wait()
	if loginErr != nil {
		t.Fatal(loginErr)
	}
	if loginStatus != 200 && loginStatus != 401 {
		t.Fatalf("concurrent login returned %d", loginStatus)
	}
	concurrent.request("GET", "/auth/me", nil, 401)
	b.request("GET", "/auth/me", nil, 401)
	a.request("GET", "/auth/me", nil, 200)
	newClient().request("POST", "/auth/login", credentials{"password-admin@example.com", "third-admin-password"}, 200)
}
