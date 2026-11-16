package players

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/wolf29f/hushtongue/internal/server/auth"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// fakeStore keeps the players in memory. When err is set, every call fails
// with it.
type fakeStore struct {
	players []storage.User
	links   map[int][]byte // by player id
	err     error
}

func newFakeStore() *fakeStore {
	return &fakeStore{links: make(map[int][]byte)}
}

func (s *fakeStore) ListPlayers(context.Context) ([]storage.User, error) {
	return s.players, s.err
}

func (s *fakeStore) CreatePlayer(_ context.Context, name string, linkHash []byte) (storage.User, error) {
	if s.err != nil {
		return storage.User{}, s.err
	}
	for _, player := range s.players {
		if player.Name == name {
			return storage.User{}, storage.ErrNameTaken
		}
	}
	player := storage.User{ID: len(s.players) + 2, Name: name, Role: storage.RolePlayer}
	s.players = append(s.players, player)
	s.links[player.ID] = linkHash
	return player, nil
}

func (s *fakeStore) SetPlayerLink(_ context.Context, id int, linkHash []byte) error {
	if s.err != nil {
		return s.err
	}
	if _, ok := s.links[id]; !ok {
		return storage.ErrNotFound
	}
	s.links[id] = linkHash
	return nil
}

func (s *fakeStore) DeletePlayer(_ context.Context, id int) error {
	if s.err != nil {
		return s.err
	}
	if _, ok := s.links[id]; !ok {
		return storage.ErrNotFound
	}
	delete(s.links, id)
	s.players = slices.DeleteFunc(s.players, func(player storage.User) bool { return player.ID == id })
	return nil
}

// serve sends the request to the players routes, as the GM.
func serve(h *Handlers, method, path, body string) *httptest.ResponseRecorder {
	gm := storage.User{ID: 1, Name: "mj", Role: storage.RoleGM}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/players", func(w http.ResponseWriter, r *http.Request) { h.List(w, r, gm) })
	mux.HandleFunc("POST /api/players", func(w http.ResponseWriter, r *http.Request) { h.Create(w, r, gm) })
	mux.HandleFunc("POST /api/players/{id}/link", func(w http.ResponseWriter, r *http.Request) { h.RegenerateLink(w, r, gm) })
	mux.HandleFunc("DELETE /api/players/{id}", func(w http.ResponseWriter, r *http.Request) { h.Delete(w, r, gm) })

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
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

func TestCreate(t *testing.T) {
	t.Run("creates a player with a magic link", func(t *testing.T) {
		store := newFakeStore()
		h := New(nil, store)

		w := serve(h, http.MethodPost, "/api/players", `{"name": "  Ana  "}`)

		var body createResponse
		decode(t, w, &body)
		if w.Code != http.StatusCreated || body.Name != "Ana" || body.ID != 2 {
			t.Errorf("create = %d %+v, want 201 Ana", w.Code, body)
		}
		if !strings.HasPrefix(body.Link, "http://example.com/login/") || len(linkToken(t, body.Link)) < 43 {
			t.Errorf("link = %q, want http://example.com/login/{32-byte token}", body.Link)
		}
		if !slices.Equal(store.links[2], auth.HashToken(linkToken(t, body.Link))) {
			t.Error("stored link hash isn't the hash of the link's token")
		}
	})

	t.Run("refuses an empty name", func(t *testing.T) {
		h := New(nil, newFakeStore())

		assertError(t, serve(h, http.MethodPost, "/api/players", `{"name": "  "}`), http.StatusBadRequest, httpapi.CodeInvalidRequest)
	})

	t.Run("refuses a taken name", func(t *testing.T) {
		h := New(nil, newFakeStore())
		serve(h, http.MethodPost, "/api/players", `{"name": "Ana"}`)

		assertError(t, serve(h, http.MethodPost, "/api/players", `{"name": "Ana"}`), http.StatusConflict, httpapi.CodeNameTaken)
	})
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
			h := New(tt.publicURL, newFakeStore())
			r := httptest.NewRequest(http.MethodPost, "http://example.com/api/players", nil)
			for name, value := range tt.headers {
				r.Header.Set(name, value)
			}

			if got := h.loginLink(r, "token"); got != tt.want+"token" {
				t.Errorf("loginLink() = %q, want %q", got, tt.want+"token")
			}
		})
	}
}

func TestList(t *testing.T) {
	store := newFakeStore()
	h := New(nil, store)

	w := serve(h, http.MethodGet, "/api/players", "")
	if got := w.Body.String(); w.Code != http.StatusOK || got != "[]" {
		t.Errorf("list = %d %s, want 200 []", w.Code, got)
	}

	serve(h, http.MethodPost, "/api/players", `{"name": "Zoé"}`)
	serve(h, http.MethodPost, "/api/players", `{"name": "Ana"}`)
	w = serve(h, http.MethodGet, "/api/players", "")

	var body []playerResponse
	decode(t, w, &body)
	want := []playerResponse{{ID: 2, Name: "Zoé"}, {ID: 3, Name: "Ana"}}
	if w.Code != http.StatusOK || !slices.Equal(body, want) {
		t.Errorf("list = %d %+v, want 200 %+v", w.Code, body, want)
	}
}

func TestRegenerateLink(t *testing.T) {
	t.Run("replaces the link", func(t *testing.T) {
		store := newFakeStore()
		h := New(nil, store)
		var created createResponse
		decode(t, serve(h, http.MethodPost, "/api/players", `{"name": "Ana"}`), &created)

		w := serve(h, http.MethodPost, "/api/players/2/link", "{}")

		var body linkResponse
		decode(t, w, &body)
		if w.Code != http.StatusOK || body.Link == created.Link {
			t.Fatalf("regenerate = %d %+v, want 200 with a new link", w.Code, body)
		}
		if !slices.Equal(store.links[2], auth.HashToken(linkToken(t, body.Link))) {
			t.Error("stored link hash isn't the hash of the new link's token")
		}
	})

	for _, path := range []string{"/api/players/999/link", "/api/players/abc/link"} {
		t.Run("refuses "+path, func(t *testing.T) {
			h := New(nil, newFakeStore())

			assertError(t, serve(h, http.MethodPost, path, "{}"), http.StatusNotFound, httpapi.CodeNotFound)
		})
	}
}

func TestDelete(t *testing.T) {
	t.Run("deletes the player", func(t *testing.T) {
		store := newFakeStore()
		h := New(nil, store)
		serve(h, http.MethodPost, "/api/players", `{"name": "Ana"}`)

		if w := serve(h, http.MethodDelete, "/api/players/2", ""); w.Code != http.StatusNoContent {
			t.Fatalf("delete = %d %s, want 204", w.Code, w.Body)
		}
		if len(store.players) != 0 {
			t.Errorf("players = %+v, want none", store.players)
		}
	})

	t.Run("refuses an unknown player", func(t *testing.T) {
		h := New(nil, newFakeStore())

		assertError(t, serve(h, http.MethodDelete, "/api/players/999", ""), http.StatusNotFound, httpapi.CodeNotFound)
	})
}

func TestStoreFailure(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("database down")
	h := New(nil, store)

	assertError(t, serve(h, http.MethodGet, "/api/players", ""), http.StatusInternalServerError, httpapi.CodeInternal)
}
