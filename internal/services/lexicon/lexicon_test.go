package lexicon_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/lexicon"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/sqlite"
)

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

func load(t *testing.T, dao *sqlite.DAO, language string) *lexicon.Lexicon {
	t.Helper()
	l, err := lexicon.Load(dao, language)
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

func newTestDAO(t *testing.T) *sqlite.DAO {
	t.Helper()
	dao, err := sqlite.Load(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	t.Cleanup(func() {
		if err := dao.Close(); err != nil {
			t.Errorf("Close() failed: %v", err)
		}
	})
	if err := dao.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	return dao
}

func addWord(t *testing.T, dao *sqlite.DAO, lang, text, kind string) int {
	t.Helper()
	if err := dao.AddWord(lang, text); err != nil {
		t.Fatalf("adding word %q: %v", text, err)
	}
	var id int
	if err := dao.DB.QueryRow(`SELECT MAX(id) FROM words`).Scan(&id); err != nil {
		t.Fatalf("reading word id: %v", err)
	}
	if kind != storage.KindRoot {
		if _, err := dao.ChangeWordKind(id, kind); err != nil {
			t.Fatalf("changing kind of %q: %v", text, err)
		}
	}
	return id
}

func link(t *testing.T, dao *sqlite.DAO, sourceID, conID int) {
	t.Helper()
	if _, err := dao.DB.Exec(
		`INSERT INTO translations (source_word_id, con_word_id) VALUES (?, ?)`,
		sourceID, conID,
	); err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
}
