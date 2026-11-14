// Package postgres stores hushtongue's data in PostgreSQL.
package postgres

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects to the database at url and applies the pending migrations.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate applies the pending migrations, under an advisory lock so that two
// servers starting together don't apply one twice.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}

	// Closing this *sql.DB leaves the pool open
	db := stdlib.OpenDBFromPool(pool)
	defer func() {
		if err := db.Close(); err != nil {
			slog.Warn("Failed to close migration connection", "error", err)
		}
	}()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	for _, result := range results {
		slog.Info("Applied migration", "version", result.Source.Version, "duration", result.Duration.String())
	}
	return nil
}
