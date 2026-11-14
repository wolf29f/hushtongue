package postgres

import (
	"context"

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
