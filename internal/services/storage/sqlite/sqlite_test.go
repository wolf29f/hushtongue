package sqlite_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
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

func insertTranslation(t *testing.T, dao *sqlite.DAO, sourceID, conID int) {
	t.Helper()
	_, err := dao.DB.Exec(
		`INSERT INTO translations (source_word_id, con_word_id) VALUES (?, ?)`,
		sourceID, conID,
	)
	if err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
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
