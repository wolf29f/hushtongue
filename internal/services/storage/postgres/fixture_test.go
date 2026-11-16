package postgres_test

import (
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

// fixture is a store on a fresh schema, with one user to act as.
type fixture struct {
	store *postgres.Store
	pool  *pgxpool.Pool
	user  int
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := pgtest.New(t)
	return fixture{store: postgres.New(pool), pool: pool, user: pgtest.NewUser(t, pool)}
}

// otherUser returns a fixture acting as a new user of the same database.
func (f fixture) otherUser(t *testing.T) fixture {
	t.Helper()
	f.user = pgtest.NewUser(t, f.pool)
	return f
}

// word inserts a word of the user, its text as normalized form.
func (f fixture) word(t *testing.T, lang, text, kind string) int {
	t.Helper()
	var id int
	if err := f.pool.QueryRow(t.Context(),
		`INSERT INTO words (user_id, lang, text, normalized, kind) VALUES ($1, $2, $3, $3, $4) RETURNING id`,
		f.user, lang, text, kind,
	).Scan(&id); err != nil {
		t.Fatalf("inserting word %q: %v", text, err)
	}
	return id
}

// translation links two words of the user.
func (f fixture) translation(t *testing.T, sourceID, conID int, origin string) int {
	t.Helper()
	var id int
	if err := f.pool.QueryRow(t.Context(),
		`INSERT INTO translations (user_id, source_word_id, con_word_id, origin) VALUES ($1, $2, $3, $4) RETURNING id`,
		f.user, sourceID, conID, origin,
	).Scan(&id); err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
	return id
}

func (f fixture) findWord(t *testing.T, lang, text, kind string) int {
	t.Helper()
	var id int
	if err := f.pool.QueryRow(t.Context(),
		`SELECT id FROM words WHERE user_id = $1 AND lang = $2 AND text = $3 AND kind = $4`,
		f.user, lang, text, kind,
	).Scan(&id); err != nil {
		t.Fatalf("finding word %q: %v", text, err)
	}
	return id
}

func (f fixture) countTranslations(t *testing.T, wordID int) int {
	t.Helper()
	var count int
	if err := f.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM translations WHERE source_word_id = $1 OR con_word_id = $1`,
		wordID,
	).Scan(&count); err != nil {
		t.Fatalf("counting translations: %v", err)
	}
	return count
}

func (f fixture) assertTranslations(t *testing.T, wordID int, want ...string) {
	t.Helper()
	translations, err := f.store.ListTranslations(t.Context(), f.user, wordID)
	if err != nil {
		t.Fatalf("ListTranslations() failed: %v", err)
	}
	got := make([]string, len(translations))
	for i, translation := range translations {
		got[i] = translation.Word.Text
	}
	if !slices.Equal(got, want) {
		t.Errorf("translations of word %d = %v, want %v", wordID, got, want)
	}
}

func (f fixture) assertOrigin(t *testing.T, sourceID, conID int, want string) {
	t.Helper()
	var got string
	if err := f.pool.QueryRow(t.Context(),
		`SELECT origin FROM translations WHERE source_word_id = $1 AND con_word_id = $2`,
		sourceID, conID,
	).Scan(&got); err != nil {
		t.Fatalf("reading link origin: %v", err)
	}
	if got != want {
		t.Errorf("link origin = %q, want %q", got, want)
	}
}

func (f fixture) assertWordExists(t *testing.T, id int) {
	t.Helper()
	if _, err := f.store.Word(t.Context(), f.user, id); err != nil {
		t.Errorf("word %d: %v, want it stored", id, err)
	}
}
