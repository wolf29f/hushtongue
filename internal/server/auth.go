package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

const (
	sessionCookie = "hushtongue_session"

	gmSessionLifetime     = 30 * 24 * time.Hour
	playerSessionLifetime = 365 * 24 * time.Hour
	// sessionExtendAfter spaces out the writes extending a session.
	sessionExtendAfter = 24 * time.Hour

	loginAttempts = 5
	loginPeriod   = time.Minute
)

func sessionLifetime(role string) time.Duration {
	if role == storage.RoleGM {
		return gmSessionLifetime
	}
	return playerSessionLifetime
}

type meResponse struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// handleMe serves GET /api/me: the user's name and role.
func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, user storage.User) {
	writeJSON(w, http.StatusOK, meResponse{Name: user.Name, Role: user.Role})
}

type loginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// handleLogin serves POST /api/login: it logs the GM in, with a limited
// number of attempts per minute and per IP.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if ok, wait := s.loginLimiter.allow(clientIP(r)); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		writeError(w, http.StatusTooManyRequests, codeTooManyAttempts, "Trop de tentatives, réessayez dans une minute.")
		return
	}

	var req loginRequest
	if !readJSON(w, r, &req) {
		return
	}
	// Comparing hashes of equal length keeps the comparison constant-time,
	// and both are compared to tell nothing of which one is wrong
	name, wantName := sha256.Sum256([]byte(req.Name)), sha256.Sum256([]byte(s.cfg.GMUsername))
	password, wantPassword := sha256.Sum256([]byte(req.Password)), sha256.Sum256([]byte(s.cfg.GMPassword))
	if subtle.ConstantTimeCompare(name[:], wantName[:])&subtle.ConstantTimeCompare(password[:], wantPassword[:]) != 1 {
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials, "Nom ou mot de passe incorrect.")
		return
	}

	gm, err := s.store.GM(r.Context())
	if err != nil {
		s.writeInternalError(w, r, err)
		return
	}
	if err := s.createSession(w, r, gm); err != nil {
		s.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Name: gm.Name, Role: gm.Role})
}

// handleLogout serves POST /api/logout: it deletes the current session, if
// any.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := s.store.DeleteSession(r.Context(), hashToken(cookie.Value)); err != nil {
			s.writeInternalError(w, r, err)
			return
		}
	}
	s.setSessionCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

type linkLoginRequest struct {
	Token string `json:"token"`
}

// handleLinkLogin serves POST /api/login/link: it logs the player in with
// the token of their magic link.
func (s *Server) handleLinkLogin(w http.ResponseWriter, r *http.Request) {
	var req linkLoginRequest
	if !readJSON(w, r, &req) {
		return
	}
	player, err := s.store.PlayerByLink(r.Context(), hashToken(req.Token))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, codeInvalidLink, "Lien de connexion invalide, demandez-en un nouveau au MJ.")
			return
		}
		s.writeInternalError(w, r, err)
		return
	}
	if err := s.createSession(w, r, player); err != nil {
		s.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Name: player.Name, Role: player.Role})
}

// newToken returns a random token of 32 bytes, encoded for a cookie or a
// URL, and its hash.
func newToken() (string, []byte, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	return token, hashToken(token), nil
}

// hashToken returns the hash of the token, the form stored in the database
// for the sessions and the magic links.
func hashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

// createSession stores a new session of the user and sets its cookie.
func (s *Server) createSession(w http.ResponseWriter, r *http.Request, user storage.User) error {
	token, tokenHash, err := newToken()
	if err != nil {
		return err
	}
	lifetime := sessionLifetime(user.Role)
	if err := s.store.CreateSession(r.Context(), tokenHash, user.ID, lifetime); err != nil {
		return err
	}
	s.setSessionCookie(w, token, lifetime)
	return nil
}

// setSessionCookie sets the session cookie to token for lifetime, or removes
// it when lifetime is negative.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string, lifetime time.Duration) {
	maxAge := int(lifetime / time.Second)
	if lifetime < 0 {
		maxAge = -1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		// Safari refuses a Secure cookie on http://localhost
		Secure:   s.cfg.PublicURL == nil || s.cfg.PublicURL.Scheme != "http",
		SameSite: http.SameSiteLaxMode,
	})
}

// clientIP returns the last entry of X-Forwarded-For, added by the reverse
// proxy: the client can only forge the ones before. Without the header, it
// returns the address of the connection.
func clientIP(r *http.Request) string {
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		entries := strings.Split(values[len(values)-1], ",")
		if ip := strings.TrimSpace(entries[len(entries)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
