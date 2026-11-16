package postgres_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

func TestListTranslations(t *testing.T) {
	t.Run("lists the words on the other side, in insertion order", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		vora := f.word(t, "con", "vora", "root")
		ciel := f.word(t, "source", "ciel", "root")
		merVora := f.translation(t, mer, vora, "manual")
		merAilo := f.translation(t, mer, ailo, "manual")
		cielAilo := f.translation(t, ciel, ailo, "manual")

		got, err := f.store.ListTranslations(t.Context(), f.user, mer)
		if err != nil {
			t.Fatalf("ListTranslations() failed: %v", err)
		}
		want := []storage.Translation{
			{ID: merVora, Word: storage.Word{ID: vora, Text: "vora"}},
			{ID: merAilo, Word: storage.Word{ID: ailo, Text: "ailo"}},
		}
		if !slices.Equal(got, want) {
			t.Errorf("ListTranslations(mer) = %v, want %v", got, want)
		}

		got, err = f.store.ListTranslations(t.Context(), f.user, ailo)
		if err != nil {
			t.Fatalf("ListTranslations() failed: %v", err)
		}
		want = []storage.Translation{
			{ID: merAilo, Word: storage.Word{ID: mer, Text: "mer"}},
			{ID: cielAilo, Word: storage.Word{ID: ciel, Text: "ciel"}},
		}
		if !slices.Equal(got, want) {
			t.Errorf("ListTranslations(ailo) = %v, want %v", got, want)
		}
	})
}

func TestListTranslationsByLanguage(t *testing.T) {
	t.Run("keys the links by the words of the language", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		vora := f.word(t, "con", "vora", "root")
		merAilo := f.translation(t, mer, ailo, "manual")
		merVora := f.translation(t, mer, vora, "manual")

		got, err := f.store.ListTranslationsByLanguage(t.Context(), f.user, storage.LangCon)
		if err != nil {
			t.Fatalf("ListTranslationsByLanguage() failed: %v", err)
		}
		want := map[int][]storage.Translation{
			ailo: {{ID: merAilo, Word: storage.Word{ID: mer, Text: "mer"}}},
			vora: {{ID: merVora, Word: storage.Word{ID: mer, Text: "mer"}}},
		}
		if len(got) != len(want) || !slices.Equal(got[ailo], want[ailo]) || !slices.Equal(got[vora], want[vora]) {
			t.Errorf("ListTranslationsByLanguage(con) = %v, want %v", got, want)
		}
	})
}

func TestDeleteTranslation(t *testing.T) {
	t.Run("deletes the link but keeps both words", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		ciel := f.word(t, "source", "ciel", "root")
		merAilo := f.translation(t, mer, ailo, "manual")
		f.translation(t, ciel, ailo, "manual")

		if err := f.store.DeleteTranslation(t.Context(), f.user, merAilo); err != nil {
			t.Fatalf("DeleteTranslation() failed: %v", err)
		}

		if got := f.countTranslations(t, mer); got != 0 {
			t.Errorf("mer has %d translations, want 0", got)
		}
		if got := f.countTranslations(t, ciel); got != 1 {
			t.Errorf("unrelated translation deleted: ciel has %d translations, want 1", got)
		}
		f.assertWordExists(t, mer)
		f.assertWordExists(t, ailo)
	})

	t.Run("reports a missing link", func(t *testing.T) {
		f := newFixture(t)

		if err := f.store.DeleteTranslation(t.Context(), f.user, 1); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("DeleteTranslation() error = %v, want ErrNotFound", err)
		}
	})
}

func TestAddTranslation(t *testing.T) {
	t.Run("links a source word to a con word, as manual", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")

		if err := f.store.AddTranslation(t.Context(), f.user, mer, ailo); err != nil {
			t.Fatalf("AddTranslation() failed: %v", err)
		}

		f.assertTranslations(t, mer, "ailo")
		f.assertOrigin(t, mer, ailo, "manual")
	})

	t.Run("links from the con side", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")

		if err := f.store.AddTranslation(t.Context(), f.user, ailo, mer); err != nil {
			t.Fatalf("AddTranslation() failed: %v", err)
		}

		f.assertTranslations(t, mer, "ailo")
		f.assertOrigin(t, mer, ailo, "manual")
	})

	t.Run("rejects words of the same language", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ciel := f.word(t, "source", "ciel", "root")

		if err := f.store.AddTranslation(t.Context(), f.user, mer, ciel); !errors.Is(err, storage.ErrSameLanguage) {
			t.Fatalf("AddTranslation() error = %v, want ErrSameLanguage", err)
		}
		if got := f.countTranslations(t, mer); got != 0 {
			t.Errorf("mer has %d translations, want 0", got)
		}
	})

	t.Run("keeps an existing link as is", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		f.translation(t, mer, ailo, "generated")

		if err := f.store.AddTranslation(t.Context(), f.user, mer, ailo); err != nil {
			t.Fatalf("AddTranslation() failed: %v", err)
		}

		f.assertTranslations(t, mer, "ailo")
		f.assertOrigin(t, mer, ailo, "generated")
	})
}

func TestAddTranslationWord(t *testing.T) {
	t.Run("creates the con root and links it", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")

		if err := f.store.AddTranslationWord(t.Context(), f.user, mer, "ailo"); err != nil {
			t.Fatalf("AddTranslationWord() failed: %v", err)
		}

		ailo := f.findWord(t, "con", "ailo", "root")
		f.assertTranslations(t, mer, "ailo")
		f.assertOrigin(t, mer, ailo, "manual")
	})

	t.Run("creates the source root from a con word", func(t *testing.T) {
		f := newFixture(t)
		ailo := f.word(t, "con", "ailo", "root")

		if err := f.store.AddTranslationWord(t.Context(), f.user, ailo, "mer"); err != nil {
			t.Fatalf("AddTranslationWord() failed: %v", err)
		}

		mer := f.findWord(t, "source", "mer", "root")
		f.assertTranslations(t, ailo, "mer")
		f.assertOrigin(t, mer, ailo, "manual")
	})

	t.Run("reuses an existing root with the same normalized form", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		f.word(t, "con", "ailo", "root")

		if err := f.store.AddTranslationWord(t.Context(), f.user, mer, "Ailo"); err != nil {
			t.Fatalf("AddTranslationWord() failed: %v", err)
		}

		f.assertTranslations(t, mer, "ailo")
	})
}

func TestSaveGeneratedTranslation(t *testing.T) {
	t.Run("creates the missing words and links, and the compound", func(t *testing.T) {
		f := newFixture(t)
		refaite := f.word(t, "source", "Refaîte", "root")
		re := f.word(t, "source", "re", "prefix")
		ki := f.word(t, "con", "ki", "prefix")
		f.translation(t, re, ki, "manual")

		err := f.store.SaveGeneratedTranslation(t.Context(), f.user, refaite, []storage.TranslationPart{
			{Kind: "prefix", SourceID: re, Source: "re", ConID: ki, Con: "ki"},
			{Kind: "root", Source: "faîte", Con: "mola"},
		})
		if err != nil {
			t.Fatalf("SaveGeneratedTranslation() failed: %v", err)
		}

		f.assertTranslations(t, re, "ki")
		f.assertTranslations(t, refaite, "kimola")
		faite := f.findWord(t, "source", "faîte", "root")
		f.assertTranslations(t, faite, "mola")
		f.assertOrigin(t, faite, f.findWord(t, "con", "mola", "root"), "generated")
	})

	t.Run("links a single part to the word itself", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")

		err := f.store.SaveGeneratedTranslation(t.Context(), f.user, mer, []storage.TranslationPart{
			{Kind: "root", SourceID: mer, Source: "mer", Con: "ailo"},
		})
		if err != nil {
			t.Fatalf("SaveGeneratedTranslation() failed: %v", err)
		}

		f.assertTranslations(t, mer, "ailo")
	})

	t.Run("reuses existing words and links", func(t *testing.T) {
		f := newFixture(t)
		mer := f.word(t, "source", "mer", "root")
		ailo := f.word(t, "con", "ailo", "root")
		f.translation(t, mer, ailo, "manual")

		err := f.store.SaveGeneratedTranslation(t.Context(), f.user, mer, []storage.TranslationPart{
			{Kind: "root", SourceID: mer, Source: "mer", Con: "ailo"},
		})
		if err != nil {
			t.Fatalf("SaveGeneratedTranslation() failed: %v", err)
		}

		f.assertTranslations(t, mer, "ailo")
	})
}
