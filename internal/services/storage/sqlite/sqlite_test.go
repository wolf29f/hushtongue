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
		defer dao.Close()

		if _, err := os.Stat(path); err != nil {
			t.Errorf("Load() did not create database file: %v", err)
		}
		assertForeignKeysEnabled(t, dao)
		assertSchemaMigrated(t, dao)
	})

	t.Run("creates missing parent directories", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "dir", "test.db")

		dao, err := sqlite.Load(path)
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		defer dao.Close()

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
		defer dao.Close()

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

	t.Run("running the migration again on an existing database is a no-op", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.db")

		dao, err := sqlite.Load(path)
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		dao.Close()

		// Load() runs the migration on every startup, not just on a fresh
		// database, so reopening the same file must not error.
		dao, err = sqlite.Load(path)
		if err != nil {
			t.Fatalf("Load() is not idempotent: %v", err)
		}
		defer dao.Close()
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
			t.Errorf("table %q not created by Load(): %v", table, err)
		}
	}

	for _, index := range []string{"idx_translations_source", "idx_translations_con"} {
		var name string
		err := dao.DB.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, index,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not created by Load(): %v", index, err)
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
