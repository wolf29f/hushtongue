// Package server serves the hushtongue API.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/wolf29f/hushtongue/internal/config"
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

	purgeInterval = 24 * time.Hour
)

// Server holds the dependencies of the handlers.
type Server struct {
	cfg          config.Config
	store        *postgres.Store
	logger       *slog.Logger
	loginLimiter *limiter
}

// New creates the server on top of store.
func New(cfg config.Config, store *postgres.Store, logger *slog.Logger) *Server {
	return &Server{
		cfg:          cfg,
		store:        store,
		logger:       logger,
		loginLimiter: newLimiter(loginAttempts, loginPeriod, time.Now),
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
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()
	for {
		purged, err := s.store.PurgeSessions(ctx)
		if err != nil && ctx.Err() == nil {
			s.logger.Error("Purging sessions failed", "error", err)
		} else if purged > 0 {
			s.logger.Info("Purged expired sessions", "count", purged)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health/live", s.handleLive)
	mux.HandleFunc("GET /api/health/ready", s.handleReady)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.requireSession(s.handleMe))

	crossOrigin := http.NewCrossOriginProtection()
	crossOrigin.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusForbidden, codeCrossOrigin, "Requête d'un autre site refusée.")
	}))
	return http.MaxBytesHandler(crossOrigin.Handler(apiHandler(mux)), maxBodySize)
}
