package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

func TestRequireGM(t *testing.T) {
	store := newFakeStore()
	h := New(testConfig, store)
	addPlayer(store, "link")
	player := sessionCookieOf(serve(h.LinkLogin, `{"token": "link"}`))
	handler := h.RequireGM(func(w http.ResponseWriter, _ *http.Request, user storage.User) {
		httpapi.WriteJSON(w, http.StatusOK, user.Name)
	})

	tests := []struct {
		name   string
		cookie *http.Cookie
		status int
	}{
		{"GM", login(t, h), http.StatusOK},
		{"player", player, http.StatusForbidden},
		{"no session", nil, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w *httptest.ResponseRecorder
			if tt.cookie == nil {
				w = serve(handler, "")
			} else {
				w = serve(handler, "", tt.cookie)
			}
			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestRequireSessionDatabaseDown(t *testing.T) {
	store := newFakeStore()
	h := New(testConfig, store)
	cookie := login(t, h)
	store.err = errors.New("database down")

	assertError(t, me(h, cookie), http.StatusInternalServerError, httpapi.CodeInternal)
}
