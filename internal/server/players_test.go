package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// createPlayer creates the player as the GM and returns their id and link.
func createPlayer(t *testing.T, s *Server, gm *http.Cookie, name string) (int, string) {
	t.Helper()
	w := serve(s, http.MethodPost, "/api/players", `{"name": "`+name+`"}`, gm)
	var body createPlayerResponse
	decode(t, w, &body)
	if w.Code != http.StatusCreated {
		t.Fatalf("creating player = %d %s, want 201", w.Code, w.Body)
	}
	return body.ID, body.Link
}

// linkToken returns the token of the magic link.
func linkToken(t *testing.T, link string) string {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parsing link %q: %v", link, err)
	}
	token, found := strings.CutPrefix(parsed.Path, "/login/")
	if !found || token == "" {
		t.Fatalf("link %q, want /login/{token}", link)
	}
	return token
}

// linkLogin logs in with the magic link.
func linkLogin(t *testing.T, s *Server, link string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(s, http.MethodPost, "/api/login/link", `{"token": "`+linkToken(t, link)+`"}`)
}

func countSessions(t *testing.T, pool *pgxpool.Pool, userID int) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM sessions WHERE user_id = $1", userID).Scan(&count); err != nil {
		t.Fatalf("counting sessions: %v", err)
	}
	return count
}

func TestCreatePlayer(t *testing.T) {
	t.Run("creates a player with a magic link", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)
		gm := login(t, s)

		w := serve(s, http.MethodPost, "/api/players", `{"name": "  Ana  "}`, gm)

		var body createPlayerResponse
		decode(t, w, &body)
		if w.Code != http.StatusCreated || body.Name != "Ana" || body.ID == 0 {
			t.Errorf("create = %d %+v, want 201 Ana", w.Code, body)
		}
		if !strings.HasPrefix(body.Link, "http://example.com/login/") || len(linkToken(t, body.Link)) < 43 {
			t.Errorf("link = %q, want http://example.com/login/{32-byte token}", body.Link)
		}
	})

	t.Run("refuses an empty name", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)

		w := serve(s, http.MethodPost, "/api/players", `{"name": "  "}`, login(t, s))

		assertError(t, w, http.StatusBadRequest, codeInvalidRequest)
	})

	for _, name := range []string{"Ana", "mj"} {
		t.Run("refuses a taken name: "+name, func(t *testing.T) {
			s, _ := newTestServer(t, testConfig)
			gm := login(t, s)
			createPlayer(t, s, gm, "Ana")

			w := serve(s, http.MethodPost, "/api/players", `{"name": "`+name+`"}`, gm)

			assertError(t, w, http.StatusConflict, codeNameTaken)
		})
	}

}

func TestLoginLink(t *testing.T) {
	tests := []struct {
		name      string
		publicURL *url.URL
		headers   map[string]string
		want      string
	}{
		{"public URL", &url.URL{Scheme: "https", Host: "jdr.example", Path: "/hushtongue/"}, nil,
			"https://jdr.example/hushtongue/login/"},
		{"request address", nil, nil, "http://example.com/login/"},
		{"reverse proxy", nil, map[string]string{"X-Forwarded-Proto": "https"}, "https://example.com/login/"},
		{"invalid forwarded scheme", nil, map[string]string{"X-Forwarded-Proto": "javascript"}, "http://example.com/login/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig
			cfg.PublicURL = tt.publicURL
			s := &Server{cfg: cfg}
			r, err := http.NewRequest(http.MethodPost, "http://example.com/api/players", nil)
			if err != nil {
				t.Fatalf("NewRequest() failed: %v", err)
			}
			for name, value := range tt.headers {
				r.Header.Set(name, value)
			}

			if got := s.loginLink(r, "token"); got != tt.want+"token" {
				t.Errorf("loginLink() = %q, want %q", got, tt.want+"token")
			}
		})
	}
}

func TestListPlayers(t *testing.T) {
	s, _ := newTestServer(t, testConfig)
	gm := login(t, s)

	w := serve(s, http.MethodGet, "/api/players", "", gm)
	if got := strings.TrimSpace(w.Body.String()); w.Code != http.StatusOK || got != "[]" {
		t.Errorf("list = %d %s, want 200 []", w.Code, got)
	}

	zoe, _ := createPlayer(t, s, gm, "Zoé")
	ana, _ := createPlayer(t, s, gm, "Ana")
	w = serve(s, http.MethodGet, "/api/players", "", gm)

	var body []playerResponse
	decode(t, w, &body)
	want := []playerResponse{{ID: zoe, Name: "Zoé"}, {ID: ana, Name: "Ana"}}
	if w.Code != http.StatusOK || len(body) != 2 || body[0] != want[0] || body[1] != want[1] {
		t.Errorf("list = %d %+v, want 200 %+v", w.Code, body, want)
	}
}

func TestLinkLogin(t *testing.T) {
	t.Run("logs the player in", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)
		_, link := createPlayer(t, s, login(t, s), "Ana")

		response := linkLogin(t, s, link)

		cookie := sessionCookieOf(response)
		if response.Code != http.StatusOK || cookie == nil || cookie.MaxAge != int(playerSessionLifetime/time.Second) {
			t.Fatalf("link login = %d with cookie %+v, want 200 with a cookie for a year", response.Code, cookie)
		}
		w := serve(s, http.MethodGet, "/api/me", "", cookie)
		var me meResponse
		decode(t, w, &me)
		if me != (meResponse{Name: "Ana", Role: "player"}) {
			t.Errorf("me = %+v, want Ana player", me)
		}
	})

	t.Run("works again with the same link", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)
		_, link := createPlayer(t, s, login(t, s), "Ana")
		linkLogin(t, s, link)

		if response := linkLogin(t, s, link); response.Code != http.StatusOK {
			t.Errorf("second link login = %d, want 200", response.Code)
		}
	})

	t.Run("refuses an unknown token", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)

		w := serve(s, http.MethodPost, "/api/login/link", `{"token": "nope"}`)

		assertError(t, w, http.StatusUnauthorized, codeInvalidLink)
	})
}

func TestRegeneratePlayerLink(t *testing.T) {
	t.Run("replaces the link and ends the sessions", func(t *testing.T) {
		s, pool := newTestServer(t, testConfig)
		gm := login(t, s)
		id, oldLink := createPlayer(t, s, gm, "Ana")
		linkLogin(t, s, oldLink)

		w := serve(s, http.MethodPost, "/api/players/"+strconv.Itoa(id)+"/link", "{}", gm)

		var body playerLinkResponse
		decode(t, w, &body)
		if w.Code != http.StatusOK || body.Link == oldLink {
			t.Fatalf("regenerate = %d %+v, want 200 with a new link", w.Code, body)
		}
		if count := countSessions(t, pool, id); count != 0 {
			t.Errorf("%d sessions left, want none", count)
		}
		if response := linkLogin(t, s, oldLink); response.Code != http.StatusUnauthorized {
			t.Errorf("login with the old link = %d, want 401", response.Code)
		}
		if response := linkLogin(t, s, body.Link); response.Code != http.StatusOK {
			t.Errorf("login with the new link = %d, want 200", response.Code)
		}
	})

	for _, path := range []string{"/api/players/999/link", "/api/players/abc/link"} {
		t.Run("refuses "+path, func(t *testing.T) {
			s, _ := newTestServer(t, testConfig)

			assertError(t, serve(s, http.MethodPost, path, "{}", login(t, s)), http.StatusNotFound, codeNotFound)
		})
	}

	t.Run("leaves the GM", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)
		gm := login(t, s)
		gmUser, err := s.store.GM(t.Context())
		if err != nil {
			t.Fatalf("GM() failed: %v", err)
		}

		w := serve(s, http.MethodPost, "/api/players/"+strconv.Itoa(gmUser.ID)+"/link", "{}", gm)

		assertError(t, w, http.StatusNotFound, codeNotFound)
	})
}

func TestDeletePlayer(t *testing.T) {
	t.Run("deletes the player and their sessions", func(t *testing.T) {
		s, pool := newTestServer(t, testConfig)
		gm := login(t, s)
		id, link := createPlayer(t, s, gm, "Ana")
		player := sessionCookieOf(linkLogin(t, s, link))

		w := serve(s, http.MethodDelete, "/api/players/"+strconv.Itoa(id), "{}", gm)

		if w.Code != http.StatusNoContent {
			t.Fatalf("delete = %d %s, want 204", w.Code, w.Body)
		}
		if count := countSessions(t, pool, id); count != 0 {
			t.Errorf("%d sessions left, want none", count)
		}
		assertError(t, serve(s, http.MethodGet, "/api/me", "", player), http.StatusUnauthorized, codeUnauthenticated)
	})

	t.Run("refuses an unknown player", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)

		w := serve(s, http.MethodDelete, "/api/players/999", "{}", login(t, s))

		assertError(t, w, http.StatusNotFound, codeNotFound)
	})

	t.Run("requires a JSON Content-Type", func(t *testing.T) {
		s, _ := newTestServer(t, testConfig)
		id, _ := createPlayer(t, s, login(t, s), "Ana")
		gm := login(t, s)

		r := httptest.NewRequest(http.MethodDelete, "/api/players/"+strconv.Itoa(id), nil)
		r.AddCookie(gm)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)

		assertError(t, w, http.StatusUnsupportedMediaType, codeUnsupportedMediaType)
	})
}

func TestPlayersRequireGM(t *testing.T) {
	s, _ := newTestServer(t, testConfig)
	id, link := createPlayer(t, s, login(t, s), "Ana")
	player := sessionCookieOf(linkLogin(t, s, link))
	path := "/api/players/" + strconv.Itoa(id)

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
			assertError(t, serve(s, route.method, route.path, `{"name": "Bob"}`, player), http.StatusForbidden, codeForbidden)
			assertError(t, serve(s, route.method, route.path, `{"name": "Bob"}`), http.StatusUnauthorized, codeUnauthenticated)
		})
	}
}
