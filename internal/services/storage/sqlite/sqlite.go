package sqlite

import (
	"database/sql"
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	_ "modernc.org/sqlite"
)

type DAO struct {
	DB *sql.DB
}

var _ storage.Storage = (*DAO)(nil)

//go:embed migration.sql
var migration string

// Load opens a connection to the SQLite database at the given path and returns a DAO.
// If the path is empty, it defaults to the standard location in the user's XDG data home.
func Load(path string) (*DAO, error) {

	if path == "" {
		path = filepath.Join(xdg.DataHome, "hushtongue", "hushtongue.db")
	}

	slog.Info("Opening SQLite database", "path", path)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// SQLite doesn't support concurrent writers anyway; pinning the pool to a
	// single connection lets a one-time PRAGMA below apply to every query
	// instead of only whichever connection happened to run it.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		if err := db.Close(); err != nil {
			slog.Warn("Failed to close SQLite database after error", "error", err)
		}
		return nil, err
	}

	slog.Info("SQLite database opened successfully", "path", path)
	return &DAO{DB: db}, nil
}

func (dao *DAO) Init() error {
	slog.Info("Initializing SQLite database")
	_, err := dao.DB.Exec(migration)
	return err
}

func (dao *DAO) Close() error {
	return dao.DB.Close()
}
