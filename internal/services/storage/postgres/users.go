package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// SetGM creates the GM, or renames them to name. It returns
// storage.ErrNameTaken when a player has that name.
func (s *Store) SetGM(ctx context.Context, name string) error {
	_, err := s.pool.Exec(ctx,
		"INSERT INTO users (name, role) VALUES ($1, 'gm') "+
			"ON CONFLICT (role) WHERE role = 'gm' DO UPDATE SET name = EXCLUDED.name",
		name,
	)
	if isViolation(err, uniqueViolation) {
		return storage.ErrNameTaken
	}
	return err
}

// GM returns the GM, created by SetGM.
func (s *Store) GM(ctx context.Context) (storage.User, error) {
	var user storage.User
	err := s.pool.QueryRow(ctx, "SELECT id, name, role FROM users WHERE role = 'gm'").Scan(&user.ID, &user.Name, &user.Role)
	return user, notFoundIfNoRows(err)
}

// CreatePlayer stores a player whose magic link has the token hash
// linkHash. It returns storage.ErrNameTaken when a user has that name.
func (s *Store) CreatePlayer(ctx context.Context, name string, linkHash []byte) (storage.User, error) {
	player := storage.User{Name: name, Role: storage.RolePlayer}
	err := s.pool.QueryRow(ctx,
		"INSERT INTO users (name, role, login_token_hash) VALUES ($1, 'player', $2) RETURNING id",
		name, linkHash,
	).Scan(&player.ID)
	if err != nil {
		if isViolation(err, uniqueViolation) {
			return storage.User{}, storage.ErrNameTaken
		}
		return storage.User{}, err
	}
	return player, nil
}

// ListPlayers lists the players, in creation order.
func (s *Store) ListPlayers(ctx context.Context) ([]storage.User, error) {
	rows, err := s.pool.Query(ctx, "SELECT id, name, role FROM users WHERE role = 'player' ORDER BY id")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[storage.User])
}

// PlayerByLink returns the player whose magic link has the token hash
// linkHash, or storage.ErrNotFound.
func (s *Store) PlayerByLink(ctx context.Context, linkHash []byte) (storage.User, error) {
	var player storage.User
	err := s.pool.QueryRow(ctx,
		"SELECT id, name, role FROM users WHERE login_token_hash = $1 AND role = 'player'",
		linkHash,
	).Scan(&player.ID, &player.Name, &player.Role)
	return player, notFoundIfNoRows(err)
}

// SetPlayerLink replaces the player's magic link with the one of token hash
// linkHash, and deletes the player's sessions.
func (s *Store) SetPlayerLink(ctx context.Context, id int, linkHash []byte) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := notFoundIfNoneAffected(tx.Exec(ctx,
			"UPDATE users SET login_token_hash = $1 WHERE id = $2 AND role = 'player'",
			linkHash, id,
		)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", id)
		return err
	})
}

// DeletePlayer deletes the player and, by cascade, their dictionary, texts
// and sessions.
func (s *Store) DeletePlayer(ctx context.Context, id int) error {
	return notFoundIfNoneAffected(s.pool.Exec(ctx, "DELETE FROM users WHERE id = $1 AND role = 'player'", id))
}
