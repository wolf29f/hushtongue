package postgres_test

import (
	"errors"
	"testing"
	"time"

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

func TestCreatePlayer(t *testing.T) {
	t.Run("creates a player", func(t *testing.T) {
		store := postgres.New(pgtest.New(t))

		created, err := store.CreatePlayer(ctx, "ana", []byte("link"))
		if err != nil {
			t.Fatalf("CreatePlayer() failed: %v", err)
		}
		found, err := store.PlayerByLink(ctx, []byte("link"))
		if err != nil || found != created || found.Name != "ana" || found.Role != storage.RolePlayer {
			t.Errorf("PlayerByLink() = %+v, %v, want %+v", found, err, created)
		}
	})

	for _, existing := range []string{"player", "gm"} {
		t.Run("refuses the name of the "+existing, func(t *testing.T) {
			pool := pgtest.New(t)
			store := postgres.New(pool)
			exec(t, pool, `INSERT INTO users (name, role) VALUES ('ana', '`+existing+`')`)

			if _, err := store.CreatePlayer(ctx, "ana", []byte("link")); !errors.Is(err, storage.ErrNameTaken) {
				t.Errorf("CreatePlayer() = %v, want ErrNameTaken", err)
			}
		})
	}
}

func TestListPlayers(t *testing.T) {
	store := postgres.New(pgtest.New(t))
	if err := store.SetGM(ctx, "mj"); err != nil {
		t.Fatalf("SetGM() failed: %v", err)
	}
	for _, name := range []string{"zoé", "ana"} {
		if _, err := store.CreatePlayer(ctx, name, []byte(name)); err != nil {
			t.Fatalf("CreatePlayer(%q) failed: %v", name, err)
		}
	}

	players, err := store.ListPlayers(ctx)
	if err != nil || len(players) != 2 || players[0].Name != "zoé" || players[1].Name != "ana" {
		t.Errorf("ListPlayers() = %+v, %v, want zoé then ana", players, err)
	}
}

func TestPlayerByLink(t *testing.T) {
	store := postgres.New(pgtest.New(t))

	if _, err := store.PlayerByLink(ctx, []byte("unknown")); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("PlayerByLink() = %v, want ErrNotFound", err)
	}
}

func TestSetPlayerLink(t *testing.T) {
	t.Run("replaces the link and ends the sessions", func(t *testing.T) {
		store := postgres.New(pgtest.New(t))
		player, err := store.CreatePlayer(ctx, "ana", []byte("old"))
		if err != nil {
			t.Fatalf("CreatePlayer() failed: %v", err)
		}
		if err := store.CreateSession(ctx, []byte("session"), player.ID, time.Hour); err != nil {
			t.Fatalf("CreateSession() failed: %v", err)
		}

		if err := store.SetPlayerLink(ctx, player.ID, []byte("new")); err != nil {
			t.Fatalf("SetPlayerLink() failed: %v", err)
		}
		if _, err := store.PlayerByLink(ctx, []byte("old")); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("PlayerByLink(old) = %v, want ErrNotFound", err)
		}
		if found, err := store.PlayerByLink(ctx, []byte("new")); err != nil || found.ID != player.ID {
			t.Errorf("PlayerByLink(new) = %+v, %v, want ana", found, err)
		}
		if _, err := store.Session(ctx, []byte("session")); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Session() = %v, want ErrNotFound", err)
		}
	})

	t.Run("leaves the GM", func(t *testing.T) {
		store := postgres.New(pgtest.New(t))
		if err := store.SetGM(ctx, "mj"); err != nil {
			t.Fatalf("SetGM() failed: %v", err)
		}
		gm, err := store.GM(ctx)
		if err != nil {
			t.Fatalf("GM() failed: %v", err)
		}

		if err := store.SetPlayerLink(ctx, gm.ID, []byte("link")); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("SetPlayerLink(GM) = %v, want ErrNotFound", err)
		}
	})
}

func TestDeletePlayer(t *testing.T) {
	t.Run("deletes the player", func(t *testing.T) {
		store := postgres.New(pgtest.New(t))
		player, err := store.CreatePlayer(ctx, "ana", []byte("link"))
		if err != nil {
			t.Fatalf("CreatePlayer() failed: %v", err)
		}

		if err := store.DeletePlayer(ctx, player.ID); err != nil {
			t.Fatalf("DeletePlayer() failed: %v", err)
		}
		if err := store.DeletePlayer(ctx, player.ID); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("second DeletePlayer() = %v, want ErrNotFound", err)
		}
	})

	t.Run("leaves the GM", func(t *testing.T) {
		store := postgres.New(pgtest.New(t))
		if err := store.SetGM(ctx, "mj"); err != nil {
			t.Fatalf("SetGM() failed: %v", err)
		}
		gm, err := store.GM(ctx)
		if err != nil {
			t.Fatalf("GM() failed: %v", err)
		}

		if err := store.DeletePlayer(ctx, gm.ID); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("DeletePlayer(GM) = %v, want ErrNotFound", err)
		}
	})
}
