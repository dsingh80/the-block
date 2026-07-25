package domain

// Tiered bid-increment schedule, ported from client/src/utils/bidding.ts's
// getBidIncrement -- a deliberate design decision (see guidelines/06-backend-architecture.md
// and the client's own README Notable Decisions), not a default: a flat increment
// is a rounding error on a cheap listing and an odd granularity on an expensive one.
// Money is a whole integer everywhere (guidelines/06-backend-architecture.md), so this
// takes and returns whole currency units, never a float.
//
// The boundaries are exported constants, not inlined in GetBidIncrement's switch,
// specifically so the place-bid Lua script (internal/platform/redisstore) can take
// them as ARGV instead of re-hardcoding the schedule -- if these ever drift from
// what the script is passed, it shows up as a values-only diff in review.
const (
	Tier1Ceiling   int64 = 5_000
	Tier1Increment int64 = 100
	Tier2Ceiling   int64 = 15_000
	Tier2Increment int64 = 250
	Tier3Increment int64 = 500
)

func GetBidIncrement(currentPrice int64) int64 {
	switch {
	case currentPrice < Tier1Ceiling:
		return Tier1Increment
	case currentPrice < Tier2Ceiling:
		return Tier2Increment
	default:
		return Tier3Increment
	}
}
