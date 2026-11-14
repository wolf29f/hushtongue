package postgres

import (
	"context"
	"time"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// CreateSession stores a session of the user, identified by the hash of its
// token and expiring in lifetime.
func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID int, lifetime time.Duration) error {
	_, err := s.pool.Exec(ctx,
		"INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, now() + $3)",
		tokenHash, userID, lifetime,
	)
	return err
}

// Session returns the unexpired session of the token's hash, or
// storage.ErrNotFound.
func (s *Store) Session(ctx context.Context, tokenHash []byte) (storage.Session, error) {
	var session storage.Session
	err := s.pool.QueryRow(ctx,
		"SELECT u.id, u.name, u.role, s.expires_at - now() FROM sessions s "+
			"JOIN users u ON u.id = s.user_id WHERE s.token_hash = $1 AND s.expires_at > now()",
		tokenHash,
	).Scan(&session.User.ID, &session.User.Name, &session.User.Role, &session.ExpiresIn)
	return session, notFoundIfNoRows(err)
}

// ExtendSession makes the session of the token's hash expire in lifetime.
func (s *Store) ExtendSession(ctx context.Context, tokenHash []byte, lifetime time.Duration) error {
	_, err := s.pool.Exec(ctx, "UPDATE sessions SET expires_at = now() + $2 WHERE token_hash = $1", tokenHash, lifetime)
	return err
}

// DeleteSession deletes the session of the token's hash, if any.
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHash)
	return err
}

// PurgeSessions deletes the expired sessions and returns their number.
func (s *Store) PurgeSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at <= now()")
	return tag.RowsAffected(), err
}
