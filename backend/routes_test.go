package main

import (
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
		{"/prisoner-of-war/skins/tribal_red/small_idle.png", http.StatusOK},
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
