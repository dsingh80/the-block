// Package health is the healthcheck use-case: a narrow port over "is this
// dependency reachable right now" (guidelines/06-backend-architecture.md,
// "API design"). Kept free of net/http, redis, and pgx types -- Redis and
// Postgres satisfy this identically (pgxpool.Pool already has a matching
// Ping(context.Context) error method; only Redis needs a one-line adapter,
// since *redis.Client.Ping returns a *redis.StatusCmd, not a plain error).
package health

import "context"

// Pinger reports whether a dependency is currently reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// PingerFunc adapts a plain function to Pinger.
type PingerFunc func(ctx context.Context) error

func (f PingerFunc) Ping(ctx context.Context) error { return f(ctx) }
