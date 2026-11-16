// Package server serves the hushtongue API: it wires the handlers of each
// feature to their routes.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/server/auth"
	"github.com/wolf29f/hushtongue/internal/server/health"
	"github.com/wolf29f/hushtongue/internal/server/middleware"
	"github.com/wolf29f/hushtongue/internal/server/players"
	"github.com/wolf29f/hushtongue/internal/server/settings"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 2 * time.Minute

	// maxBodySize is the limit of a request body, sized for the import of a
	// v1 dictionary.
	maxBodySize = 10 << 20 // 10 MB
)

// Server holds the handlers of each feature.
type Server struct {
	sessions *middleware.Sessions
	auth     *auth.Handlers
	health   *health.Handlers
	settings *settings.Handlers
	players  *players.Handlers
}

// New creates the server on top of store.
func New(cfg config.Config, store *postgres.Store) *Server {
	return &Server{
		sessions: middleware.NewSessions(cfg, store),
		auth:     auth.New(cfg, store),
		health:   health.New(store),
		settings: settings.New(cfg),
		players:  players.New(cfg.PublicURL, store),
	}
}

// HTTPServer returns the HTTP server serving the API on addr.
func (s *Server) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}
}

// PurgeSessions deletes the expired sessions now, then once a day until ctx
// is done.
func (s *Server) PurgeSessions(ctx context.Context) {
	s.auth.PurgeSessions(ctx)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health/live", s.health.Live)
	mux.HandleFunc("GET /api/health/ready", s.health.Ready)
	mux.HandleFunc("GET /api/settings", s.settings.Get)

	mux.HandleFunc("POST /api/login", s.auth.Login)
	mux.HandleFunc("POST /api/login/link", s.auth.LinkLogin)
	mux.HandleFunc("POST /api/logout", s.auth.Logout)
	mux.Handle("GET /api/me", s.sessions.RequireSession(http.HandlerFunc(s.auth.Me)))

	mux.Handle("GET /api/players", s.sessions.RequireGM(http.HandlerFunc(s.players.List)))
	mux.Handle("POST /api/players", s.sessions.RequireGM(http.HandlerFunc(s.players.Create)))
	mux.Handle("POST /api/players/{id}/link", s.sessions.RequireGM(http.HandlerFunc(s.players.RegenerateLink)))
	mux.Handle("DELETE /api/players/{id}", s.sessions.RequireGM(http.HandlerFunc(s.players.Delete)))

	return http.MaxBytesHandler(middleware.CrossOrigin(middleware.JSON(mux)), maxBodySize)
}
