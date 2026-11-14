package server

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/pgtest"
)

const gmLogin = `{"name": "mj", "password": "secret"}`

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
func login(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	w := serve(s, http.MethodPost, "/api/login", gmLogin)
	cookie := sessionCookieOf(w)
	if w.Code != http.StatusOK || cookie == nil {
		t.Fatalf("login = %d %s, want 200 with a cookie", w.Code, w.Body)
	}
	return cookie
}

// expiresIn returns the remaining lifetime of the cookie's session.
func expiresIn(t *testing.T, pool *pgxpool.Pool, cookie *http.Cookie) time.Duration {
	t.Helper()
	var remaining time.Duration
	if err := pool.QueryRow(context.Background(),
		"SELECT expires_at - now() FROM sessions WHERE token_hash = $1", tokenHash(cookie),
	).Scan(&remaining); err != nil {
		t.Fatalf("reading session: %v", err)
	}
	return remaining
}

// setExpiresIn makes the cookie's session expire in remaining.
func setExpiresIn(t *testing.T, pool *pgxpool.Pool, cookie *http.Cookie, remaining time.Duration) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"UPDATE sessions SET expires_at = now() + $2 WHERE token_hash = $1", tokenHash(cookie), remaining,
	); err != nil {
		t.Fatalf("updating session: %v", err)
	}
}

func tokenHash(cookie *http.Cookie) []byte {
	hash := sha256.Sum256([]byte(cookie.Value))
	return hash[:]
}

func TestLogin(t *testing.T) {
	t.Run("logs the GM in", func(t *testing.T) {
		s, pool := newTestServer(t, testConfig)

		w := serve(s, http.MethodPost, "/api/login", gmLogin)

		var body meResponse
		if err := json.UnmarshalRead(w.Body, &body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		want := meResponse{Name: "mj", Role: "gm"}
		if w.Code != http.StatusOK || body != want {
			t.Errorf("login = %d %+v, want 200 %+v", w.Code, body, want)
		}
		cookie := sessionCookieOf(w)
		if cookie == nil {
			t.Fatal("no session cookie")
		}
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" ||
			cookie.MaxAge != int(gmSessionLifetime/time.Second) || len(cookie.Value) < 43 {
			t.Errorf("cookie = %+v, want a 32-byte token, HttpOnly, Secure, SameSite=Lax, for 30 days", cookie)
		}
		if remaining := expiresIn(t, pool, cookie); remaining < gmSessionLifetime-time.Minute {
			t.Errorf("session expires in %v, want 30 days", remaining)
		}
	})

	t.Run("sets an insecure cookie for an http public URL", func(t *testing.T) {
		cfg := testConfig
		cfg.PublicURL = &url.URL{Scheme: "http", Host: "localhost:8080"}
		s, _ := newTestServer(t, cfg)

		if cookie := login(t, s); cookie.Secure {
			t.Error("cookie is Secure, want it not to be")
		}
	})

	tests := []struct {
		name   string
		body   string
		status int
		code   errorCode
	}{
		{"wrong password", `{"name": "mj", "password": "secret2"}`, http.StatusUnauthorized, codeInvalidCredentials},
		{"wrong name", `{"name": "mj2", "password": "secret"}`, http.StatusUnauthorized, codeInvalidCredentials},
		{"empty", `{}`, http.StatusUnauthorized, codeInvalidCredentials},
		{"invalid JSON", `{"name":`, http.StatusBadRequest, codeInvalidRequest},
	}
	for _, tt := range tests {
		t.Run("refuses "+tt.name, func(t *testing.T) {
			s, _ := newTestServer(t, testConfig)

			w := serve(s, http.MethodPost, "/api/login", tt.body)

			assertError(t, w, tt.status, tt.code)
			if sessionCookieOf(w) != nil {
				t.Error("session cookie set")
			}
		})
	}

	t.Run("limits the attempts per IP", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)
		now := time.Now()
		s.loginLimiter = newLimiter(loginAttempts, loginPeriod, func() time.Time { return now })
		attempt := func(forwardedFor string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Forwarded-For", forwardedFor)
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
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
		assertError(t, w, http.StatusTooManyRequests, codeTooManyAttempts)

		if w := attempt("203.0.113.2"); w.Code == http.StatusTooManyRequests {
			t.Error("attempt from another IP refused")
		}
	})
}

func TestMe(t *testing.T) {
	t.Run("returns the session's user", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)

		w := serve(s, http.MethodGet, "/api/me", "", login(t, s))

		var body meResponse
		if err := json.UnmarshalRead(w.Body, &body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		if w.Code != http.StatusOK || body.Name != "mj" || body.Role != "gm" {
			t.Errorf("me = %d %+v, want 200 mj gm", w.Code, body)
		}
	})

	t.Run("requires a session", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)

		assertError(t, serve(s, http.MethodGet, "/api/me", ""), http.StatusUnauthorized, codeUnauthenticated)
	})

	t.Run("refuses an unknown session", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)

		w := serve(s, http.MethodGet, "/api/me", "", &http.Cookie{Name: sessionCookie, Value: "nope"})

		assertError(t, w, http.StatusUnauthorized, codeUnauthenticated)
	})

	t.Run("refuses an expired session", func(t *testing.T) {
		s, pool := newTestServer(t, testConfig)
		cookie := login(t, s)
		setExpiresIn(t, pool, cookie, -time.Second)

		assertError(t, serve(s, http.MethodGet, "/api/me", "", cookie), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestSessionExtension(t *testing.T) {
	t.Run("leaves a session extended less than a day ago", func(t *testing.T) {
		s, pool := newTestServer(t, testConfig)
		cookie := login(t, s)
		setExpiresIn(t, pool, cookie, gmSessionLifetime-23*time.Hour)

		w := serve(s, http.MethodGet, "/api/me", "", cookie)

		if w.Code != http.StatusOK || sessionCookieOf(w) != nil {
			t.Errorf("me = %d with cookie %v, want 200 without cookie", w.Code, sessionCookieOf(w))
		}
		if remaining := expiresIn(t, pool, cookie); remaining > gmSessionLifetime-23*time.Hour {
			t.Errorf("session expires in %v, want it unchanged", remaining)
		}
	})

	tests := []struct {
		name     string
		cookie   func(t *testing.T, s *Server, pool *pgxpool.Pool) *http.Cookie
		lifetime time.Duration
	}{
		{"GM", func(t *testing.T, s *Server, _ *pgxpool.Pool) *http.Cookie {
			return login(t, s)
		}, gmSessionLifetime},
		{"player", func(t *testing.T, s *Server, pool *pgxpool.Pool) *http.Cookie {
			cookie := &http.Cookie{Name: sessionCookie, Value: "player-token"}
			if err := s.store.CreateSession(context.Background(), tokenHash(cookie), pgtest.NewUser(t, pool), time.Hour); err != nil {
				t.Fatalf("CreateSession() failed: %v", err)
			}
			return cookie
		}, playerSessionLifetime},
	}
	for _, tt := range tests {
		t.Run("extends a "+tt.name+" session after a day", func(t *testing.T) {
			s, pool := newTestServer(t, testConfig)
			cookie := tt.cookie(t, s, pool)
			setExpiresIn(t, pool, cookie, tt.lifetime-25*time.Hour)

			w := serve(s, http.MethodGet, "/api/me", "", cookie)

			renewed := sessionCookieOf(w)
			if w.Code != http.StatusOK || renewed == nil || renewed.Value != cookie.Value ||
				renewed.MaxAge != int(tt.lifetime/time.Second) {
				t.Errorf("me = %d with cookie %+v, want 200 renewing the cookie for %v", w.Code, renewed, tt.lifetime)
			}
			if remaining := expiresIn(t, pool, cookie); remaining < tt.lifetime-time.Minute {
				t.Errorf("session expires in %v, want %v", remaining, tt.lifetime)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	t.Run("ends the session", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)
		cookie := login(t, s)

		w := serve(s, http.MethodPost, "/api/logout", "{}", cookie)

		if cleared := sessionCookieOf(w); w.Code != http.StatusNoContent || cleared == nil || cleared.MaxAge >= 0 {
			t.Errorf("logout = %d with cookie %+v, want 204 removing the cookie", w.Code, cleared)
		}
		assertError(t, serve(s, http.MethodGet, "/api/me", "", cookie), http.StatusUnauthorized, codeUnauthenticated)
	})

	t.Run("succeeds without a session", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)

		if w := serve(s, http.MethodPost, "/api/logout", "{}"); w.Code != http.StatusNoContent {
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
