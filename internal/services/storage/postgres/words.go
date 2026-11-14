package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

func scanWord(row pgx.Row) (storage.WordDetails, error) {
	var word storage.WordDetails
	err := row.Scan(&word.ID, &word.Language, &word.Text, &word.Normalized, &word.Kind)
	return word, err
}

// AddWord stores a root. It returns a *storage.WordExistsError when the user
// has a root of the same language and normalized form.
func (s *Store) AddWord(ctx context.Context, userID int, language, text string) (storage.WordDetails, error) {
	normalized := storage.Normalize(text)
	word, err := scanWord(s.pool.QueryRow(ctx,
		"INSERT INTO words (user_id, lang, text, normalized) VALUES ($1, $2, $3, $4) RETURNING id, lang, text, normalized, kind",
		userID, language, text, normalized,
	))
	if isViolation(err, uniqueViolation) {
		return storage.WordDetails{}, existingWord(ctx, s.pool, err, userID, 0, language, normalized, storage.KindRoot)
	}
	return word, err
}

// ListWordDetails lists the user's words of language, in insertion order.
func (s *Store) ListWordDetails(ctx context.Context, userID int, language string) ([]storage.WordDetails, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, lang, text, normalized, kind FROM words WHERE user_id = $1 AND lang = $2 ORDER BY id",
		userID, language,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (storage.WordDetails, error) {
		return scanWord(row)
	})
}

func (s *Store) GetWord(ctx context.Context, userID, id int) (storage.WordDetails, error) {
	word, err := scanWord(s.pool.QueryRow(ctx,
		"SELECT id, lang, text, normalized, kind FROM words WHERE id = $1 AND user_id = $2",
		id, userID,
	))
	return word, notFoundIfNoRows(err)
}

// SaveWord updates the word. It returns a *storage.WordExistsError when
// another word of the user has the same language, normalized form and kind.
func (s *Store) SaveWord(ctx context.Context, userID int, word storage.WordDetails) (storage.WordDetails, error) {
	word.Normalized = storage.Normalize(word.Text)
	saved, err := scanWord(s.pool.QueryRow(ctx,
		"UPDATE words SET lang = $1, text = $2, normalized = $3, kind = $4 WHERE id = $5 AND user_id = $6 RETURNING id, lang, text, normalized, kind",
		word.Language, word.Text, word.Normalized, word.Kind, word.ID, userID,
	))
	if isViolation(err, uniqueViolation) {
		return storage.WordDetails{}, existingWord(ctx, s.pool, err, userID, word.ID, word.Language, word.Normalized, word.Kind)
	}
	return saved, notFoundIfNoRows(err)
}

// existingWord turns the unique violation err into a
// *storage.WordExistsError naming the user's word it collides with.
func existingWord(ctx context.Context, q querier, err error, userID, id int, language, normalized, kind string) error {
	var existingID int
	lookupErr := q.QueryRow(ctx,
		"SELECT id FROM words WHERE user_id = $1 AND lang = $2 AND normalized = $3 AND kind = $4 AND id != $5",
		userID, language, normalized, kind, id,
	).Scan(&existingID)
	if lookupErr != nil {
		return errors.Join(err, lookupErr)
	}
	return &storage.WordExistsError{ExistingID: existingID}
}

// MergeWords moves the translations of fromID to intoID, a word of the same
// language, then deletes fromID. A link both words share keeps intoID's.
func (s *Store) MergeWords(ctx context.Context, userID, fromID, intoID int) error {
	if fromID == intoID {
		return fmt.Errorf("merging word %d into itself", fromID)
	}

	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		language, err := wordLanguage(ctx, tx, userID, fromID)
		if err != nil {
			return err
		}
		intoLanguage, err := wordLanguage(ctx, tx, userID, intoID)
		if err != nil {
			return err
		}
		if language != intoLanguage {
			return fmt.Errorf("merging words of different languages: %d and %d", fromID, intoID)
		}

		query := "INSERT INTO translations (user_id, source_word_id, con_word_id, origin, note, created_at) " +
			"SELECT user_id, $1, con_word_id, origin, note, created_at FROM translations WHERE source_word_id = $2 AND user_id = $3 " +
			"ON CONFLICT (source_word_id, con_word_id) DO NOTHING"
		if language == storage.LangCon {
			query = "INSERT INTO translations (user_id, source_word_id, con_word_id, origin, note, created_at) " +
				"SELECT user_id, source_word_id, $1, origin, note, created_at FROM translations WHERE con_word_id = $2 AND user_id = $3 " +
				"ON CONFLICT (source_word_id, con_word_id) DO NOTHING"
		}
		if _, err := tx.Exec(ctx, query, intoID, fromID, userID); err != nil {
			return err
		}

		// The cascade deletes fromID's remaining links
		_, err = tx.Exec(ctx, "DELETE FROM words WHERE id = $1 AND user_id = $2", fromID, userID)
		return err
	})
}

// DeleteWord deletes the word and, by cascade, its translations.
func (s *Store) DeleteWord(ctx context.Context, userID, id int) error {
	return notFoundIfNoneAffected(s.pool.Exec(ctx, "DELETE FROM words WHERE id = $1 AND user_id = $2", id, userID))
}

// ChangeWordKind sets the word's kind and deletes its translations. It
// returns a *storage.WordExistsError when the user has a word of the same
// language and normalized form with that kind.
func (s *Store) ChangeWordKind(ctx context.Context, userID, id int, kind string) (storage.WordDetails, error) {
	var word storage.WordDetails
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			"DELETE FROM translations WHERE (source_word_id = $1 OR con_word_id = $1) AND user_id = $2",
			id, userID,
		); err != nil {
			return err
		}

		var err error
		word, err = scanWord(tx.QueryRow(ctx,
			"UPDATE words SET kind = $1 WHERE id = $2 AND user_id = $3 RETURNING id, lang, text, normalized, kind",
			kind, id, userID,
		))
		return notFoundIfNoRows(err)
	})
	if isViolation(err, uniqueViolation) {
		current, getErr := s.GetWord(ctx, userID, id)
		if getErr != nil {
			return storage.WordDetails{}, errors.Join(err, getErr)
		}
		return storage.WordDetails{}, existingWord(ctx, s.pool, err, userID, id, current.Language, current.Normalized, kind)
	}
	if err != nil {
		return storage.WordDetails{}, err
	}
	return word, nil
}

func wordLanguage(ctx context.Context, q querier, userID, id int) (string, error) {
	var language string
	err := q.QueryRow(ctx, "SELECT lang FROM words WHERE id = $1 AND user_id = $2", id, userID).Scan(&language)
	return language, notFoundIfNoRows(err)
}

// upsertWord returns the id of the user's word, inserting it when missing.
func upsertWord(ctx context.Context, q querier, userID int, language, text, kind string) (int, error) {
	var id int
	err := q.QueryRow(ctx,
		"INSERT INTO words (user_id, lang, text, normalized, kind) VALUES ($1, $2, $3, $4, $5) "+
			"ON CONFLICT (user_id, lang, normalized, kind) DO UPDATE SET text = words.text RETURNING id",
		userID, language, text, storage.Normalize(text), kind,
	).Scan(&id)
	return id, err
}
