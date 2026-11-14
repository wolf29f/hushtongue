package server

import (
	"context"
	"net/http"
	"time"
)

// readyTimeout bounds the database ping of the readiness check.
const readyTimeout = 2 * time.Second

// handleLive serves GET /api/health/live: it answers as soon as the server
// runs, whatever the state of the database, which restarting the server
// wouldn't fix.
func (s *Server) handleLive(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady serves GET /api/health/ready: it answers when the database
// does.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		s.logger.Warn("Database unreachable", "error", err)
		writeError(w, http.StatusServiceUnavailable, codeDatabaseUnavailable, "Base de données indisponible.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
