package postgres_test

import (
	"errors"
	"testing"

	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

func TestSetGM(t *testing.T) {
	t.Run("creates the GM", func(t *testing.T) {
		store := postgres.New(pgtest.New(t))

		if err := store.SetGM(ctx, "mj"); err != nil {
			t.Fatalf("SetGM() failed: %v", err)
		}
		gm, err := store.GM(ctx)
		if err != nil || gm.Name != "mj" || gm.Role != storage.RoleGM {
			t.Errorf("GM() = %+v, %v, want mj", gm, err)
		}
	})

	t.Run("keeps the GM's account", func(t *testing.T) {
		store := postgres.New(pgtest.New(t))
		if err := store.SetGM(ctx, "mj"); err != nil {
			t.Fatalf("SetGM() failed: %v", err)
		}
		before, err := store.GM(ctx)
		if err != nil {
			t.Fatalf("GM() failed: %v", err)
		}

		for _, name := range []string{"mj", "conteur"} {
			if err := store.SetGM(ctx, name); err != nil {
				t.Fatalf("SetGM(%q) failed: %v", name, err)
			}
			gm, err := store.GM(ctx)
			if err != nil || gm.ID != before.ID || gm.Name != name {
				t.Errorf("GM() = %+v, %v, want id %d named %s", gm, err, before.ID, name)
			}
		}
	})

	t.Run("refuses a player's name", func(t *testing.T) {
		pool := pgtest.New(t)
		store := postgres.New(pool)
		exec(t, pool, `INSERT INTO users (name, role) VALUES ('ana', 'player')`)

		if err := store.SetGM(ctx, "ana"); !errors.Is(err, storage.ErrNameTaken) {
			t.Errorf("SetGM() = %v, want ErrNameTaken", err)
		}
	})
}

func TestGM(t *testing.T) {
	store := postgres.New(pgtest.New(t))

	if _, err := store.GM(ctx); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GM() = %v, want ErrNotFound before SetGM", err)
	}
}
