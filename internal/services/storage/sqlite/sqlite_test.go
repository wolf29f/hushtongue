package sqlite_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adrg/xdg"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/sqlite"
)

func TestLoad(t *testing.T) {
	t.Run("opens and creates a database file at the given path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.db")

		dao, err := sqlite.Load(path)
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		defer closeDAO(t, dao)

		if _, err := os.Stat(path); err != nil {
			t.Errorf("Load() did not create database file: %v", err)
		}
		assertForeignKeysEnabled(t, dao)
	})

	t.Run("creates missing parent directories", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "dir", "test.db")

		dao, err := sqlite.Load(path)
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		defer closeDAO(t, dao)

		if _, err := os.Stat(path); err != nil {
			t.Errorf("Load() did not create database file: %v", err)
		}
	})

	t.Run("defaults to the XDG data home when path is empty", func(t *testing.T) {
		// xdg.DataHome is resolved once at package init from the real
		// environment, so overriding it here (instead of the env var, which
		// would arrive too late) is the only way to exercise the default-path
		// branch without writing into the developer's actual data directory.
		original := xdg.DataHome
		xdg.DataHome = t.TempDir()
		defer func() { xdg.DataHome = original }()

		dao, err := sqlite.Load("")
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		defer closeDAO(t, dao)

		want := filepath.Join(xdg.DataHome, "hushtongue", "hushtongue.db")
		if _, err := os.Stat(want); err != nil {
			t.Errorf("Load() did not create database file at %s: %v", want, err)
		}
	})

	t.Run("errors when the path cannot be created", func(t *testing.T) {
		// A regular file can never be turned into a parent directory.
		blocker := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := sqlite.Load(filepath.Join(blocker, "test.db"))
		if err == nil {
			t.Fatal("Load() succeeded unexpectedly")
		}
	})

}

func TestInit(t *testing.T) {
	t.Run("creates the schema", func(t *testing.T) {
		dao := newTestDAO(t)
		assertSchemaMigrated(t, dao)
	})

	t.Run("running the migration again on an existing database is a no-op", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.db")

		dao, err := sqlite.Load(path)
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		if err := dao.Init(); err != nil {
			t.Fatalf("Init() failed: %v", err)
		}
		closeDAO(t, dao)

		// Init() runs the migration on every startup, not just on a fresh
		// database, so running it on the reopened file must not error.
		dao, err = sqlite.Load(path)
		if err != nil {
			t.Fatalf("Load() failed on reopen: %v", err)
		}
		defer closeDAO(t, dao)
		if err := dao.Init(); err != nil {
			t.Fatalf("Init() is not idempotent: %v", err)
		}
	})
}

func assertForeignKeysEnabled(t *testing.T, dao *sqlite.DAO) {
	t.Helper()
	var enabled int
	if err := dao.DB.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("querying foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Errorf("foreign_keys = %d, want 1", enabled)
	}
}

func assertSchemaMigrated(t *testing.T, dao *sqlite.DAO) {
	t.Helper()

	for _, table := range []string{"words", "translations"} {
		var name string
		err := dao.DB.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not created by Init(): %v", table, err)
		}
	}

	for _, index := range []string{"idx_translations_source", "idx_translations_con"} {
		var name string
		err := dao.DB.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, index,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not created by Init(): %v", index, err)
		}
	}

	// The words.lang CHECK constraint must reject anything outside ('source', 'con').
	_, err := dao.DB.Exec(`INSERT INTO words (lang, text, normalized) VALUES ('fr', 'mer', 'mer')`)
	if err == nil {
		t.Error("expected a CHECK constraint violation for an invalid lang, got none")
	}

	res, err := dao.DB.Exec(`INSERT INTO words (lang, text, normalized) VALUES ('source', 'mer', 'mer')`)
	if err != nil {
		t.Fatalf("inserting a valid word: %v", err)
	}
	wordID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("reading inserted word id: %v", err)
	}

	// Foreign keys must actually be enforced (Load enables the pragma):
	// a translations row pointing at a nonexistent con_word_id must be rejected.
	_, err = dao.DB.Exec(
		`INSERT INTO translations (source_word_id, con_word_id) VALUES (?, ?)`,
		wordID, 9999,
	)
	if err == nil {
		t.Error("expected a foreign key violation for a nonexistent con_word_id, got none")
	}
}

func TestSaveWord(t *testing.T) {
	t.Run("renames the word to another case of itself", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")

		got, err := dao.SaveWord(storage.WordDetails{ID: mer, Language: "source", Text: "Mer", Kind: "root"})
		if err != nil {
			t.Fatalf("SaveWord() failed: %v", err)
		}
		if got.Text != "Mer" || got.Normalized != "mer" {
			t.Errorf("SaveWord() = %+v, want text Mer normalized mer", got)
		}
	})

	t.Run("reports the existing word on collision", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ciel := insertWord(t, dao, "source", "ciel", "root")

		_, err := dao.SaveWord(storage.WordDetails{ID: ciel, Language: "source", Text: "Mèr", Kind: "root"})
		var exists *storage.WordExistsError
		if !errors.As(err, &exists) {
			t.Fatalf("SaveWord() error = %v, want a WordExistsError", err)
		}
		if exists.ExistingID != mer {
			t.Errorf("ExistingID = %d, want %d", exists.ExistingID, mer)
		}
	})

	t.Run("allows the same text with another kind", func(t *testing.T) {
		dao := newTestDAO(t)
		insertWord(t, dao, "source", "re", "prefix")
		ciel := insertWord(t, dao, "source", "ciel", "root")

		if _, err := dao.SaveWord(storage.WordDetails{ID: ciel, Language: "source", Text: "re", Kind: "root"}); err != nil {
			t.Fatalf("SaveWord() failed: %v", err)
		}
	})
}

func TestMergeWords(t *testing.T) {
	t.Run("moves the source word's links and deletes it", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ocean := insertWord(t, dao, "source", "ocean", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		vora := insertWord(t, dao, "con", "vora", "root")
		if _, err := dao.DB.Exec(
			`INSERT INTO translations (source_word_id, con_word_id, source) VALUES (?, ?, 'generated')`,
			mer, ailo,
		); err != nil {
			t.Fatalf("inserting translation: %v", err)
		}
		insertTranslation(t, dao, ocean, ailo)
		insertTranslation(t, dao, ocean, vora)

		if err := dao.MergeWords(ocean, mer); err != nil {
			t.Fatalf("MergeWords() failed: %v", err)
		}

		assertTranslations(t, dao, mer, "ailo", "vora")
		assertLinkSource(t, dao, mer, ailo, "generated")
		if _, err := dao.GetWord(ocean); err == nil {
			t.Error("merged word still exists")
		}
	})

	t.Run("moves the con word's links", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		aylo := insertWord(t, dao, "con", "aylo", "root")
		insertTranslation(t, dao, mer, aylo)

		if err := dao.MergeWords(aylo, ailo); err != nil {
			t.Fatalf("MergeWords() failed: %v", err)
		}

		assertTranslations(t, dao, ailo, "mer")
		assertTranslations(t, dao, mer, "ailo")
	})

	t.Run("rejects words of different languages", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")

		if err := dao.MergeWords(mer, ailo); err == nil {
			t.Fatal("MergeWords() succeeded unexpectedly")
		}
		if _, err := dao.GetWord(mer); err != nil {
			t.Errorf("word deleted despite the error: %v", err)
		}
	})
}

func TestDeleteWord(t *testing.T) {
	t.Run("deletes the word and its translations", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		ciel := insertWord(t, dao, "source", "ciel", "root")
		insertTranslation(t, dao, mer, ailo)
		insertTranslation(t, dao, ciel, ailo)

		if err := dao.DeleteWord(mer); err != nil {
			t.Fatalf("DeleteWord() failed: %v", err)
		}

		if _, err := dao.GetWord(mer); err == nil {
			t.Error("deleted word is still stored")
		}
		if _, err := dao.GetWord(ailo); err != nil {
			t.Errorf("translated word was deleted: %v", err)
		}
		if got := countTranslations(t, dao, mer); got != 0 {
			t.Errorf("deleted word still has %d translations, want 0", got)
		}
		if got := countTranslations(t, dao, ciel); got != 1 {
			t.Errorf("other word has %d translations, want 1", got)
		}
	})
}

func TestChangeWordKind(t *testing.T) {
	t.Run("changes the kind and deletes the word's translations", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		ciel := insertWord(t, dao, "source", "ciel", "root")
		vora := insertWord(t, dao, "con", "vora", "root")
		insertTranslation(t, dao, mer, ailo)
		insertTranslation(t, dao, ciel, ailo)
		insertTranslation(t, dao, ciel, vora)

		word, err := dao.ChangeWordKind(ailo, "prefix")
		if err != nil {
			t.Fatalf("ChangeWordKind() failed: %v", err)
		}

		if word.Kind != "prefix" {
			t.Errorf("returned kind = %q, want %q", word.Kind, "prefix")
		}
		stored, err := dao.GetWord(ailo)
		if err != nil {
			t.Fatalf("GetWord() failed: %v", err)
		}
		if stored.Kind != "prefix" {
			t.Errorf("stored kind = %q, want %q", stored.Kind, "prefix")
		}
		if got := countTranslations(t, dao, ailo); got != 0 {
			t.Errorf("word still has %d translations, want 0", got)
		}
		if got := countTranslations(t, dao, ciel); got != 1 {
			t.Errorf("unrelated translation deleted: ciel has %d translations, want 1", got)
		}
	})

	t.Run("keeps the translations when the change is rejected", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		insertWord(t, dao, "source", "mer", "prefix")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		insertTranslation(t, dao, mer, ailo)

		if _, err := dao.ChangeWordKind(mer, "prefix"); err == nil {
			t.Fatal("ChangeWordKind() succeeded unexpectedly")
		}

		stored, err := dao.GetWord(mer)
		if err != nil {
			t.Fatalf("GetWord() failed: %v", err)
		}
		if stored.Kind != "root" {
			t.Errorf("stored kind = %q, want %q", stored.Kind, "root")
		}
		if got := countTranslations(t, dao, mer); got != 1 {
			t.Errorf("word has %d translations, want 1", got)
		}
	})
}

func TestListTranslations(t *testing.T) {
	t.Run("lists the words on the other side, in insertion order", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		vora := insertWord(t, dao, "con", "vora", "root")
		ciel := insertWord(t, dao, "source", "ciel", "root")
		merVora := insertTranslation(t, dao, mer, vora)
		merAilo := insertTranslation(t, dao, mer, ailo)
		cielAilo := insertTranslation(t, dao, ciel, ailo)

		got, err := dao.ListTranslations(mer)
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

		got, err = dao.ListTranslations(ailo)
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

func TestDeleteTranslation(t *testing.T) {
	t.Run("deletes the link but keeps both words", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		ciel := insertWord(t, dao, "source", "ciel", "root")
		merAilo := insertTranslation(t, dao, mer, ailo)
		insertTranslation(t, dao, ciel, ailo)

		if err := dao.DeleteTranslation(merAilo); err != nil {
			t.Fatalf("DeleteTranslation() failed: %v", err)
		}

		if got := countTranslations(t, dao, mer); got != 0 {
			t.Errorf("mer has %d translations, want 0", got)
		}
		if got := countTranslations(t, dao, ciel); got != 1 {
			t.Errorf("unrelated translation deleted: ciel has %d translations, want 1", got)
		}
		for _, id := range []int{mer, ailo} {
			if _, err := dao.GetWord(id); err != nil {
				t.Errorf("word %d was deleted: %v", id, err)
			}
		}
	})
}

func TestAddTranslation(t *testing.T) {
	t.Run("links a source word to a con word, as manual", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")

		if err := dao.AddTranslation(mer, ailo); err != nil {
			t.Fatalf("AddTranslation() failed: %v", err)
		}

		assertTranslations(t, dao, mer, "ailo")
		assertLinkSource(t, dao, mer, ailo, "manual")
	})

	t.Run("links from the con side", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")

		if err := dao.AddTranslation(ailo, mer); err != nil {
			t.Fatalf("AddTranslation() failed: %v", err)
		}

		assertTranslations(t, dao, mer, "ailo")
		assertLinkSource(t, dao, mer, ailo, "manual")
	})

	t.Run("rejects words of the same language", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ciel := insertWord(t, dao, "source", "ciel", "root")

		err := dao.AddTranslation(mer, ciel)
		if !errors.Is(err, storage.ErrSameLanguage) {
			t.Fatalf("AddTranslation() error = %v, want %v", err, storage.ErrSameLanguage)
		}
		if got := countTranslations(t, dao, mer); got != 0 {
			t.Errorf("mer has %d translations, want 0", got)
		}
	})

	t.Run("keeps an existing link as is", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		if _, err := dao.DB.Exec(
			`INSERT INTO translations (source_word_id, con_word_id, source) VALUES (?, ?, 'generated')`,
			mer, ailo,
		); err != nil {
			t.Fatalf("inserting translation: %v", err)
		}

		if err := dao.AddTranslation(mer, ailo); err != nil {
			t.Fatalf("AddTranslation() failed: %v", err)
		}

		assertTranslations(t, dao, mer, "ailo")
		assertLinkSource(t, dao, mer, ailo, "generated")
	})
}

func TestAddTranslationWord(t *testing.T) {
	t.Run("creates the con root and links it", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")

		if err := dao.AddTranslationWord(mer, "ailo"); err != nil {
			t.Fatalf("AddTranslationWord() failed: %v", err)
		}

		ailo := findWord(t, dao, "con", "ailo", "root")
		assertTranslations(t, dao, mer, "ailo")
		assertLinkSource(t, dao, mer, ailo, "manual")
	})

	t.Run("creates the source root from a con word", func(t *testing.T) {
		dao := newTestDAO(t)
		ailo := insertWord(t, dao, "con", "ailo", "root")

		if err := dao.AddTranslationWord(ailo, "mer"); err != nil {
			t.Fatalf("AddTranslationWord() failed: %v", err)
		}

		mer := findWord(t, dao, "source", "mer", "root")
		assertTranslations(t, dao, ailo, "mer")
		assertLinkSource(t, dao, mer, ailo, "manual")
	})

	t.Run("reuses an existing root with the same normalized form", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		insertWord(t, dao, "con", "ailo", "root")

		if err := dao.AddTranslationWord(mer, "Ailo"); err != nil {
			t.Fatalf("AddTranslationWord() failed: %v", err)
		}

		assertTranslations(t, dao, mer, "ailo")
	})
}

func TestListWordDetails(t *testing.T) {
	t.Run("lists the words of the language with their details", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		re := insertWord(t, dao, "source", "re", "prefix")
		insertWord(t, dao, "con", "ailo", "root")

		got, err := dao.ListWordDetails(storage.LangSource)
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

func TestSaveGeneratedTranslation(t *testing.T) {
	t.Run("creates the missing words and links, and the compound", func(t *testing.T) {
		dao := newTestDAO(t)
		refaite := insertWord(t, dao, "source", "Refaîte", "root")
		re := insertWord(t, dao, "source", "re", "prefix")
		ki := insertWord(t, dao, "con", "ki", "prefix")
		insertTranslation(t, dao, re, ki)

		err := dao.SaveGeneratedTranslation(refaite, []storage.TranslationPart{
			{Kind: "prefix", SourceID: re, Source: "re", ConID: ki, Con: "ki"},
			{Kind: "root", Source: "faîte", Con: "mola"},
		})
		if err != nil {
			t.Fatalf("SaveGeneratedTranslation() failed: %v", err)
		}

		assertTranslations(t, dao, re, "ki")
		assertTranslations(t, dao, refaite, "kimola")
		faite := findWord(t, dao, "source", "faîte", "root")
		assertTranslations(t, dao, faite, "mola")
	})

	t.Run("links a single part to the word itself", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")

		err := dao.SaveGeneratedTranslation(mer, []storage.TranslationPart{
			{Kind: "root", SourceID: mer, Source: "mer", Con: "ailo"},
		})
		if err != nil {
			t.Fatalf("SaveGeneratedTranslation() failed: %v", err)
		}

		assertTranslations(t, dao, mer, "ailo")
	})

	t.Run("reuses existing words and links", func(t *testing.T) {
		dao := newTestDAO(t)
		mer := insertWord(t, dao, "source", "mer", "root")
		ailo := insertWord(t, dao, "con", "ailo", "root")
		insertTranslation(t, dao, mer, ailo)

		err := dao.SaveGeneratedTranslation(mer, []storage.TranslationPart{
			{Kind: "root", SourceID: mer, Source: "mer", Con: "ailo"},
		})
		if err != nil {
			t.Fatalf("SaveGeneratedTranslation() failed: %v", err)
		}

		assertTranslations(t, dao, mer, "ailo")
	})
}

func findWord(t *testing.T, dao *sqlite.DAO, lang, text, kind string) int {
	t.Helper()
	var id int
	err := dao.DB.QueryRow(
		`SELECT id FROM words WHERE lang = ? AND text = ? AND kind = ?`,
		lang, text, kind,
	).Scan(&id)
	if err != nil {
		t.Fatalf("finding word %q: %v", text, err)
	}
	return id
}

func assertTranslations(t *testing.T, dao *sqlite.DAO, wordID int, want ...string) {
	t.Helper()
	translations, err := dao.ListTranslations(wordID)
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

func assertLinkSource(t *testing.T, dao *sqlite.DAO, sourceID, conID int, want string) {
	t.Helper()
	var got string
	err := dao.DB.QueryRow(
		`SELECT source FROM translations WHERE source_word_id = ? AND con_word_id = ?`,
		sourceID, conID,
	).Scan(&got)
	if err != nil {
		t.Fatalf("reading link source: %v", err)
	}
	if got != want {
		t.Errorf("link source = %q, want %q", got, want)
	}
}

func newTestDAO(t *testing.T) *sqlite.DAO {
	t.Helper()
	dao, err := sqlite.Load(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	t.Cleanup(func() { closeDAO(t, dao) })
	if err := dao.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	return dao
}

func insertWord(t *testing.T, dao *sqlite.DAO, lang, text, kind string) int {
	t.Helper()
	res, err := dao.DB.Exec(
		`INSERT INTO words (lang, text, normalized, kind) VALUES (?, ?, ?, ?)`,
		lang, text, text, kind,
	)
	if err != nil {
		t.Fatalf("inserting word %q: %v", text, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("reading inserted word id: %v", err)
	}
	return int(id)
}

func insertTranslation(t *testing.T, dao *sqlite.DAO, sourceID, conID int) int {
	t.Helper()
	res, err := dao.DB.Exec(
		`INSERT INTO translations (source_word_id, con_word_id) VALUES (?, ?)`,
		sourceID, conID,
	)
	if err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("reading inserted translation id: %v", err)
	}
	return int(id)
}

func countTranslations(t *testing.T, dao *sqlite.DAO, wordID int) int {
	t.Helper()
	var count int
	err := dao.DB.QueryRow(
		`SELECT COUNT(*) FROM translations WHERE source_word_id = ? OR con_word_id = ?`,
		wordID, wordID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("counting translations: %v", err)
	}
	return count
}

func closeDAO(t *testing.T, dao *sqlite.DAO) {
	t.Helper()
	if err := dao.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}
