package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// The passwordless picker must be unreachable unless explicitly enabled. The
// auth routes need a database, so handleAuth is mounted directly; the JSON body
// distinguishes the gate's 404 from chi's plain-text unmatched-route 404.
func TestDevelopmentLoginEndpointsAreHiddenUnlessEnabled(t *testing.T) {
	t.Setenv("DEVELOPMENT_LOGIN", "")
	web := &webServer{defaultMode: "easy"}
	router := chi.NewRouter()
	router.Get("/api/auth/{authAction}", web.handleAuth)
	router.Post("/api/auth/{authAction}", web.handleAuth)
	server := httptest.NewServer(router)
	defer server.Close()
	for _, request := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/auth/users", ""},
		{http.MethodPost, "/api/auth/assume", `{"userId":"anything"}`},
	} {
		req, err := http.NewRequest(request.method, server.URL+request.path, strings.NewReader(request.body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound || !strings.Contains(string(body), `"error":"not found"`) {
			t.Fatalf("%s %s = %d %q with DEVELOPMENT_LOGIN unset, want the gate's JSON 404", request.method, request.path, response.StatusCode, body)
		}
	}
}

func TestDevelopmentLoginEnabledRequiresExplicitTrue(t *testing.T) {
	for value, want := range map[string]bool{"": false, "false": false, "1": false, "TRUE": false, "true": true} {
		t.Setenv("DEVELOPMENT_LOGIN", value)
		if got := developmentLoginEnabled(); got != want {
			t.Fatalf("DEVELOPMENT_LOGIN=%q enabled=%v, want %v", value, got, want)
		}
	}
}
