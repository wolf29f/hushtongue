package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

func TestSessions(t *testing.T) {
	token := []byte("token")

	t.Run("finds the session's user", func(t *testing.T) {
		f := newFixture(t)
		if err := f.store.CreateSession(t.Context(), token, f.user, time.Hour); err != nil {
			t.Fatalf("CreateSession() failed: %v", err)
		}

		session, err := f.store.Session(t.Context(), token)
		if err != nil {
			t.Fatalf("Session() failed: %v", err)
		}
		if session.User.ID != f.user || session.User.Role != storage.RolePlayer {
			t.Errorf("Session().User = %+v, want player %d", session.User, f.user)
		}
		if session.ExpiresIn <= 59*time.Minute || session.ExpiresIn > time.Hour {
			t.Errorf("Session().ExpiresIn = %v, want about an hour", session.ExpiresIn)
		}
	})

	t.Run("ignores an unknown token", func(t *testing.T) {
		f := newFixture(t)

		if _, err := f.store.Session(t.Context(), token); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Session() = %v, want ErrNotFound", err)
		}
	})

	t.Run("ignores an expired session", func(t *testing.T) {
		f := newFixture(t)
		if err := f.store.CreateSession(t.Context(), token, f.user, -time.Second); err != nil {
			t.Fatalf("CreateSession() failed: %v", err)
		}

		if _, err := f.store.Session(t.Context(), token); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Session() = %v, want ErrNotFound", err)
		}
	})

	t.Run("extends a session", func(t *testing.T) {
		f := newFixture(t)
		if err := f.store.CreateSession(t.Context(), token, f.user, time.Hour); err != nil {
			t.Fatalf("CreateSession() failed: %v", err)
		}

		if err := f.store.ExtendSession(t.Context(), token, 48*time.Hour); err != nil {
			t.Fatalf("ExtendSession() failed: %v", err)
		}
		session, err := f.store.Session(t.Context(), token)
		if err != nil || session.ExpiresIn <= 47*time.Hour {
			t.Errorf("Session() = %+v, %v, want it to expire in 48h", session, err)
		}
	})

	t.Run("deletes a session", func(t *testing.T) {
		f := newFixture(t)
		if err := f.store.CreateSession(t.Context(), token, f.user, time.Hour); err != nil {
			t.Fatalf("CreateSession() failed: %v", err)
		}

		if err := f.store.DeleteSession(t.Context(), token); err != nil {
			t.Fatalf("DeleteSession() failed: %v", err)
		}
		if _, err := f.store.Session(t.Context(), token); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Session() = %v, want ErrNotFound", err)
		}
	})

	t.Run("purges the expired sessions only", func(t *testing.T) {
		f := newFixture(t)
		if err := f.store.CreateSession(t.Context(), []byte("expired"), f.user, -time.Second); err != nil {
			t.Fatalf("CreateSession() failed: %v", err)
		}
		if err := f.store.CreateSession(t.Context(), token, f.user, time.Hour); err != nil {
			t.Fatalf("CreateSession() failed: %v", err)
		}

		purged, err := f.store.PurgeSessions(t.Context())
		if err != nil || purged != 1 {
			t.Errorf("PurgeSessions() = %d, %v, want 1", purged, err)
		}
		if _, err := f.store.Session(t.Context(), token); err != nil {
			t.Errorf("Session() of the unexpired session = %v", err)
		}
	})
}
