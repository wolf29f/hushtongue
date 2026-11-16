package server

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
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
		{"settings", http.MethodGet, "/api/settings", http.StatusOK, `{"languages":{"source":"Français","con":"Ailo"}}`},
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
			New(testConfig, postgres.New(nil)).routes().ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := w.Body.String(); got != tt.wantBody {
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
			New(testConfig, postgres.New(nil)).routes().ServeHTTP(w, r)

			if tt.denied {
				assertError(t, w, http.StatusForbidden, httpapi.CodeCrossOrigin)
			} else if w.Code == http.StatusForbidden {
				t.Errorf("status = 403, want the request through")
			}
		})
	}
}

var testConfig = config.Config{
	GMUsername:      "mj",
	GMPassword:      "secret",
	SourceLangLabel: "Français",
	ConLangLabel:    "Ailo",
}

// newTestServer returns a server on a fresh database holding the GM.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	store := postgres.New(pgtest.New(t))
	if err := store.SetGM(t.Context(), testConfig.GMUsername); err != nil {
		t.Fatalf("SetGM() failed: %v", err)
	}
	return New(testConfig, store)
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

// These tests go through the routes on Postgres: the wiring of the handlers,
// which the features' tests on fakes can't check.

const sessionCookie = "hushtongue_session"

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
	w := serve(s, http.MethodPost, "/api/login", `{"name": "mj", "password": "secret"}`)
	cookie := sessionCookieOf(w)
	if w.Code != http.StatusOK || cookie == nil {
		t.Fatalf("login = %d %s, want 200 with a cookie", w.Code, w.Body)
	}
	return cookie
}

type player struct {
	ID   int    `json:"id"`
	Link string `json:"link"`
}

// createPlayer creates the player as the GM.
func createPlayer(t *testing.T, s *Server, gm *http.Cookie, name string) player {
	t.Helper()
	w := serve(s, http.MethodPost, "/api/players", `{"name": "`+name+`"}`, gm)
	var created player
	decode(t, w, &created)
	if w.Code != http.StatusCreated {
		t.Fatalf("creating player = %d %s, want 201", w.Code, w.Body)
	}
	return created
}

// linkLogin logs in with the magic link.
func linkLogin(s *Server, link string) *httptest.ResponseRecorder {
	token := link[strings.LastIndex(link, "/")+1:]
	return serve(s, http.MethodPost, "/api/login/link", `{"token": "`+token+`"}`)
}

func TestReady(t *testing.T) {
	if w := serve(newTestServer(t), http.MethodGet, "/api/health/ready", ""); w.Code != http.StatusOK {
		t.Errorf("ready = %d %s, want 200", w.Code, w.Body)
	}
}

func TestPlayersRequireGM(t *testing.T) {
	s := newTestServer(t)
	created := createPlayer(t, s, login(t, s), "Ana")
	playerCookie := sessionCookieOf(linkLogin(s, created.Link))
	path := "/api/players/" + strconv.Itoa(created.ID)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/players"},
		{http.MethodPost, "/api/players"},
		{http.MethodPost, path + "/link"},
		{http.MethodDelete, path},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			assertError(t, serve(s, route.method, route.path, `{"name": "Bob"}`, playerCookie), http.StatusForbidden, httpapi.CodeForbidden)
			assertError(t, serve(s, route.method, route.path, `{"name": "Bob"}`), http.StatusUnauthorized, httpapi.CodeUnauthenticated)
		})
	}
}

// TestPlayerLifecycle follows a player from creation to deletion, through
// their sessions and links.
func TestPlayerLifecycle(t *testing.T) {
	s := newTestServer(t)
	gm := login(t, s)

	if w := serve(s, http.MethodGet, "/api/me", "", gm); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"role":"gm"`) {
		t.Fatalf("GM me = %d %s, want 200 gm", w.Code, w.Body)
	}

	created := createPlayer(t, s, gm, "Ana")
	assertError(t, serve(s, http.MethodPost, "/api/players", `{"name": "mj"}`, gm), http.StatusConflict, httpapi.CodeNameTaken)

	oldSession := sessionCookieOf(linkLogin(s, created.Link))
	if w := serve(s, http.MethodGet, "/api/me", "", oldSession); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"name":"Ana"`) {
		t.Fatalf("player me = %d %s, want 200 Ana", w.Code, w.Body)
	}

	w := serve(s, http.MethodPost, "/api/players/"+strconv.Itoa(created.ID)+"/link", "{}", gm)
	var regenerated player
	decode(t, w, &regenerated)
	if w.Code != http.StatusOK {
		t.Fatalf("regenerate = %d, want 200", w.Code)
	}
	assertError(t, serve(s, http.MethodGet, "/api/me", "", oldSession), http.StatusUnauthorized, httpapi.CodeUnauthenticated)
	assertError(t, linkLogin(s, created.Link), http.StatusUnauthorized, httpapi.CodeInvalidLink)
	newSession := sessionCookieOf(linkLogin(s, regenerated.Link))
	if newSession == nil {
		t.Fatal("login with the new link refused")
	}

	if w := serve(s, http.MethodDelete, "/api/players/"+strconv.Itoa(created.ID), "{}", gm); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", w.Code)
	}
	assertError(t, serve(s, http.MethodGet, "/api/me", "", newSession), http.StatusUnauthorized, httpapi.CodeUnauthenticated)

	if w := serve(s, http.MethodPost, "/api/logout", "{}", gm); w.Code != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", w.Code)
	}
	assertError(t, serve(s, http.MethodGet, "/api/me", "", gm), http.StatusUnauthorized, httpapi.CodeUnauthenticated)
}
