package settings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wolf29f/hushtongue/internal/config"
)

func TestGet(t *testing.T) {
	h := New(config.Config{SourceLangLabel: "Français", ConLangLabel: "Ailo"})

	w := httptest.NewRecorder()
	h.Get(w, httptest.NewRequest(http.MethodGet, "/api/settings", nil))

	want := `{"languages":{"source":"Français","con":"Ailo"}}`
	if got := w.Body.String(); w.Code != http.StatusOK || got != want {
		t.Errorf("settings = %d %s, want 200 %s", w.Code, got, want)
	}
}
