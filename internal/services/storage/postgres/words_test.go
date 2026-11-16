package postgres_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

func TestAddWord(t *testing.T) {
	t.Run("stores a root with its normalized form", func(t *testing.T) {
		f := newFixture(t)

		got, err := f.store.AddWord(t.Context(), f.user, storage.LangSource, "Éléphant")
		if err != nil {
			t.Fatalf("AddWord() failed: %v", err)
		}
		want := storage.WordDetails{ID: got.ID, Language: "source", Text: "Éléphant", Normalized: "elephant", Kind: "root"}
		if got != want {
			t.Errorf("AddWord() = %+v, want %+v", got, want)
		}
	})

	t.Run("reports the existing root", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")

		_, err := f.store.AddWord(t.Context(), f.user, storage.LangSource, "Mèr")
		if exists, ok := errors.AsType[*storage.WordExistsError](err); !ok || exists.ExistingID != mer {
			t.Errorf("AddWord() error = %v, want a WordExistsError for %d", err, mer)
		}
	})
}

func TestWord(t *testing.T) {
	t.Run("reports a missing word", func(t *testing.T) {
		f := newFixture(t)

		if _, err := f.store.Word(t.Context(), f.user, 1); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Word() error = %v, want ErrNotFound", err)
		}
	})
}

func TestListWordDetails(t *testing.T) {
	t.Run("lists the words of the language with their details", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		re := f.word(t, "source", "re", "prefix")
		f.word(t, "con", "ailo", "root")

		got, err := f.store.ListWordDetails(t.Context(), f.user, storage.LangSource)
		if err != nil {
			t.Fatalf("ListWordDetails() failed: %v", err)
		}
		want := []storage.WordDetails{
			{ID: mer, Language: "source", Text: "mer", Normalized: "mer", Kind: "root"},
			{ID: re, Language: "source", Text: "re", Normalized: "re", Kind: "prefix"},
		}
		if !slices.Equal(got, want) {
			t.Errorf("ListWordDetails(source) = %v, want %v", got, want)
		}
	})
}

func TestSaveWord(t *testing.T) {
	t.Run("renames the word to another case of itself", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")

		got, err := f.store.SaveWord(t.Context(), f.user, storage.WordDetails{ID: mer, Language: "source", Text: "Mer", Kind: "root"})
		if err != nil {
			t.Fatalf("SaveWord() failed: %v", err)
		}
		if got.Text != "Mer" || got.Normalized != "mer" {
			t.Errorf("SaveWord() = %+v, want text Mer normalized mer", got)
		}
	})

	t.Run("reports the existing word on collision", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ciel := f.word(t, "source", "ciel", "root")

		_, err := f.store.SaveWord(t.Context(), f.user, storage.WordDetails{ID: ciel, Language: "source", Text: "Mèr", Kind: "root"})
		if exists, ok := errors.AsType[*storage.WordExistsError](err); !ok || exists.ExistingID != mer {
			t.Errorf("SaveWord() error = %v, want a WordExistsError for %d", err, mer)
		}
	})

	t.Run("allows the same text with another kind", func(t *testing.T) {
		f := newFixture(t)
		f.word(t, "source", "re", "prefix")
		ciel := f.word(t, "source", "ciel", "root")

		if _, err := f.store.SaveWord(t.Context(), f.user, storage.WordDetails{ID: ciel, Language: "source", Text: "re", Kind: "root"}); err != nil {
			t.Fatalf("SaveWord() failed: %v", err)
		}
	})
}

func TestMergeWords(t *testing.T) {
	t.Run("moves the source word's links and deletes it", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ocean := f.word(t, "source", "ocean", "root")
		ailo := f.word(t, "con", "ailo", "root")
		vora := f.word(t, "con", "vora", "root")
		f.translation(t, mer, ailo, "generated")
		f.translation(t, ocean, ailo, "manual")
		f.translation(t, ocean, vora, "manual")

		if err := f.store.MergeWords(t.Context(), f.user, ocean, mer); err != nil {
			t.Fatalf("MergeWords() failed: %v", err)
		}

		f.assertTranslations(t, mer, "ailo", "vora")
		f.assertOrigin(t, mer, ailo, "generated")
		if _, err := f.store.Word(t.Context(), f.user, ocean); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("merged word: %v, want ErrNotFound", err)
		}
	})

	t.Run("moves the con word's links", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		aylo := f.word(t, "con", "aylo", "root")
		f.translation(t, mer, aylo, "manual")

		if err := f.store.MergeWords(t.Context(), f.user, aylo, ailo); err != nil {
			t.Fatalf("MergeWords() failed: %v", err)
		}

		f.assertTranslations(t, ailo, "mer")
		f.assertTranslations(t, mer, "ailo")
	})

	t.Run("rejects words of different languages", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")

		if err := f.store.MergeWords(t.Context(), f.user, mer, ailo); err == nil {
			t.Fatal("MergeWords() succeeded unexpectedly")
		}
		f.assertWordExists(t, mer)
	})

	t.Run("rejects merging a word into itself", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		f.translation(t, mer, ailo, "manual")

		if err := f.store.MergeWords(t.Context(), f.user, mer, mer); err == nil {
			t.Fatal("MergeWords() succeeded unexpectedly")
		}
		f.assertWordExists(t, mer)
		f.assertTranslations(t, mer, "ailo")
	})
}

func TestDeleteWord(t *testing.T) {
	t.Run("deletes the word and its translations", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		ciel := f.word(t, "source", "ciel", "root")
		f.translation(t, mer, ailo, "manual")
		f.translation(t, ciel, ailo, "manual")

		if err := f.store.DeleteWord(t.Context(), f.user, mer); err != nil {
			t.Fatalf("DeleteWord() failed: %v", err)
		}

		if _, err := f.store.Word(t.Context(), f.user, mer); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("deleted word: %v, want ErrNotFound", err)
		}
		f.assertWordExists(t, ailo)
		if got := f.countTranslations(t, mer); got != 0 {
			t.Errorf("deleted word still has %d translations, want 0", got)
		}
		if got := f.countTranslations(t, ciel); got != 1 {
			t.Errorf("other word has %d translations, want 1", got)
		}
	})

	t.Run("reports a missing word", func(t *testing.T) {
		f := newFixture(t)

		if err := f.store.DeleteWord(t.Context(), f.user, 1); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("DeleteWord() error = %v, want ErrNotFound", err)
		}
	})
}

func TestChangeWordKind(t *testing.T) {
	t.Run("changes the kind and deletes the word's translations", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		ciel := f.word(t, "source", "ciel", "root")
		vora := f.word(t, "con", "vora", "root")
		f.translation(t, mer, ailo, "manual")
		f.translation(t, ciel, ailo, "manual")
		f.translation(t, ciel, vora, "manual")

		word, err := f.store.ChangeWordKind(t.Context(), f.user, ailo, "prefix")
		if err != nil {
			t.Fatalf("ChangeWordKind() failed: %v", err)
		}

		if word.Kind != "prefix" {
			t.Errorf("returned kind = %q, want %q", word.Kind, "prefix")
		}
		stored, err := f.store.Word(t.Context(), f.user, ailo)
		if err != nil {
			t.Fatalf("Word() failed: %v", err)
		}
		if stored.Kind != "prefix" {
			t.Errorf("stored kind = %q, want %q", stored.Kind, "prefix")
		}
		if got := f.countTranslations(t, ailo); got != 0 {
			t.Errorf("word still has %d translations, want 0", got)
		}
		if got := f.countTranslations(t, ciel); got != 1 {
			t.Errorf("unrelated translation deleted: ciel has %d translations, want 1", got)
		}
	})

	t.Run("keeps the translations when the change collides", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		merPrefix := f.word(t, "source", "mer", "prefix")
		ailo := f.word(t, "con", "ailo", "root")
		f.translation(t, mer, ailo, "manual")

		_, err := f.store.ChangeWordKind(t.Context(), f.user, mer, "prefix")
		if exists, ok := errors.AsType[*storage.WordExistsError](err); !ok || exists.ExistingID != merPrefix {
			t.Fatalf("ChangeWordKind() error = %v, want a WordExistsError for %d", err, merPrefix)
		}

		stored, err := f.store.Word(t.Context(), f.user, mer)
		if err != nil {
			t.Fatalf("Word() failed: %v", err)
		}
		if stored.Kind != "root" {
			t.Errorf("stored kind = %q, want %q", stored.Kind, "root")
		}
		if got := f.countTranslations(t, mer); got != 1 {
			t.Errorf("word has %d translations, want 1", got)
		}
	})
}
