// Package ctxkeys provides a generic, type-safe context key mechanism that
// prevents collisions between packages using context.WithValue.
package ctxkeys

import "context"

// Key is a typed context key. Create one per value type you store in context.
//
//	var UserIDKey = ctxkeys.New[string]("user_id")
//	ctx = UserIDKey.Set(ctx, "abc123")
//	id, ok := UserIDKey.Get(ctx)
type Key[T any] struct {
	name string
}

// New creates a named Key for type T. The name is only used for debugging.
func New[T any](name string) Key[T] {
	return Key[T]{name: name}
}

// Set returns a new context with the value stored under this key.
func (k Key[T]) Set(ctx context.Context, v T) context.Context {
	return context.WithValue(ctx, k, v)
}

// Get retrieves the value stored under this key.
func (k Key[T]) Get(ctx context.Context) (T, bool) {
	v, ok := ctx.Value(k).(T)
	return v, ok
}

// MustGet retrieves the value stored under this key.
// Panics if the value is not present — only use when presence is guaranteed by middleware.
func (k Key[T]) MustGet(ctx context.Context) T {
	v, ok := ctx.Value(k).(T)
	if !ok {
		panic("ctxkeys: value not found for key " + k.name)
	}
	return v
}
