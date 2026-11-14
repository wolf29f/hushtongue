package server

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

func TestSettings(t *testing.T) {
	s := New(testConfig, postgres.New(nil), slog.Default())

	w := serve(s, http.MethodGet, "/api/settings", "")

	want := `{"languages":{"source":"Français","con":"Ailo"}}`
	if got := strings.TrimSpace(w.Body.String()); w.Code != http.StatusOK || got != want {
		t.Errorf("settings = %d %s, want 200 %s", w.Code, got, want)
	}
}
