// Package players lets the GM manage the players and their magic links.
package players

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/wolf29f/hushtongue/internal/server/auth"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// Store is the storage of the players the handlers need.
type Store interface {
	ListPlayers(ctx context.Context) ([]storage.User, error)
	CreatePlayer(ctx context.Context, name string, linkHash []byte) (storage.User, error)
	SetPlayerLink(ctx context.Context, id int, linkHash []byte) error
	DeletePlayer(ctx context.Context, id int) error
}

// Handlers serves the players routes.
type Handlers struct {
	publicURL *url.URL // nil when deduced from each request
	store     Store
}

// New creates the handlers on top of store. The magic links point to
// publicURL, or to the address each request was sent to when nil.
func New(publicURL *url.URL, store Store) *Handlers {
	return &Handlers{publicURL: publicURL, store: store}
}

type playerResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// List serves GET /api/players: the players, in creation order.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request, _ storage.User) {
	players, err := h.store.ListPlayers(r.Context())
	if err != nil {
		httpapi.WriteServiceError(w, r, err)
		return
	}
	response := make([]playerResponse, 0, len(players))
	for _, player := range players {
		response = append(response, playerResponse{ID: player.ID, Name: player.Name})
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

type createRequest struct {
	Name string `json:"name"`
}

type createResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Link string `json:"link"`
}

// Create serves POST /api/players: it creates a player and returns their
// magic link, shown this once.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request, _ storage.User) {
	var req createRequest
	if !httpapi.ReadJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httpapi.WriteError(w, http.StatusBadRequest, httpapi.CodeInvalidRequest, "Le nom du joueur est requis.")
		return
	}

	token, linkHash, err := auth.NewToken()
	if err != nil {
		httpapi.WriteInternalError(w, r, err)
		return
	}
	player, err := h.store.CreatePlayer(r.Context(), name, linkHash)
	if err != nil {
		httpapi.WriteServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, createResponse{ID: player.ID, Name: player.Name, Link: h.loginLink(r, token)})
}

type linkResponse struct {
	Link string `json:"link"`
}

// RegenerateLink serves POST /api/players/{id}/link: it replaces the
// player's magic link, ends their sessions and returns the new link, shown
// this once.
func (h *Handlers) RegenerateLink(w http.ResponseWriter, r *http.Request, _ storage.User) {
	id, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	token, linkHash, err := auth.NewToken()
	if err != nil {
		httpapi.WriteInternalError(w, r, err)
		return
	}
	if err := h.store.SetPlayerLink(r.Context(), id, linkHash); err != nil {
		httpapi.WriteServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, linkResponse{Link: h.loginLink(r, token)})
}

// Delete serves DELETE /api/players/{id}: it deletes the player with their
// dictionary, texts and sessions.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request, _ storage.User) {
	id, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	if err := h.store.DeletePlayer(r.Context(), id); err != nil {
		httpapi.WriteServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loginLink returns the magic link of token: the front's page /login/{token}
// on the public URL, or on the address the request was sent to.
func (h *Handlers) loginLink(r *http.Request, token string) string {
	base := h.publicURL
	if base == nil {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		// Set by the reverse proxy, which terminates HTTPS
		if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
			scheme = proto
		}
		base = &url.URL{Scheme: scheme, Host: r.Host}
	}
	return base.JoinPath("login", token).String()
}
