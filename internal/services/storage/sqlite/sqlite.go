package sqlite

import (
	"database/sql"
	_ "embed"
	"errors"
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

func (dao *DAO) SaveWord(word storage.WordDetails) (storage.WordDetails, error) {
	word.Normalized = normalize(word.Text)

	row := dao.DB.QueryRow(
		"UPDATE words SET lang = ?, text = ?, normalized = ?, kind = ? WHERE id = ? "+
			"RETURNING id, lang, text, normalized, kind",
		word.Language, word.Text, word.Normalized, word.Kind, word.ID,
	)

	if err := row.Scan(&word.ID, &word.Language, &word.Text, &word.Normalized, &word.Kind); err != nil {
		return storage.WordDetails{}, err
	}

	return word, nil
}

// DeleteWord deletes the word and, by cascade, its translations.
func (dao *DAO) DeleteWord(id int) error {
	_, err := dao.DB.Exec("DELETE FROM words WHERE id = ?", id)
	return err
}

// ChangeWordKind sets the word's kind and deletes its translations.
func (dao *DAO) ChangeWordKind(id int, kind string) (storage.WordDetails, error) {
	tx, err := dao.DB.Begin()
	if err != nil {
		return storage.WordDetails{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Warn("Failed to rollback transaction", "error", err)
		}
	}()

	if _, err := tx.Exec(
		"DELETE FROM translations WHERE source_word_id = ? OR con_word_id = ?", id, id,
	); err != nil {
		return storage.WordDetails{}, err
	}

	var word storage.WordDetails
	row := tx.QueryRow(
		"UPDATE words SET kind = ? WHERE id = ? RETURNING id, lang, text, normalized, kind",
		kind, id,
	)
	if err := row.Scan(&word.ID, &word.Language, &word.Text, &word.Normalized, &word.Kind); err != nil {
		return storage.WordDetails{}, err
	}

	if err := tx.Commit(); err != nil {
		return storage.WordDetails{}, err
	}
	return word, nil
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

func (dao *DAO) GetWord(id int) (storage.WordDetails, error) {
	var word storage.WordDetails
	err := dao.DB.QueryRow("SELECT id, lang, text, normalized, kind FROM words WHERE id = ?", id).Scan(&word.ID, &word.Language, &word.Text, &word.Normalized, &word.Kind)
	if err != nil {
		return storage.WordDetails{}, err
	}
	return word, nil
}

// ListTranslations lists the word's translation links, in insertion order.
func (dao *DAO) ListTranslations(wordID int) ([]storage.Translation, error) {
	word, err := dao.GetWord(wordID)
	if err != nil {
		return nil, err
	}

	query := "SELECT t.id, w.id, w.text FROM translations t " +
		"JOIN words w ON w.id = t.con_word_id " +
		"WHERE t.source_word_id = ? ORDER BY t.id"
	if word.Language == storage.LangCon {
		query = "SELECT t.id, w.id, w.text FROM translations t " +
			"JOIN words w ON w.id = t.source_word_id " +
			"WHERE t.con_word_id = ? ORDER BY t.id"
	}

	rows, err := dao.DB.Query(query, wordID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Warn("Failed to close rows", "error", err)
		}
	}()

	translations := make([]storage.Translation, 0)
	for rows.Next() {
		var translation storage.Translation
		if err := rows.Scan(&translation.ID, &translation.Word.ID, &translation.Word.Text); err != nil {
			return nil, err
		}
		translations = append(translations, translation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return translations, nil
}

// DeleteTranslation deletes the translation link, not its words.
func (dao *DAO) DeleteTranslation(id int) error {
	_, err := dao.DB.Exec("DELETE FROM translations WHERE id = ?", id)
	return err
}
