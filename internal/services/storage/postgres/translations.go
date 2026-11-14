package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// Origins of a translation link.
const (
	originManual    = "manual"
	originGenerated = "generated"
)

// ListTranslations lists the word's translation links, in insertion order.
func (s *Store) ListTranslations(ctx context.Context, userID, wordID int) ([]storage.Translation, error) {
	language, err := wordLanguage(ctx, s.pool, userID, wordID)
	if err != nil {
		return nil, err
	}

	query := "SELECT t.id, w.id, w.text FROM translations t " +
		"JOIN words w ON w.id = t.con_word_id " +
		"WHERE t.source_word_id = $1 AND t.user_id = $2 ORDER BY t.id"
	if language == storage.LangCon {
		query = "SELECT t.id, w.id, w.text FROM translations t " +
			"JOIN words w ON w.id = t.source_word_id " +
			"WHERE t.con_word_id = $1 AND t.user_id = $2 ORDER BY t.id"
	}

	rows, err := s.pool.Query(ctx, query, wordID, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (storage.Translation, error) {
		var translation storage.Translation
		err := row.Scan(&translation.ID, &translation.Word.ID, &translation.Word.Text)
		return translation, err
	})
}

// ListTranslationsByLanguage lists the translation links of every word of
// the user in language, keyed by word id, in insertion order.
func (s *Store) ListTranslationsByLanguage(ctx context.Context, userID int, language string) (map[int][]storage.Translation, error) {
	query := "SELECT t.source_word_id, t.id, w.id, w.text FROM translations t " +
		"JOIN words w ON w.id = t.con_word_id WHERE t.user_id = $1 ORDER BY t.id"
	if language == storage.LangCon {
		query = "SELECT t.con_word_id, t.id, w.id, w.text FROM translations t " +
			"JOIN words w ON w.id = t.source_word_id WHERE t.user_id = $1 ORDER BY t.id"
	}

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	translations := make(map[int][]storage.Translation)
	var wordID int
	var translation storage.Translation
	_, err = pgx.ForEachRow(rows, []any{&wordID, &translation.ID, &translation.Word.ID, &translation.Word.Text}, func() error {
		translations[wordID] = append(translations[wordID], translation)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return translations, nil
}

// DeleteTranslation deletes the translation link, not its words.
func (s *Store) DeleteTranslation(ctx context.Context, userID, id int) error {
	return notFoundIfNoneAffected(s.pool.Exec(ctx, "DELETE FROM translations WHERE id = $1 AND user_id = $2", id, userID))
}

// AddTranslation links the word to targetID, a word of the other language,
// as a manual translation. Linking words already linked is a no-op.
func (s *Store) AddTranslation(ctx context.Context, userID, wordID, targetID int) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		language, err := wordLanguage(ctx, tx, userID, wordID)
		if err != nil {
			return err
		}
		targetLanguage, err := wordLanguage(ctx, tx, userID, targetID)
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
		return link(ctx, tx, userID, sourceID, conID, originManual)
	})
}

// AddTranslationWord links the word, as a manual translation, to the root
// spelled text in the other language, creating it when missing.
func (s *Store) AddTranslationWord(ctx context.Context, userID, wordID int, text string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		language, err := wordLanguage(ctx, tx, userID, wordID)
		if err != nil {
			return err
		}

		var sourceID, conID int
		if language == storage.LangSource {
			sourceID = wordID
			if conID, err = upsertWord(ctx, tx, userID, storage.LangCon, text, storage.KindRoot); err != nil {
				return err
			}
		} else {
			conID = wordID
			if sourceID, err = upsertWord(ctx, tx, userID, storage.LangSource, text, storage.KindRoot); err != nil {
				return err
			}
		}
		return link(ctx, tx, userID, sourceID, conID, originManual)
	})
}

// SaveGeneratedTranslation links each part to its con translation, creating
// the missing words and links. With several parts, it also links wordID to
// the concatenation of the con parts, stored as a con root.
func (s *Store) SaveGeneratedTranslation(ctx context.Context, userID, wordID int, parts []storage.TranslationPart) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := wordLanguage(ctx, tx, userID, wordID); err != nil {
			return err
		}

		var full strings.Builder
		for _, part := range parts {
			// The parts' ids come from the caller: an already linked pair of
			// another user would slip past the foreign keys through ON CONFLICT
			for _, id := range []int{part.SourceID, part.ConID} {
				if id == 0 {
					continue
				}
				if _, err := wordLanguage(ctx, tx, userID, id); err != nil {
					return err
				}
			}

			sourceID := part.SourceID
			var err error
			if sourceID == 0 {
				if sourceID, err = upsertWord(ctx, tx, userID, storage.LangSource, part.Source, part.Kind); err != nil {
					return err
				}
			}
			conID := part.ConID
			if conID == 0 {
				if conID, err = upsertWord(ctx, tx, userID, storage.LangCon, part.Con, part.Kind); err != nil {
					return err
				}
			}
			if err := link(ctx, tx, userID, sourceID, conID, originGenerated); err != nil {
				return err
			}
			full.WriteString(part.Con)
		}

		if len(parts) > 1 {
			fullID, err := upsertWord(ctx, tx, userID, storage.LangCon, full.String(), storage.KindRoot)
			if err != nil {
				return err
			}
			if err := link(ctx, tx, userID, wordID, fullID, originGenerated); err != nil {
				return err
			}
		}
		return nil
	})
}

// link links the two words of the user, unless they already are. A word of
// another user breaks the composite foreign keys and is reported as
// storage.ErrNotFound.
func link(ctx context.Context, q querier, userID, sourceID, conID int, origin string) error {
	_, err := q.Exec(ctx,
		"INSERT INTO translations (user_id, source_word_id, con_word_id, origin) VALUES ($1, $2, $3, $4) "+
			"ON CONFLICT (source_word_id, con_word_id) DO NOTHING",
		userID, sourceID, conID, origin,
	)
	if isViolation(err, foreignKeyViolation) {
		return storage.ErrNotFound
	}
	return err
}
