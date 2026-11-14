package server

import (
	"context"
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Run(m))
}

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"live", http.MethodGet, "/api/health/live", http.StatusOK, `{"status":"ok"}`},
		{"unknown route", http.MethodGet, "/api/nope", http.StatusNotFound,
			`{"error":"not_found","message":"Ressource introuvable."}`},
		{"wrong method", http.MethodPost, "/api/health/live", http.StatusMethodNotAllowed,
			`{"error":"method_not_allowed","message":"Méthode non autorisée."}`},
		{"mutation without JSON", http.MethodPost, "/api/logout", http.StatusUnsupportedMediaType,
			`{"error":"unsupported_media_type","message":"Le corps de la requête doit être en JSON."}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			New(config.Config{}, postgres.New(nil), slog.Default()).routes().ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := strings.TrimSpace(w.Body.String()); got != tt.wantBody {
				t.Errorf("body = %s, want %s", got, tt.wantBody)
			}
		})
	}
}

func TestCrossOrigin(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		denied  bool
	}{
		{"cross-site mutation", http.MethodPost, "/api/login", map[string]string{"Sec-Fetch-Site": "cross-site"}, true},
		{"same-site mutation", http.MethodPost, "/api/login", map[string]string{"Sec-Fetch-Site": "same-site"}, true},
		{"same-origin mutation", http.MethodPost, "/api/login", map[string]string{"Sec-Fetch-Site": "same-origin"}, false},
		{"other Origin without Sec-Fetch-Site", http.MethodPost, "/api/login",
			map[string]string{"Origin": "https://evil.example"}, true},
		{"same Origin without Sec-Fetch-Site", http.MethodPost, "/api/login",
			map[string]string{"Origin": "http://example.com"}, false},
		{"non-browser mutation", http.MethodPost, "/api/login", nil, false},
		{"cross-site read", http.MethodGet, "/api/health/live", map[string]string{"Sec-Fetch-Site": "cross-site"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Empty credentials are refused before reaching the database
			r := httptest.NewRequest(tt.method, tt.path, strings.NewReader("{}"))
			r.Header.Set("Content-Type", "application/json")
			for name, value := range tt.headers {
				r.Header.Set(name, value)
			}
			w := httptest.NewRecorder()
			New(testConfig, postgres.New(nil), slog.Default()).routes().ServeHTTP(w, r)

			if tt.denied {
				assertError(t, w, http.StatusForbidden, codeCrossOrigin)
			} else if w.Code == http.StatusForbidden {
				t.Errorf("status = 403, want the request through")
			}
		})
	}
}

func TestIsJSON(t *testing.T) {
	tests := map[string]bool{
		"application/json":                  true,
		"application/json; charset=utf-8":   true,
		"Application/JSON":                  true,
		"text/plain":                        false,
		"application/x-www-form-urlencoded": false,
		"":                                  false,
	}
	for contentType, want := range tests {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Content-Type", contentType)
		if got := isJSON(r); got != want {
			t.Errorf("isJSON(%q) = %v, want %v", contentType, got, want)
		}
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, codeNotFound, "Ressource introuvable.")

	var body apiError
	if err := json.UnmarshalRead(w.Body, &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if w.Code != http.StatusNotFound || body != (apiError{Code: codeNotFound, Message: "Ressource introuvable."}) {
		t.Errorf("writeError() = %d %+v", w.Code, body)
	}
}

var testConfig = config.Config{
	GMUsername:      "mj",
	GMPassword:      "secret",
	SourceLangLabel: "Français",
	ConLangLabel:    "Ailo",
}

// newTestServer returns a server with testConfig on a fresh database holding
// the GM, and the database's pool.
func newTestServer(t *testing.T, cfg config.Config) (*Server, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	store := postgres.New(pool)
	if err := store.SetGM(context.Background(), cfg.GMUsername); err != nil {
		t.Fatalf("SetGM() failed: %v", err)
	}
	return New(cfg, store, slog.Default()), pool
}

// serve sends the request to the server's routes, with a JSON Content-Type
// when it has a body.
func serve(s *Server, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	return w
}

// assertError checks the status and code of an error response.
func assertError(t *testing.T, w *httptest.ResponseRecorder, status int, code errorCode) {
	t.Helper()
	var body apiError
	if err := json.UnmarshalRead(w.Body, &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if w.Code != status || body.Code != code {
		t.Errorf("response = %d %s, want %d %s", w.Code, body.Code, status, code)
	}
}

func TestPurgeSessions(t *testing.T) {
	s, pool := newTestServer(t, testConfig)
	setExpiresIn(t, pool, login(t, s), -time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		s.PurgeSessions(ctx)
		close(stopped)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var count int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&count); err != nil {
			t.Fatalf("counting sessions: %v", err)
		}
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired session not purged at start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-stopped
}

func TestReadJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name": "mj"}`))
	w := httptest.NewRecorder()
	r.Body = http.MaxBytesReader(w, r.Body, 4)

	var body loginRequest
	if readJSON(w, r, &body) {
		t.Fatal("readJSON() = true, want false for a body over the limit")
	}
	assertError(t, w, http.StatusRequestEntityTooLarge, codeRequestTooLarge)
}
