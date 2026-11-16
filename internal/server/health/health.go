// Package health tells whether the server and its database answer.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/wolf29f/hushtongue/internal/server/httpapi"
)

// readyTimeout bounds the database ping of the readiness check.
const readyTimeout = 2 * time.Second

// Pinger checks that the database answers.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handlers serves the health routes.
type Handlers struct {
	db Pinger
}

// New creates the handlers checking db.
func New(db Pinger) *Handlers {
	return &Handlers{db: db}
}

// Live serves GET /api/health/live: it answers as soon as the server runs,
// whatever the state of the database, which restarting the server wouldn't
// fix.
func (h *Handlers) Live(w http.ResponseWriter, _ *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready serves GET /api/health/ready: it answers when the database does.
func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		slog.Warn("Database unreachable", "error", err)
		httpapi.WriteError(w, http.StatusServiceUnavailable, httpapi.CodeDatabaseUnavailable, "Base de données indisponible.")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
