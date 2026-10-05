package sqlite

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
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
	sqlitedriver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
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

func (dao *DAO) ListWordDetails(language string) ([]storage.WordDetails, error) {
	rows, err := dao.DB.Query("SELECT id, lang, text, normalized, kind FROM words WHERE lang = ?", language)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Warn("Failed to close rows", "error", err)
		}
	}()

	words := make([]storage.WordDetails, 0)
	for rows.Next() {
		var word storage.WordDetails
		if err := rows.Scan(&word.ID, &word.Language, &word.Text, &word.Normalized, &word.Kind); err != nil {
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

// SaveWord updates the word. It returns a *storage.WordExistsError when
// another word has the same language, normalized form and kind.
func (dao *DAO) SaveWord(word storage.WordDetails) (storage.WordDetails, error) {
	word.Normalized = normalize(word.Text)

	row := dao.DB.QueryRow(
		"UPDATE words SET lang = ?, text = ?, normalized = ?, kind = ? WHERE id = ? "+
			"RETURNING id, lang, text, normalized, kind",
		word.Language, word.Text, word.Normalized, word.Kind, word.ID,
	)

	if err := row.Scan(&word.ID, &word.Language, &word.Text, &word.Normalized, &word.Kind); err != nil {
		if !isUniqueViolation(err) {
			return storage.WordDetails{}, err
		}
		var existingID int
		lookupErr := dao.DB.QueryRow(
			"SELECT id FROM words WHERE lang = ? AND normalized = ? AND kind = ? AND id != ?",
			word.Language, word.Normalized, word.Kind, word.ID,
		).Scan(&existingID)
		if lookupErr != nil {
			return storage.WordDetails{}, errors.Join(err, lookupErr)
		}
		return storage.WordDetails{}, &storage.WordExistsError{ExistingID: existingID}
	}

	return word, nil
}

// MergeWords moves the translations of fromID to intoID, a word of the same
// language, then deletes fromID. A link both words share keeps intoID's.
func (dao *DAO) MergeWords(fromID, intoID int) error {
	tx, err := dao.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Warn("Failed to rollback transaction", "error", err)
		}
	}()

	language, err := wordLanguage(tx, fromID)
	if err != nil {
		return err
	}
	intoLanguage, err := wordLanguage(tx, intoID)
	if err != nil {
		return err
	}
	if language != intoLanguage {
		return fmt.Errorf("merging words of different languages: %d and %d", fromID, intoID)
	}

	query := "INSERT INTO translations (source_word_id, con_word_id, source, created_at) " +
		"SELECT ?, con_word_id, source, created_at FROM translations WHERE source_word_id = ? " +
		"ON CONFLICT (source_word_id, con_word_id) DO NOTHING"
	if language == storage.LangCon {
		query = "INSERT INTO translations (source_word_id, con_word_id, source, created_at) " +
			"SELECT source_word_id, ?, source, created_at FROM translations WHERE con_word_id = ? " +
			"ON CONFLICT (source_word_id, con_word_id) DO NOTHING"
	}
	if _, err := tx.Exec(query, intoID, fromID); err != nil {
		return err
	}

	// The cascade deletes fromID's remaining links
	if _, err := tx.Exec("DELETE FROM words WHERE id = ?", fromID); err != nil {
		return err
	}

	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	var sqliteErr *sqlitedriver.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
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

// AddTranslation links the word to targetID, a word of the other language,
// as a manual translation. Linking words already linked is a no-op.
func (dao *DAO) AddTranslation(wordID, targetID int) error {
	tx, err := dao.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Warn("Failed to rollback transaction", "error", err)
		}
	}()

	language, err := wordLanguage(tx, wordID)
	if err != nil {
		return err
	}
	targetLanguage, err := wordLanguage(tx, targetID)
	if err != nil {
		return err
	}
	if language == targetLanguage {
		return storage.ErrSameLanguage
	}

	sourceID, conID := wordID, targetID
	if language == storage.LangCon {
		sourceID, conID = targetID, wordID
	}
	if err := linkManual(tx, sourceID, conID); err != nil {
		return err
	}

	return tx.Commit()
}

// AddTranslationWord links the word, as a manual translation, to the root
// spelled text in the other language, creating it when missing.
func (dao *DAO) AddTranslationWord(wordID int, text string) error {
	tx, err := dao.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Warn("Failed to rollback transaction", "error", err)
		}
	}()

	language, err := wordLanguage(tx, wordID)
	if err != nil {
		return err
	}

	var sourceID, conID int
	if language == storage.LangSource {
		sourceID = wordID
		if conID, err = upsertWord(tx, storage.LangCon, text, storage.KindRoot); err != nil {
			return err
		}
	} else {
		conID = wordID
		if sourceID, err = upsertWord(tx, storage.LangSource, text, storage.KindRoot); err != nil {
			return err
		}
	}
	if err := linkManual(tx, sourceID, conID); err != nil {
		return err
	}

	return tx.Commit()
}

// SaveGeneratedTranslation links each part to its con translation, creating
// the missing words and links. With several parts, it also links wordID to
// the concatenation of the con parts, stored as a con root.
func (dao *DAO) SaveGeneratedTranslation(wordID int, parts []storage.TranslationPart) error {
	tx, err := dao.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Warn("Failed to rollback transaction", "error", err)
		}
	}()

	var full strings.Builder
	for _, part := range parts {
		sourceID := part.SourceID
		if sourceID == 0 {
			if sourceID, err = upsertWord(tx, storage.LangSource, part.Source, part.Kind); err != nil {
				return err
			}
		}
		conID := part.ConID
		if conID == 0 {
			if conID, err = upsertWord(tx, storage.LangCon, part.Con, part.Kind); err != nil {
				return err
			}
		}
		if err := linkGenerated(tx, sourceID, conID); err != nil {
			return err
		}
		full.WriteString(part.Con)
	}

	if len(parts) > 1 {
		fullID, err := upsertWord(tx, storage.LangCon, full.String(), storage.KindRoot)
		if err != nil {
			return err
		}
		if err := linkGenerated(tx, wordID, fullID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// upsertWord returns the id of the word, inserting it when missing.
func upsertWord(tx *sql.Tx, language, text, kind string) (int, error) {
	var id int
	err := tx.QueryRow(
		"INSERT INTO words (lang, text, normalized, kind) VALUES (?, ?, ?, ?) "+
			"ON CONFLICT (lang, normalized, kind) DO UPDATE SET text = text RETURNING id",
		language, text, normalize(text), kind,
	).Scan(&id)
	return id, err
}

func wordLanguage(tx *sql.Tx, id int) (string, error) {
	var language string
	err := tx.QueryRow("SELECT lang FROM words WHERE id = ?", id).Scan(&language)
	return language, err
}

// linkManual links the two words, unless they already are.
func linkManual(tx *sql.Tx, sourceID, conID int) error {
	_, err := tx.Exec(
		"INSERT INTO translations (source_word_id, con_word_id, source) VALUES (?, ?, 'manual') "+
			"ON CONFLICT (source_word_id, con_word_id) DO NOTHING",
		sourceID, conID,
	)
	return err
}

// linkGenerated links the two words, unless they already are.
func linkGenerated(tx *sql.Tx, sourceID, conID int) error {
	_, err := tx.Exec(
		"INSERT INTO translations (source_word_id, con_word_id, source) VALUES (?, ?, 'generated') "+
			"ON CONFLICT (source_word_id, con_word_id) DO NOTHING",
		sourceID, conID,
	)
	return err
}
