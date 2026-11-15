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
		f := newFixture(t)
		mer := addWord(t, f, "source", "Mer", "root")
		addWord(t, f, "source", "terre", "root")
		ailo := addWord(t, f, "con", "ailo", "root")
		vora := addWord(t, f, "con", "vora", "root")
		link(t, f, mer, ailo)
		link(t, f, mer, vora)

		l := load(t, f, "source")

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
		f := newFixture(t)
		link(t, f, addWord(t, f, "source", "mer", "root"), addWord(t, f, "con", "ailo", "root"))

		entry := load(t, f, "con").Lookup("ailo")

		if got, want := translationTexts(entry.Translations), []string{"mer"}; !slices.Equal(got, want) {
			t.Errorf("translations = %v, want %v", got, want)
		}
	})

	t.Run("ignores case, accents and surrounding punctuation", func(t *testing.T) {
		f := newFixture(t)
		ile := addWord(t, f, "source", "île", "root")

		l := load(t, f, "source")

		for _, typed := range []string{"Île", "ile", "ile,", "«île»", "île..."} {
			if got := l.Lookup(typed).Word.ID; got != ile {
				t.Errorf("Lookup(%q) matched word %d, want %d", typed, got, ile)
			}
		}
	})

	t.Run("matches roots only", func(t *testing.T) {
		f := newFixture(t)
		addWord(t, f, "source", "anti", "prefix")

		if got := load(t, f, "source").Lookup("anti").Status; got != lexicon.StatusUnknown {
			t.Errorf("Lookup(anti).Status = %v, want unknown", got)
		}
	})
}

func TestComplete(t *testing.T) {
	t.Run("ranks the words beginning with the prefix first", func(t *testing.T) {
		f := newFixture(t)
		for _, word := range []string{"montagne", "maison", "mer", "amer", "ciel"} {
			addWord(t, f, "source", word, "root")
		}

		words, more := load(t, f, "source").Complete("me", 5)

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
		f := newFixture(t)
		for _, word := range []string{"ma", "mai", "maison", "mal", "malle", "mare"} {
			addWord(t, f, "source", word, "root")
		}

		words, more := load(t, f, "source").Complete("ma", 5)

		if len(words) != 5 || !more {
			t.Errorf("Complete(ma, 5) = %v, %v, want 5 words and more", wordTexts(words), more)
		}
		if words[0].Text != "ma" {
			t.Errorf("first word = %q, want the exact match ma", words[0].Text)
		}
	})

	t.Run("returns nothing for an empty prefix", func(t *testing.T) {
		f := newFixture(t)
		addWord(t, f, "source", "mer", "root")

		if words, more := load(t, f, "source").Complete(" ,", 5); len(words) != 0 || more {
			t.Errorf("Complete() = %v, %v, want nothing", wordTexts(words), more)
		}
	})
}

func load(t *testing.T, f *fixture, language string) *lexicon.Lexicon {
	t.Helper()
	l, err := lexicon.Load(context.Background(), f.store, f.user, language)
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

type fixture struct {
	store *postgres.Store
	pool  *pgxpool.Pool
	user  int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := pgtest.New(t)
	return &fixture{store: postgres.New(pool), pool: pool, user: pgtest.NewUser(t, pool)}
}

func addWord(t *testing.T, f *fixture, lang, text, kind string) int {
	t.Helper()
	word, err := f.store.AddWord(context.Background(), f.user, lang, text)
	if err != nil {
		t.Fatalf("adding word %q: %v", text, err)
	}
	if kind != storage.KindRoot {
		if _, err := f.store.ChangeWordKind(context.Background(), f.user, word.ID, kind); err != nil {
			t.Fatalf("changing kind of %q: %v", text, err)
		}
	}
	return word.ID
}

func link(t *testing.T, f *fixture, sourceID, conID int) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO translations (user_id, source_word_id, con_word_id) VALUES ($1, $2, $3)`,
		f.user, sourceID, conID,
	); err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
}
