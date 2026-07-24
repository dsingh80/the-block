package domain

import "time"

type Lifecycle string

const (
	LifecycleUpcoming Lifecycle = "upcoming"
	LifecycleActive   Lifecycle = "active"
	LifecycleEnded    Lifecycle = "ended"
)

// DeriveLifecycle mirrors client/src/utils/lifecycle.ts's deriveLifecycle: active
// includes the start instant and excludes the end instant. Unlike the client (which
// derives end from a hardcoded 24h duration), this takes an explicit end time --
// auction duration is per-listing data now, not a constant (guidelines/06-backend-architecture.md).
func DeriveLifecycle(start, end, now time.Time) Lifecycle {
	if start.After(now) {
		return LifecycleUpcoming
	}
	if now.Before(end) {
		return LifecycleActive
	}
	return LifecycleEnded
}
