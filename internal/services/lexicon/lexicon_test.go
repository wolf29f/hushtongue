package lexicon_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/services/lexicon"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Run(m))
}

func TestLookup(t *testing.T) {
	t.Run("tells translated, untranslated and unknown words apart", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := addWord(t, dao, "source", "Mer", "root")
		addWord(t, dao, "source", "terre", "root")
		ailo := addWord(t, dao, "con", "ailo", "root")
		vora := addWord(t, dao, "con", "vora", "root")
		link(t, dao, mer, ailo)
		link(t, dao, mer, vora)

		l := load(t, dao, "source")

		entry := l.Lookup("mer")
		if entry.Status != lexicon.StatusTranslated || entry.Word.ID != mer {
			t.Errorf("Lookup(mer) = %+v, want translated word %d", entry, mer)
		}
		if got, want := translationTexts(entry.Translations), []string{"ailo", "vora"}; !slices.Equal(got, want) {
			t.Errorf("translations = %v, want %v", got, want)
		}
		if got := l.Lookup("terre").Status; got != lexicon.StatusUntranslated {
			t.Errorf("Lookup(terre).Status = %v, want untranslated", got)
		}
		if got := l.Lookup("ciel").Status; got != lexicon.StatusUnknown {
			t.Errorf("Lookup(ciel).Status = %v, want unknown", got)
		}
	})

	t.Run("loads the translations of con words", func(t *testing.T) {
		dao := newTestDAO(t)
		link(t, dao, addWord(t, dao, "source", "mer", "root"), addWord(t, dao, "con", "ailo", "root"))

		entry := load(t, dao, "con").Lookup("ailo")

		if got, want := translationTexts(entry.Translations), []string{"mer"}; !slices.Equal(got, want) {
			t.Errorf("translations = %v, want %v", got, want)
		}
	})

	t.Run("ignores case, accents and surrounding punctuation", func(t *testing.T) {
		dao := newTestDAO(t)
		ile := addWord(t, dao, "source", "île", "root")

		l := load(t, dao, "source")

		for _, typed := range []string{"Île", "ile", "ile,", "«île»", "île..."} {
			if got := l.Lookup(typed).Word.ID; got != ile {
				t.Errorf("Lookup(%q) matched word %d, want %d", typed, got, ile)
			}
		}
	})

	t.Run("matches roots only", func(t *testing.T) {
		dao := newTestDAO(t)
		addWord(t, dao, "source", "anti", "prefix")

		if got := load(t, dao, "source").Lookup("anti").Status; got != lexicon.StatusUnknown {
			t.Errorf("Lookup(anti).Status = %v, want unknown", got)
		}
	})
}

func TestComplete(t *testing.T) {
	t.Run("ranks the words beginning with the prefix first", func(t *testing.T) {
		dao := newTestDAO(t)
		for _, word := range []string{"montagne", "maison", "mer", "amer", "ciel"} {
			addWord(t, dao, "source", word, "root")
		}

		words, more := load(t, dao, "source").Complete("me", 5)

		if got := wordTexts(words); len(got) == 0 || got[0] != "mer" {
			t.Errorf("Complete(me) = %v, want mer first", got)
		}
		if slices.Contains(wordTexts(words), "ciel") {
			t.Errorf("Complete(me) = %v, want no word without a shared trigram", wordTexts(words))
		}
		if more {
			t.Error("more = true, want false")
		}
	})

	t.Run("caps the result and reports the words left out", func(t *testing.T) {
		dao := newTestDAO(t)
		for _, word := range []string{"ma", "mai", "maison", "mal", "malle", "mare"} {
			addWord(t, dao, "source", word, "root")
		}

		words, more := load(t, dao, "source").Complete("ma", 5)

		if len(words) != 5 || !more {
			t.Errorf("Complete(ma, 5) = %v, %v, want 5 words and more", wordTexts(words), more)
		}
		if words[0].Text != "ma" {
			t.Errorf("first word = %q, want the exact match ma", words[0].Text)
		}
	})

	t.Run("returns nothing for an empty prefix", func(t *testing.T) {
		dao := newTestDAO(t)
		addWord(t, dao, "source", "mer", "root")

		if words, more := load(t, dao, "source").Complete(" ,", 5); len(words) != 0 || more {
			t.Errorf("Complete() = %v, %v, want nothing", wordTexts(words), more)
		}
	})
}

func load(t *testing.T, dao *testDB, language string) *lexicon.Lexicon {
	t.Helper()
	l, err := lexicon.Load(context.Background(), dao.store, dao.user, language)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	return l
}

func translationTexts(translations []storage.Translation) []string {
	out := make([]string, len(translations))
	for i, translation := range translations {
		out[i] = translation.Word.Text
	}
	return out
}

func wordTexts(words []storage.Word) []string {
	out := make([]string, len(words))
	for i, word := range words {
		out[i] = word.Text
	}
	return out
}

type testDB struct {
	store *postgres.Store
	pool  *pgxpool.Pool
	user  int
}

func newTestDAO(t *testing.T) *testDB {
	t.Helper()
	pool := pgtest.New(t)
	return &testDB{store: postgres.New(pool), pool: pool, user: pgtest.NewUser(t, pool)}
}

func addWord(t *testing.T, dao *testDB, lang, text, kind string) int {
	t.Helper()
	word, err := dao.store.AddWord(context.Background(), dao.user, lang, text)
	if err != nil {
		t.Fatalf("adding word %q: %v", text, err)
	}
	if kind != storage.KindRoot {
		if _, err := dao.store.ChangeWordKind(context.Background(), dao.user, word.ID, kind); err != nil {
			t.Fatalf("changing kind of %q: %v", text, err)
		}
	}
	return word.ID
}

func link(t *testing.T, dao *testDB, sourceID, conID int) {
	t.Helper()
	if _, err := dao.pool.Exec(context.Background(),
		`INSERT INTO translations (user_id, source_word_id, con_word_id) VALUES ($1, $2, $3)`,
		dao.user, sourceID, conID,
	); err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
}
