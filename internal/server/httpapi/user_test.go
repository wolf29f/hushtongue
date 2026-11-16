package httpapi

import (
	"context"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

func TestUser(t *testing.T) {
	t.Run("returns the user of the context", func(t *testing.T) {
		user := storage.User{ID: 1, Name: "mj", Role: storage.RoleGM}

		if got := User(WithUser(context.Background(), user)); got != user {
			t.Errorf("User() = %+v, want %+v", got, user)
		}
	})

	t.Run("panics without a user", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("User() didn't panic")
			}
		}()
		User(context.Background())
	})
}
