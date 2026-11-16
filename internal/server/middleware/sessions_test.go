package middleware

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/server/auth"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

var (
	gm     = storage.User{ID: 1, Name: "mj", Role: storage.RoleGM}
	player = storage.User{ID: 2, Name: "Ana", Role: storage.RolePlayer}

	gmSessionLifetime     = auth.SessionLifetime(storage.RoleGM)
	playerSessionLifetime = auth.SessionLifetime(storage.RolePlayer)
)

// fakeStore keeps the sessions in memory, by token hash. When err is set,
// every call fails with it.
type fakeStore struct {
	sessions map[string]*storage.Session
	err      error
}

func (s *fakeStore) Session(_ context.Context, tokenHash []byte) (storage.Session, error) {
	if s.err != nil {
		return storage.Session{}, s.err
	}
	session, ok := s.sessions[string(tokenHash)]
	if !ok || session.ExpiresIn <= 0 {
		return storage.Session{}, storage.ErrNotFound
	}
	return *session, nil
}

func (s *fakeStore) ExtendSession(_ context.Context, tokenHash []byte, lifetime time.Duration) error {
	if s.err != nil {
		return s.err
	}
	s.sessions[string(tokenHash)].ExpiresIn = lifetime
	return nil
}

// newSession returns a store holding a session of user expiring in
// expiresIn, and the session's cookie.
func newSession(user storage.User, expiresIn time.Duration) (*fakeStore, *http.Cookie) {
	cookie := &http.Cookie{Name: auth.SessionCookie, Value: "token"}
	store := &fakeStore{sessions: map[string]*storage.Session{
		string(auth.HashToken(cookie.Value)): {User: user, ExpiresIn: expiresIn},
	}}
	return store, cookie
}

// serve sends a request with the cookies through middleware, to a handler
// answering with the name of the context's user.
func serve(middleware func(http.Handler) http.Handler, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, httpapi.User(r.Context()).Name)
	})).ServeHTTP(w, r)
	return w
}

func assertError(t *testing.T, w *httptest.ResponseRecorder, status int, code httpapi.ErrorCode) {
	t.Helper()
	var body httpapi.Error
	if err := json.UnmarshalRead(w.Body, &body); err != nil {
		t.Fatalf("decoding body %s: %v", w.Body, err)
	}
	if w.Code != status || body.Code != code {
		t.Errorf("response = %d %s, want %d %s", w.Code, body.Code, status, code)
	}
}

// sessionCookieOf returns the session cookie set by the response, or nil.
func sessionCookieOf(w *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == auth.SessionCookie {
			return cookie
		}
	}
	return nil
}

func TestRequireSession(t *testing.T) {
	t.Run("passes the session's user", func(t *testing.T) {
		store, cookie := newSession(gm, time.Hour)

		w := serve(New(config.Config{}, store).RequireSession, cookie)

		if got := w.Body.String(); w.Code != http.StatusOK || got != `"mj"` {
			t.Errorf("response = %d %s, want 200 mj", w.Code, got)
		}
	})

	t.Run("requires a session", func(t *testing.T) {
		store, _ := newSession(gm, time.Hour)

		assertError(t, serve(New(config.Config{}, store).RequireSession), http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	})

	t.Run("refuses an unknown session", func(t *testing.T) {
		store, _ := newSession(gm, time.Hour)

		w := serve(New(config.Config{}, store).RequireSession, &http.Cookie{Name: auth.SessionCookie, Value: "nope"})

		assertError(t, w, http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	})

	t.Run("refuses an expired session", func(t *testing.T) {
		store, cookie := newSession(gm, -time.Second)

		w := serve(New(config.Config{}, store).RequireSession, cookie)

		assertError(t, w, http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	})
}

func TestSessionExtension(t *testing.T) {
	t.Run("leaves a session extended less than a day ago", func(t *testing.T) {
		store, cookie := newSession(gm, gmSessionLifetime-23*time.Hour)

		w := serve(New(config.Config{}, store).RequireSession, cookie)

		if w.Code != http.StatusOK || sessionCookieOf(w) != nil {
			t.Errorf("response = %d with cookie %v, want 200 without cookie", w.Code, sessionCookieOf(w))
		}
		if remaining := store.sessions[string(auth.HashToken(cookie.Value))].ExpiresIn; remaining != gmSessionLifetime-23*time.Hour {
			t.Errorf("session expires in %v, want it unchanged", remaining)
		}
	})

	t.Run("extends a GM session after a day", func(t *testing.T) {
		store, cookie := newSession(gm, gmSessionLifetime-25*time.Hour)

		w := serve(New(config.Config{}, store).RequireSession, cookie)

		renewed := sessionCookieOf(w)
		if w.Code != http.StatusOK || renewed == nil || renewed.Value != cookie.Value || !renewed.Secure ||
			renewed.MaxAge != int(gmSessionLifetime/time.Second) {
			t.Errorf("response = %d with cookie %+v, want 200 renewing the Secure cookie for 30 days", w.Code, renewed)
		}
		if remaining := store.sessions[string(auth.HashToken(cookie.Value))].ExpiresIn; remaining != gmSessionLifetime {
			t.Errorf("session expires in %v, want 30 days", remaining)
		}
	})

	t.Run("extends a player session after a day", func(t *testing.T) {
		store, cookie := newSession(player, playerSessionLifetime-25*time.Hour)

		w := serve(New(config.Config{}, store).RequireSession, cookie)

		renewed := sessionCookieOf(w)
		if w.Code != http.StatusOK || renewed == nil || renewed.Value != cookie.Value ||
			renewed.MaxAge != int(playerSessionLifetime/time.Second) {
			t.Errorf("response = %d with cookie %+v, want 200 renewing the cookie for a year", w.Code, renewed)
		}
		if remaining := store.sessions[string(auth.HashToken(cookie.Value))].ExpiresIn; remaining != playerSessionLifetime {
			t.Errorf("session expires in %v, want a year", remaining)
		}
	})

	t.Run("renews an insecure cookie for an http public URL", func(t *testing.T) {
		store, cookie := newSession(gm, gmSessionLifetime-25*time.Hour)
		cfg := config.Config{PublicURL: &url.URL{Scheme: "http", Host: "localhost:8080"}}

		w := serve(New(cfg, store).RequireSession, cookie)

		if renewed := sessionCookieOf(w); renewed == nil || renewed.Secure {
			t.Errorf("renewed cookie = %+v, want it not Secure", renewed)
		}
	})
}

func TestRequireGM(t *testing.T) {
	tests := []struct {
		name   string
		user   storage.User
		status int
	}{
		{"GM", gm, http.StatusOK},
		{"player", player, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cookie := newSession(tt.user, time.Hour)

			if w := serve(New(config.Config{}, store).RequireGM, cookie); w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}

	t.Run("no session", func(t *testing.T) {
		store, _ := newSession(gm, time.Hour)

		if w := serve(New(config.Config{}, store).RequireGM); w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}

func TestRequireSessionDatabaseDown(t *testing.T) {
	store, cookie := newSession(gm, time.Hour)
	store.err = errors.New("database down")

	assertError(t, serve(New(config.Config{}, store).RequireSession, cookie), http.StatusInternalServerError, httpapi.CodeInternal)
}
