package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// File names must start with their 1-based version (0001_, 0002_, …) so a
// gap or misnamed file fails instead of being applied out of order.
func loadMigrations() ([]string, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	migrations := make([]string, 0, len(entries))
	for i, entry := range entries {
		if want := fmt.Sprintf("%04d_", i+1); !strings.HasPrefix(entry.Name(), want) {
			return nil, fmt.Errorf("migration %q: expected name to start with %q", entry.Name(), want)
		}
		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, string(content))
	}
	return migrations, nil
}

func migrate(db *sql.DB, migrations []string) error {
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}

	if current > len(migrations) {
		return fmt.Errorf("%w: version %d, supported %d", storage.ErrSchemaTooNew, current, len(migrations))
	}

	for i := current; i < len(migrations); i++ {
		if err := applyMigration(db, i+1, migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	return nil
}

func applyMigration(db *sql.DB, version int, migration string) (err error) {
	slog.Info("Applying SQLite migration", "version", version)

	// The pragmas and the transaction must share a connection.
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Warn("Failed to release SQLite connection", "error", err)
		}
	}()

	// With foreign keys on, rebuilding a parent table (DROP TABLE) cascades
	// to its children. The pragma is a no-op inside a transaction.
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	defer func() {
		if _, enableErr := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); enableErr != nil {
			err = errors.Join(err, enableErr)
		}
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Warn("Failed to rollback transaction", "error", err)
		}
	}()

	// The other app may have applied it since migrate read the version
	var current int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	if current >= version {
		return nil
	}

	if _, err := tx.ExecContext(ctx, migration); err != nil {
		return err
	}

	var table, parent string
	var rowID sql.NullInt64
	var fkID int
	checkErr := tx.QueryRowContext(ctx, "PRAGMA foreign_key_check").Scan(&table, &rowID, &parent, &fkID)
	if checkErr == nil {
		return fmt.Errorf("foreign key violation: %s row %d references a missing %s row", table, rowID.Int64, parent)
	}
	if !errors.Is(checkErr, sql.ErrNoRows) {
		return checkErr
	}

	// PRAGMA doesn't accept bound parameters.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return err
	}
	return tx.Commit()
}
