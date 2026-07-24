package domain

import "time"

// Session is an opaque, unauthenticated identity (guidelines/06-backend-architecture.md,
// "Sessions & security") -- a server-side lookup key only, never a JWT, never PII.
type Session struct {
	Token      string
	CreatedAt  time.Time
	LastSeenAt time.Time
}
