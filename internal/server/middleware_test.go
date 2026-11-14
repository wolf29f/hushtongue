package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

func TestRequireGM(t *testing.T) {
	s, pool := newTestServer(t, testConfig)
	handler := s.requireGM(func(w http.ResponseWriter, _ *http.Request, user storage.User) {
		writeJSON(w, http.StatusOK, user.Name)
	})
	player := &http.Cookie{Name: sessionCookie, Value: "player-token"}
	if err := s.store.CreateSession(context.Background(), tokenHash(player), pgtest.NewUser(t, pool), time.Hour); err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	tests := []struct {
		name   string
		cookie *http.Cookie
		status int
	}{
		{"GM", login(t, s), http.StatusOK},
		{"player", player, http.StatusForbidden},
		{"no session", nil, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != nil {
				r.AddCookie(tt.cookie)
			}
			w := httptest.NewRecorder()
			handler(w, r)

			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestRequireSessionDatabaseDown(t *testing.T) {
	// Nothing listens on port 1: the query fails at once
	db, err := pgxpool.New(context.Background(), "postgres://127.0.0.1:1/hushtongue?connect_timeout=1")
	if err != nil {
		t.Fatalf("pgxpool.New() failed: %v", err)
	}
	defer db.Close()
	s := New(testConfig, postgres.New(db), slog.Default())

	w := serve(s, http.MethodGet, "/api/me", "", &http.Cookie{Name: sessionCookie, Value: "token"})

	assertError(t, w, http.StatusInternalServerError, codeInternal)
}
