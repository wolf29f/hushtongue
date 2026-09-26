package sqlite

import (
	"database/sql"
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/adrg/xdg"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
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

func (dao *DAO) ListWords(language string) ([]storage.Word, error) {
	rows, err := dao.DB.Query("SELECT id, text FROM words WHERE lang = ?", language)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Warn("Failed to close rows", "error", err)
		}
	}()

	words := make([]storage.Word, 0)
	for rows.Next() {
		var word storage.Word
		if err := rows.Scan(&word.ID, &word.Text); err != nil {
			return nil, err
		}
		words = append(words, word)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return words, nil
}

func (dao *DAO) AddWord(language, word string) error {
	normalized := normalize(word)
	_, err := dao.DB.Exec("INSERT INTO words (lang, text, normalized) VALUES (?, ?, ?)", language, word, normalized)
	return err
}

// normalize lowercases word and strips its diacritics ("Éléphant" -> "elephant").
func normalize(word string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	stripped, _, err := transform.String(t, word)
	if err != nil {
		stripped = word
	}
	return strings.ToLower(stripped)
}
