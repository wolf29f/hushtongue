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
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
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
	logger   *slog.Logger
	auth     *auth.Handlers
	health   *health.Handlers
	settings *settings.Handlers
	players  *players.Handlers
}

// New creates the server on top of store.
func New(cfg config.Config, store *postgres.Store, logger *slog.Logger) *Server {
	return &Server{
		logger:   logger,
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
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelError),
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
	mux.HandleFunc("GET /api/me", s.auth.RequireSession(s.auth.Me))

	mux.HandleFunc("GET /api/players", s.auth.RequireGM(s.players.List))
	mux.HandleFunc("POST /api/players", s.auth.RequireGM(s.players.Create))
	mux.HandleFunc("POST /api/players/{id}/link", s.auth.RequireGM(s.players.RegenerateLink))
	mux.HandleFunc("DELETE /api/players/{id}", s.auth.RequireGM(s.players.Delete))

	crossOrigin := http.NewCrossOriginProtection()
	crossOrigin.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteError(w, http.StatusForbidden, httpapi.CodeCrossOrigin, "Requête d'un autre site refusée.")
	}))
	return http.MaxBytesHandler(crossOrigin.Handler(httpapi.Handler(mux)), maxBodySize)
}
