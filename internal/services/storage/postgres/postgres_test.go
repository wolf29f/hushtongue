package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Run(m))
}

func TestOpen(t *testing.T) {
	pool, err := postgres.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Errorf("Ping() failed: %v", err)
	}
}

func TestMigrate(t *testing.T) {
	t.Run("is idempotent", func(t *testing.T) {
		pool := pgtest.New(t)

		if err := postgres.Migrate(context.Background(), pool); err != nil {
			t.Errorf("second Migrate() failed: %v", err)
		}
	})

	t.Run("allows a single GM", func(t *testing.T) {
		pool := pgtest.New(t)
		exec(t, pool, `INSERT INTO users (name, role) VALUES ('mj', 'gm')`)

		err := execErr(pool, `INSERT INTO users (name, role) VALUES ('mj2', 'gm')`)
		assertViolation(t, err, "23505") // unique_violation
	})

	t.Run("gives no magic link to the GM", func(t *testing.T) {
		pool := pgtest.New(t)

		err := execErr(pool, `INSERT INTO users (name, role, login_token_hash) VALUES ('mj', 'gm', '\x01')`)
		assertViolation(t, err, "23514") // check_violation
	})

	t.Run("refuses a translation across two dictionaries", func(t *testing.T) {
		pool := pgtest.New(t)
		exec(t, pool, `INSERT INTO users (id, name, role) OVERRIDING SYSTEM VALUE VALUES (1, 'a', 'player'), (2, 'b', 'player')`)
		exec(t, pool, `INSERT INTO words (id, user_id, lang, text, normalized) OVERRIDING SYSTEM VALUE VALUES
			(1, 1, 'source', 'mer', 'mer'),
			(2, 2, 'con', 'ailo', 'ailo')`)

		err := execErr(pool, `INSERT INTO translations (user_id, source_word_id, con_word_id) VALUES (1, 1, 2)`)
		assertViolation(t, err, "23503") // foreign_key_violation
	})

	t.Run("deletes a user's data with the user", func(t *testing.T) {
		pool := pgtest.New(t)
		exec(t, pool, `INSERT INTO users (id, name, role) OVERRIDING SYSTEM VALUE VALUES (1, 'a', 'player')`)
		exec(t, pool, `INSERT INTO words (id, user_id, lang, text, normalized) OVERRIDING SYSTEM VALUE VALUES
			(1, 1, 'source', 'mer', 'mer'),
			(2, 1, 'con', 'ailo', 'ailo')`)
		exec(t, pool, `INSERT INTO translations (user_id, source_word_id, con_word_id) VALUES (1, 1, 2)`)
		exec(t, pool, `INSERT INTO texts (user_id, lang, content) VALUES (1, 'con', 'ailo')`)
		exec(t, pool, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ('\x01', 1, now())`)

		exec(t, pool, `DELETE FROM users WHERE id = 1`)

		var count int
		if err := pool.QueryRow(context.Background(), `SELECT
			(SELECT count(*) FROM words) + (SELECT count(*) FROM translations) +
			(SELECT count(*) FROM texts) + (SELECT count(*) FROM sessions)`).Scan(&count); err != nil {
			t.Fatalf("counting rows: %v", err)
		}
		if count != 0 {
			t.Errorf("%d rows left, want none", count)
		}
	})
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if err := execErr(pool, sql); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func execErr(pool *pgxpool.Pool, sql string) error {
	_, err := pool.Exec(context.Background(), sql)
	return err
}

func assertViolation(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("error = %v, want SQLSTATE %s", err, code)
	}
}
