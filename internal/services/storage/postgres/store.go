package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// SQLSTATE codes of the constraint violations the store reports.
const (
	foreignKeyViolation = "23503"
	uniqueViolation     = "23505"
)

// Store is the Postgres implementation of storage.Storage.
type Store struct {
	pool *pgxpool.Pool
}

var _ storage.Storage = (*Store)(nil)

// New creates a store on top of pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// querier runs queries on the pool or within a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func isViolation(err error, code string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == code
}

// notFoundIfNoRows reports a missing row as storage.ErrNotFound.
func notFoundIfNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.ErrNotFound
	}
	return err
}

// notFoundIfNoneAffected reports a command that touched no row as
// storage.ErrNotFound.
func notFoundIfNoneAffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// Ping checks that the database answers.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
