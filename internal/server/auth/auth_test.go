package auth

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

const gmLogin = `{"name": "mj", "password": "secret"}`

var testConfig = config.Config{GMUsername: "mj", GMPassword: "secret"}

// serve sends a request with the body and cookies to handler.
func serve(handler http.HandlerFunc, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.UnmarshalRead(w.Body, v); err != nil {
		t.Fatalf("decoding body %s: %v", w.Body, err)
	}
}

func assertError(t *testing.T, w *httptest.ResponseRecorder, status int, code httpapi.ErrorCode) {
	t.Helper()
	var body httpapi.Error
	decode(t, w, &body)
	if w.Code != status || body.Code != code {
		t.Errorf("response = %d %s, want %d %s", w.Code, body.Code, status, code)
	}
}

// sessionCookieOf returns the session cookie set by the response, or nil.
func sessionCookieOf(w *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == sessionCookie {
			return cookie
		}
	}
	return nil
}

// login logs the GM in and returns the session cookie.
func login(t *testing.T, h *Handlers) *http.Cookie {
	t.Helper()
	w := serve(h.Login, gmLogin)
	cookie := sessionCookieOf(w)
	if w.Code != http.StatusOK || cookie == nil {
		t.Fatalf("login = %d %s, want 200 with a cookie", w.Code, w.Body)
	}
	return cookie
}

// addPlayer adds a player with the magic link token and returns them.
func addPlayer(store *fakeStore, token string) storage.User {
	player := storage.User{ID: 2, Name: "Ana", Role: storage.RolePlayer}
	store.links[string(HashToken(token))] = player
	return player
}

// session returns the stored session of the cookie.
func session(t *testing.T, store *fakeStore, cookie *http.Cookie) *fakeSession {
	t.Helper()
	stored, ok := store.sessions[string(HashToken(cookie.Value))]
	if !ok {
		t.Fatalf("no session stored for cookie %q", cookie.Value)
	}
	return stored
}

// me answers GET /api/me.
func me(h *Handlers, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	return serve(h.RequireSession(h.Me), "", cookies...)
}

func TestLogin(t *testing.T) {
	t.Run("logs the GM in", func(t *testing.T) {
		store := newFakeStore()
		h := New(testConfig, store)

		w := serve(h.Login, gmLogin)

		var body meResponse
		decode(t, w, &body)
		if w.Code != http.StatusOK || body != (meResponse{Name: "mj", Role: "gm"}) {
			t.Errorf("login = %d %+v, want 200 mj gm", w.Code, body)
		}
		cookie := sessionCookieOf(w)
		if cookie == nil {
			t.Fatal("no session cookie")
		}
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" ||
			cookie.MaxAge != int(gmSessionLifetime/time.Second) || len(cookie.Value) < 43 {
			t.Errorf("cookie = %+v, want a 32-byte token, HttpOnly, Secure, SameSite=Lax, for 30 days", cookie)
		}
		if stored := session(t, store, cookie); stored.user.ID != 1 || stored.expiresIn != gmSessionLifetime {
			t.Errorf("session = %+v, want the GM's for 30 days", stored)
		}
	})

	t.Run("sets an insecure cookie for an http public URL", func(t *testing.T) {
		cfg := testConfig
		cfg.PublicURL = &url.URL{Scheme: "http", Host: "localhost:8080"}
		h := New(cfg, newFakeStore())

		if cookie := login(t, h); cookie.Secure {
			t.Error("cookie is Secure, want it not to be")
		}
	})

	tests := []struct {
		name   string
		body   string
		status int
		code   httpapi.ErrorCode
	}{
		{"wrong password", `{"name": "mj", "password": "secret2"}`, http.StatusUnauthorized, httpapi.CodeInvalidCredentials},
		{"wrong name", `{"name": "mj2", "password": "secret"}`, http.StatusUnauthorized, httpapi.CodeInvalidCredentials},
		{"empty", `{}`, http.StatusUnauthorized, httpapi.CodeInvalidCredentials},
		{"invalid JSON", `{"name":`, http.StatusBadRequest, httpapi.CodeInvalidRequest},
	}
	for _, tt := range tests {
		t.Run("refuses "+tt.name, func(t *testing.T) {
			store := newFakeStore()
			h := New(testConfig, store)

			w := serve(h.Login, tt.body)

			assertError(t, w, tt.status, tt.code)
			if sessionCookieOf(w) != nil || len(store.sessions) != 0 {
				t.Error("session created")
			}
		})
	}

	t.Run("limits the attempts per IP", func(t *testing.T) {
		h := New(testConfig, newFakeStore())
		now := time.Now()
		h.loginLimiter = newLimiter(loginAttempts, loginPeriod, func() time.Time { return now })
		attempt := func(forwardedFor string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
			r.Header.Set("X-Forwarded-For", forwardedFor)
			w := httptest.NewRecorder()
			h.Login(w, r)
			return w
		}

		for i := range loginAttempts {
			// Forging the first entry doesn't give more attempts
			if w := attempt(string(rune('a'+i)) + ", 203.0.113.1"); w.Code == http.StatusTooManyRequests {
				t.Fatalf("attempt %d refused", i+1)
			}
		}
		w := attempt("203.0.113.1")
		if retry := w.Header().Get("Retry-After"); retry != "60" {
			t.Errorf("Retry-After = %q, want 60", retry)
		}
		assertError(t, w, http.StatusTooManyRequests, httpapi.CodeTooManyAttempts)

		if w := attempt("203.0.113.2"); w.Code == http.StatusTooManyRequests {
			t.Error("attempt from another IP refused")
		}
	})
}

func TestMe(t *testing.T) {
	t.Run("returns the session's user", func(t *testing.T) {
		h := New(testConfig, newFakeStore())

		w := me(h, login(t, h))

		var body meResponse
		decode(t, w, &body)
		if w.Code != http.StatusOK || body != (meResponse{Name: "mj", Role: "gm"}) {
			t.Errorf("me = %d %+v, want 200 mj gm", w.Code, body)
		}
	})

	t.Run("requires a session", func(t *testing.T) {
		h := New(testConfig, newFakeStore())

		assertError(t, me(h), http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	})

	t.Run("refuses an unknown session", func(t *testing.T) {
		h := New(testConfig, newFakeStore())

		w := me(h, &http.Cookie{Name: sessionCookie, Value: "nope"})

		assertError(t, w, http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	})

	t.Run("refuses an expired session", func(t *testing.T) {
		store := newFakeStore()
		h := New(testConfig, store)
		cookie := login(t, h)
		session(t, store, cookie).expiresIn = -time.Second

		assertError(t, me(h, cookie), http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	})
}

func TestSessionExtension(t *testing.T) {
	t.Run("leaves a session extended less than a day ago", func(t *testing.T) {
		store := newFakeStore()
		h := New(testConfig, store)
		cookie := login(t, h)
		session(t, store, cookie).expiresIn = gmSessionLifetime - 23*time.Hour

		w := me(h, cookie)

		if w.Code != http.StatusOK || sessionCookieOf(w) != nil {
			t.Errorf("me = %d with cookie %v, want 200 without cookie", w.Code, sessionCookieOf(w))
		}
		if remaining := session(t, store, cookie).expiresIn; remaining != gmSessionLifetime-23*time.Hour {
			t.Errorf("session expires in %v, want it unchanged", remaining)
		}
	})

	t.Run("extends a GM session after a day", func(t *testing.T) {
		store := newFakeStore()
		h := New(testConfig, store)
		cookie := login(t, h)
		session(t, store, cookie).expiresIn = gmSessionLifetime - 25*time.Hour

		w := me(h, cookie)

		renewed := sessionCookieOf(w)
		if w.Code != http.StatusOK || renewed == nil || renewed.Value != cookie.Value ||
			renewed.MaxAge != int(gmSessionLifetime/time.Second) {
			t.Errorf("me = %d with cookie %+v, want 200 renewing the cookie for 30 days", w.Code, renewed)
		}
		if remaining := session(t, store, cookie).expiresIn; remaining != gmSessionLifetime {
			t.Errorf("session expires in %v, want 30 days", remaining)
		}
	})

	t.Run("extends a player session after a day", func(t *testing.T) {
		store := newFakeStore()
		h := New(testConfig, store)
		addPlayer(store, "link")
		cookie := sessionCookieOf(serve(h.LinkLogin, `{"token": "link"}`))
		session(t, store, cookie).expiresIn = playerSessionLifetime - 25*time.Hour

		w := me(h, cookie)

		renewed := sessionCookieOf(w)
		if w.Code != http.StatusOK || renewed == nil || renewed.Value != cookie.Value ||
			renewed.MaxAge != int(playerSessionLifetime/time.Second) {
			t.Errorf("me = %d with cookie %+v, want 200 renewing the cookie for a year", w.Code, renewed)
		}
		if remaining := session(t, store, cookie).expiresIn; remaining != playerSessionLifetime {
			t.Errorf("session expires in %v, want a year", remaining)
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("ends the session", func(t *testing.T) {
		h := New(testConfig, newFakeStore())
		cookie := login(t, h)

		w := serve(h.Logout, "", cookie)

		if cleared := sessionCookieOf(w); w.Code != http.StatusNoContent || cleared == nil || cleared.MaxAge >= 0 {
			t.Errorf("logout = %d with cookie %+v, want 204 removing the cookie", w.Code, cleared)
		}
		assertError(t, me(h, cookie), http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	})

	t.Run("succeeds without a session", func(t *testing.T) {
		h := New(testConfig, newFakeStore())

		if w := serve(h.Logout, ""); w.Code != http.StatusNoContent {
			t.Errorf("logout = %d, want 204", w.Code)
		}
	})
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name         string
		forwardedFor []string
		want         string
	}{
		{"connection address", nil, "192.0.2.1"},
		{"single entry", []string{"203.0.113.1"}, "203.0.113.1"},
		{"last entry", []string{"198.51.100.1, 203.0.113.1"}, "203.0.113.1"},
		{"last header", []string{"198.51.100.1", "203.0.113.1 "}, "203.0.113.1"},
		{"empty last entry", []string{"198.51.100.1,"}, "192.0.2.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
			r.RemoteAddr = "192.0.2.1:1234"
			for _, value := range tt.forwardedFor {
				r.Header.Add("X-Forwarded-For", value)
			}
			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLinkLogin(t *testing.T) {
	t.Run("logs the player in", func(t *testing.T) {
		store := newFakeStore()
		h := New(testConfig, store)
		player := addPlayer(store, "link")

		w := serve(h.LinkLogin, `{"token": "link"}`)

		var body meResponse
		decode(t, w, &body)
		cookie := sessionCookieOf(w)
		if w.Code != http.StatusOK || body != (meResponse{Name: "Ana", Role: "player"}) ||
			cookie == nil || cookie.MaxAge != int(playerSessionLifetime/time.Second) {
			t.Fatalf("link login = %d %+v with cookie %+v, want 200 Ana with a cookie for a year", w.Code, body, cookie)
		}
		if stored := session(t, store, cookie); stored.user != player || stored.expiresIn != playerSessionLifetime {
			t.Errorf("session = %+v, want Ana's for a year", stored)
		}
	})

	t.Run("works again with the same link", func(t *testing.T) {
		store := newFakeStore()
		h := New(testConfig, store)
		addPlayer(store, "link")
		serve(h.LinkLogin, `{"token": "link"}`)

		if w := serve(h.LinkLogin, `{"token": "link"}`); w.Code != http.StatusOK {
			t.Errorf("second link login = %d, want 200", w.Code)
		}
	})

	t.Run("refuses an unknown token", func(t *testing.T) {
		h := New(testConfig, newFakeStore())

		assertError(t, serve(h.LinkLogin, `{"token": "nope"}`), http.StatusUnauthorized, httpapi.CodeInvalidLink)
	})
}

func TestPurgeSessions(t *testing.T) {
	store := newFakeStore()
	h := New(testConfig, store)
	session(t, store, login(t, h)).expiresIn = -time.Second
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		h.PurgeSessions(ctx)
		close(stopped)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for store.sessionCount() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("expired session not purged at start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-stopped
}
