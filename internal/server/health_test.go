package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/pgtest"
)

func TestHandleReady(t *testing.T) {
	t.Run("ready when the database answers", func(t *testing.T) {
		w := httptest.NewRecorder()
		New(pgtest.New(t), slog.Default()).routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health/ready", nil))

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("unavailable when the database doesn't answer", func(t *testing.T) {
		// Nothing listens on port 1: the ping fails at once
		db, err := pgxpool.New(context.Background(), "postgres://127.0.0.1:1/hushtongue?connect_timeout=1")
		if err != nil {
			t.Fatalf("pgxpool.New() failed: %v", err)
		}
		defer db.Close()

		w := httptest.NewRecorder()
		New(db, slog.Default()).routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health/ready", nil))

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", w.Code)
		}
	})
}
