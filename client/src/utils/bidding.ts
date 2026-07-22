/**
 * Tiered bid increment schedule, keyed off the listing's current price. A
 * deliberate design decision, not a framework default — a flat increment is
 * a rounding error on a cheap listing and an odd granularity on an
 * expensive one. See the README's Notable Decisions section.
 */
export function getBidIncrement(currentPrice: number): number {
  if (currentPrice < 5000) return 100
  if (currentPrice < 15000) return 250
  return 500
}
