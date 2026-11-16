package httpapi

import (
	"context"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type userKey struct{}

// WithUser returns a copy of ctx carrying the logged in user.
func WithUser(ctx context.Context, user storage.User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// User returns the logged in user that middleware.RequireSession put in ctx.
// It panics without one: a handler reading the user on a route without
// RequireSession is a wiring bug.
func User(ctx context.Context) storage.User {
	user, ok := ctx.Value(userKey{}).(storage.User)
	if !ok {
		panic("httpapi: no user in the request context, the route misses middleware.RequireSession")
	}
	return user
}
