// Package sessions is the session use-case: opaque, unauthenticated identity
// (guidelines/06-backend-architecture.md, "Sessions & security").
package sessions

import (
	"context"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// Store is the session port. Touch ensures a session exists for token, creating
// a brand-new one (with a freshly generated token, ignoring whatever was passed
// in) whenever token is empty or unknown -- expired and forged tokens are
// indistinguishable from "no session yet" on purpose. Every call, new or
// existing, slides the session's TTL forward.
type Store interface {
	Touch(ctx context.Context, token string) (session domain.Session, isNew bool, err error)
}
