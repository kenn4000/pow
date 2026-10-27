package app

import (
	"database/sql"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrefixedDeploymentRoutes(t *testing.T) {
	t.Setenv("BASE_PATH", "/prisoner-of-war")
	server := &webServer{rng: rand.New(rand.NewSource(1))}
	handler := server.routes()
	for _, test := range []struct {
		path string
		code int
	}{
		{"/healthz", http.StatusOK},
		{"/prisoner-of-war/api/state", http.StatusOK},
		{"/prisoner-of-war/skins/shop/tribal/red/small_idle.png", http.StatusOK},
		{"/prisoner-of-war/skills/icons/BattleRewards.png", http.StatusOK},
		{"/api/state", http.StatusNotFound},
		{"/other-game/api/state", http.StatusNotFound},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != test.code {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.code, response.Body.String())
			}
		})
	}
}

func TestNestedChiRouteScopes(t *testing.T) {
	t.Setenv("BASE_PATH", "/games/cards")
	router := (&webServer{db: new(sql.DB)}).routes()
	for _, test := range []struct {
		method, path, origin string
		status               int
	}{
		{"GET", "/healthz", "", 200},
		{"GET", "/games/cards/", "", 200},
		{"GET", "/games/cards/skins/shop/tribal/red/small_idle.png", "", 200},
		{"GET", "/games/cards/skills/icons/Campaign.png", "", 200},
		{"GET", "/games/cards/api/state", "", 401},
		{"GET", "/games/cards/api/admin/users", "", 401},
		{"GET", "/games/cards/api/admin/cards", "", 401},
		{"POST", "/games/cards/api/account/password", "", 401},
		{"GET", "/games/cards/api/auth/me", "", 401},
		{"POST", "/games/cards/api/auth/logout", "https://attacker.example", 403},
		{"GET", "/api/state", "", 404},
		{"GET", "/games/cards/missing", "", 404},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			r := httptest.NewRequest(test.method, test.path, nil)
			r.Header.Set("Origin", test.origin)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatalf("status %d, want %d: %s", w.Code, test.status, w.Body.String())
			}
		})
	}
}
