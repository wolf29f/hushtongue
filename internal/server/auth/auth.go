// Package auth logs the users in and out.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// SessionCookie is the name of the session cookie.
const SessionCookie = "hushtongue_session"

const (
	gmSessionLifetime     = 30 * 24 * time.Hour
	playerSessionLifetime = 365 * 24 * time.Hour

	loginAttempts = 5
	loginPeriod   = time.Minute

	purgeInterval = 24 * time.Hour
)

// Store is the storage of the users and sessions the handlers need.
type Store interface {
	GM(ctx context.Context) (storage.User, error)
	PlayerByLink(ctx context.Context, linkHash []byte) (storage.User, error)
	CreateSession(ctx context.Context, tokenHash []byte, userID int, lifetime time.Duration) error
	DeleteSession(ctx context.Context, tokenHash []byte) error
	PurgeSessions(ctx context.Context) (int64, error)
}

// Handlers serves the authentication routes.
type Handlers struct {
	gmName       string
	gmPassword   string
	secureCookie bool
	store        Store
	loginLimiter *limiter
}

// New creates the handlers on top of store.
func New(cfg config.Config, store Store) *Handlers {
	return &Handlers{
		gmName:       cfg.GMUsername,
		gmPassword:   cfg.GMPassword,
		secureCookie: cfg.SecureCookies(),
		store:        store,
		loginLimiter: newLimiter(loginAttempts, loginPeriod),
	}
}

// PurgeSessions deletes the expired sessions now, then once a day until ctx
// is done.
func (h *Handlers) PurgeSessions(ctx context.Context) {
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()
	for {
		purged, err := h.store.PurgeSessions(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("Purging sessions failed", "error", err)
		} else if purged > 0 {
			slog.Info("Purged expired sessions", "count", purged)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// SessionLifetime returns how long a session of a user of role lasts.
func SessionLifetime(role string) time.Duration {
	if role == storage.RoleGM {
		return gmSessionLifetime
	}
	return playerSessionLifetime
}

type meResponse struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// Me serves GET /api/me: the user's name and role.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	user := httpapi.User(r.Context())
	httpapi.WriteJSON(w, http.StatusOK, meResponse{Name: user.Name, Role: user.Role})
}

type loginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// Login serves POST /api/login: it logs the GM in, with a limited number of
// attempts per minute and per IP.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if ok, wait := h.loginLimiter.allow(clientIP(r)); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		httpapi.WriteError(w, http.StatusTooManyRequests, httpapi.CodeTooManyAttempts, "Trop de tentatives, réessayez dans une minute.")
		return
	}

	var req loginRequest
	if !httpapi.ReadJSON(w, r, &req) {
		return
	}
	// Comparing hashes of equal length keeps the comparison constant-time,
	// and both are compared to tell nothing of which one is wrong
	name, wantName := sha256.Sum256([]byte(req.Name)), sha256.Sum256([]byte(h.gmName))
	password, wantPassword := sha256.Sum256([]byte(req.Password)), sha256.Sum256([]byte(h.gmPassword))
	if subtle.ConstantTimeCompare(name[:], wantName[:])&subtle.ConstantTimeCompare(password[:], wantPassword[:]) != 1 {
		httpapi.WriteError(w, http.StatusUnauthorized, httpapi.CodeInvalidCredentials, "Nom ou mot de passe incorrect.")
		return
	}

	gm, err := h.store.GM(r.Context())
	if err != nil {
		httpapi.WriteInternalError(w, r, err)
		return
	}
	if err := h.createSession(w, r, gm); err != nil {
		httpapi.WriteInternalError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, meResponse{Name: gm.Name, Role: gm.Role})
}

// Logout serves POST /api/logout: it deletes the current session, if any.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookie); err == nil {
		if err := h.store.DeleteSession(r.Context(), HashToken(cookie.Value)); err != nil {
			httpapi.WriteInternalError(w, r, err)
			return
		}
	}
	SetSessionCookie(w, "", -1, h.secureCookie)
	w.WriteHeader(http.StatusNoContent)
}

type linkLoginRequest struct {
	Token string `json:"token"`
}

// LinkLogin serves POST /api/login/link: it logs the player in with the token
// of their magic link.
func (h *Handlers) LinkLogin(w http.ResponseWriter, r *http.Request) {
	var req linkLoginRequest
	if !httpapi.ReadJSON(w, r, &req) {
		return
	}
	player, err := h.store.PlayerByLink(r.Context(), HashToken(req.Token))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			httpapi.WriteError(w, http.StatusUnauthorized, httpapi.CodeInvalidLink, "Lien de connexion invalide, demandez-en un nouveau au MJ.")
			return
		}
		httpapi.WriteInternalError(w, r, err)
		return
	}
	if err := h.createSession(w, r, player); err != nil {
		httpapi.WriteInternalError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, meResponse{Name: player.Name, Role: player.Role})
}

// NewToken returns a random token of 32 bytes, encoded for a cookie or a
// URL, and its hash.
func NewToken() (string, []byte, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	return token, HashToken(token), nil
}

// HashToken returns the hash of the token, the form stored in the database
// for the sessions and the magic links.
func HashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

// createSession stores a new session of the user and sets its cookie.
func (h *Handlers) createSession(w http.ResponseWriter, r *http.Request, user storage.User) error {
	token, tokenHash, err := NewToken()
	if err != nil {
		return err
	}
	lifetime := SessionLifetime(user.Role)
	if err := h.store.CreateSession(r.Context(), tokenHash, user.ID, lifetime); err != nil {
		return err
	}
	SetSessionCookie(w, token, lifetime, h.secureCookie)
	return nil
}

// SetSessionCookie sets the session cookie to token for lifetime, or removes
// it when lifetime is negative.
func SetSessionCookie(w http.ResponseWriter, token string, lifetime time.Duration, secure bool) {
	maxAge := int(lifetime / time.Second)
	if lifetime < 0 {
		maxAge = -1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
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
