// Package pgtest runs the Postgres tests of a package against a throwaway
// Postgres container, each test in its own schema.
package pgtest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

const image = "postgres:18-alpine"

var (
	databaseURL string
	schemas     atomic.Int64
)

// Run starts the container, runs the tests of the package and removes the
// container; call it from TestMain. With -short, it starts no container and
// New skips the tests.
func Run(m *testing.M) int {
	flag.Parse()
	if testing.Short() {
		return m.Run()
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, image, tcpostgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgtest: unable to start Postgres: %v\n"+
			"The Postgres tests need Docker running; go test -short skips them.\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintf(os.Stderr, "pgtest: unable to remove the Postgres container: %v\n", err)
		}
	}()

	databaseURL, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgtest: unable to read the Postgres address: %v\n", err)
		return 1
	}
	return m.Run()
}

// New returns a pool on a new migrated schema, dropped at the end of the
// test. It skips the test with -short.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Postgres, skipped with -short")
	}
	if databaseURL == "" {
		t.Fatal("pgtest.Run must run the tests, from TestMain")
	}

	ctx := context.Background()
	schema := fmt.Sprintf("test_%d", schemas.Add(1))
	exec(t, "CREATE SCHEMA "+schema)

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parsing the Postgres address: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to Postgres: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		exec(t, "DROP SCHEMA "+schema+" CASCADE")
	})

	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return pool
}

// URL returns the address of the container's database, for the tests that
// open their own connection.
func URL(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Postgres, skipped with -short")
	}
	return databaseURL
}

func exec(t *testing.T, sql string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connecting to Postgres: %v", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			t.Errorf("closing Postgres connection: %v", err)
		}
	}()
	if _, err := conn.Exec(ctx, sql); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
