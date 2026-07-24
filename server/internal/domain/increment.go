package domain

// Tiered bid-increment schedule, ported from client/src/utils/bidding.ts's
// getBidIncrement -- a deliberate design decision (see guidelines/06-backend-architecture.md
// and the client's own README Notable Decisions), not a default: a flat increment
// is a rounding error on a cheap listing and an odd granularity on an expensive one.
// Money is a whole integer everywhere (guidelines/06-backend-architecture.md), so this
// takes and returns whole currency units, never a float.
func GetBidIncrement(currentPrice int64) int64 {
	switch {
	case currentPrice < 5_000:
		return 100
	case currentPrice < 15_000:
		return 250
	default:
		return 500
	}
}
