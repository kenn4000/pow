package app

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type requestKey int

const (
	accountKey requestKey = iota
	gameServerKey
)

func (s *webServer) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	if prefix := strings.TrimRight(os.Getenv("BASE_PATH"), "/"); prefix != "" {
		r.Route(prefix, s.applicationRoutes)
	} else {
		s.applicationRoutes(r)
	}
	return r
}

func (s *webServer) applicationRoutes(r chi.Router) {
	r.Get("/", s.handlePage)
	r.Get("/build-info", s.handleBuildInfo)
	r.Route("/skins", func(assets chi.Router) {
		assets.Get("/*", s.handleSkinAsset)
	})
	r.Route("/skills/icons", func(icons chi.Router) {
		icons.Get("/*", s.handleSkillIcon)
	})
	r.Route("/api", func(api chi.Router) {
		api.Use(sameOrigin)
		if s.db == nil {
			s.registerGameRoutes(api)
			return
		}
		api.Route("/auth", func(auth chi.Router) {
			auth.Get("/{authAction}", s.handleAuth)
			auth.Post("/{authAction}", s.handleAuth)
		})
		api.Group(func(protected chi.Router) {
			protected.Use(s.authenticate)
			protected.Get("/cards", s.handleCardCatalog)
			protected.Route("/account", func(account chi.Router) {
				account.Post("/password", s.handleChangePassword)
			})
			protected.Route("/social", func(social chi.Router) {
				social.Get("/", s.handleSocialOverview)
				social.Post("/handle", s.handleSocialSetHandle)
				social.Post("/requests", s.handleSocialFriendRequest)
				social.Post("/requests/{requestID}/{decision}", s.handleSocialRequestResponse)
				social.Post("/blocks", s.handleSocialBlock)
				social.Post("/unblock", s.handleSocialUnblock)
				social.Post("/messages", s.handleSocialMessage)
			})
			protected.Route("/puzzle-battle", func(puzzleBattle chi.Router) {
				puzzleBattle.Get("/", s.handlePuzzleBattle)
				puzzleBattle.Post("/", s.handlePuzzleBattle)
				puzzleBattle.Get("/history", s.handlePuzzleBattleHistory)
				puzzleBattle.Post("/{matchID}/action", s.handlePuzzleBattleAction)
			})
			protected.Route("/online-battle", func(onlineBattle chi.Router) {
				onlineBattle.Get("/", s.handleOnlineBattle)
				onlineBattle.Post("/", s.handleOnlineBattle)
				onlineBattle.Post("/{matchID}/action", s.handleOnlineBattleAction)
			})
			protected.Route("/admin", func(admin chi.Router) {
				admin.Use(requireAdmin)
				admin.Get("/cards", s.handleCardCatalog)
				admin.Route("/users", func(users chi.Router) {
					users.Get("/", s.handleAdmin)
					users.Patch("/{userID}/role", s.handleAdmin)
				})
			})
			protected.Route("/games", func(history chi.Router) {
				history.Get("/", s.serveGames)
				history.Get("/{gameID}", s.serveGames)
			})
			protected.Group(func(game chi.Router) {
				game.Use(s.withPlayer)
				s.registerGameRoutes(game)
			})
		})
	})
}

func (s *webServer) registerGameRoutes(r chi.Router) {
	r.Get("/state", s.gameEndpoint((*webServer).handleState))
	r.Post("/new", s.gameEndpoint((*webServer).handleNewGame))
	r.Post("/action", s.gameEndpoint((*webServer).handleAction))
	r.Get("/achievements", s.gameEndpoint((*webServer).handleAchievements))
	r.Get("/profile", s.gameEndpoint((*webServer).handleProfile))
	r.Post("/profile", s.gameEndpoint((*webServer).handleProfile))
	r.Get("/campaigns", s.gameEndpoint((*webServer).handleCampaigns))
	r.Get("/history", s.gameEndpoint((*webServer).handleHistory))
	r.Post("/preview/reset", s.gameEndpoint((*webServer).handlePreviewReset))
	for path, handler := range map[string]func(*webServer, http.ResponseWriter, *http.Request){
		"/camp-draw": (*webServer).handleCampDraw,
		"/shop":      (*webServer).handleShop,
		"/skills":    (*webServer).handleSkills,
		"/heroes":    (*webServer).handleHeroes,
	} {
		r.Get(path, s.gameEndpoint(handler))
		r.Post(path, s.gameEndpoint(handler))
	}
}

func (s *webServer) gameEndpoint(handler func(*webServer, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, _ := r.Context().Value(gameServerKey).(*webServer)
		if current == nil {
			current = s
		}
		handler(current, w, r)
	}
}

func (s *webServer) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.requireAccount(w, r)
		if !ok {
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), accountKey, u)))
	})
}

func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(accountKey).(account)
		if u.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "administrator role required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *webServer) serveGames(w http.ResponseWriter, r *http.Request) {
	u, _ := r.Context().Value(accountKey).(account)
	s.handleGames(w, r, u)
}
