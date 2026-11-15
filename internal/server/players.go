package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type playerResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// handleListPlayers serves GET /api/players: the players, in creation
// order.
func (s *Server) handleListPlayers(w http.ResponseWriter, r *http.Request, _ storage.User) {
	players, err := s.store.ListPlayers(r.Context())
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	response := make([]playerResponse, 0, len(players))
	for _, player := range players {
		response = append(response, playerResponse{ID: player.ID, Name: player.Name})
	}
	writeJSON(w, http.StatusOK, response)
}

type createPlayerRequest struct {
	Name string `json:"name"`
}

type createPlayerResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Link string `json:"link"`
}

// handleCreatePlayer serves POST /api/players: it creates a player and
// returns their magic link, shown this once.
func (s *Server) handleCreatePlayer(w http.ResponseWriter, r *http.Request, _ storage.User) {
	var req createPlayerRequest
	if !readJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "Le nom du joueur est requis.")
		return
	}

	token, linkHash, err := newToken()
	if err != nil {
		s.writeInternalError(w, r, err)
		return
	}
	player, err := s.store.CreatePlayer(r.Context(), name, linkHash)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, createPlayerResponse{ID: player.ID, Name: player.Name, Link: s.loginLink(r, token)})
}

type playerLinkResponse struct {
	Link string `json:"link"`
}

// handleRegeneratePlayerLink serves POST /api/players/{id}/link: it replaces
// the player's magic link, ends their sessions and returns the new link,
// shown this once.
func (s *Server) handleRegeneratePlayerLink(w http.ResponseWriter, r *http.Request, _ storage.User) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	token, linkHash, err := newToken()
	if err != nil {
		s.writeInternalError(w, r, err)
		return
	}
	if err := s.store.SetPlayerLink(r.Context(), id, linkHash); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, playerLinkResponse{Link: s.loginLink(r, token)})
}

// handleDeletePlayer serves DELETE /api/players/{id}: it deletes the player
// with their dictionary, texts and sessions.
func (s *Server) handleDeletePlayer(w http.ResponseWriter, r *http.Request, _ storage.User) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeletePlayer(r.Context(), id); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loginLink returns the magic link of token: the front's page /login/{token}
// on HUSHTONGUE_PUBLIC_URL, or on the address the request was sent to.
func (s *Server) loginLink(r *http.Request, token string) string {
	base := s.cfg.PublicURL
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
