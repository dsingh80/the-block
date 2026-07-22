import type { Lifecycle } from '@/types/listing'
import { AUCTION_DURATION_HOURS } from './constants'

const AUCTION_DURATION_MS = AUCTION_DURATION_HOURS * 60 * 60 * 1000

/**
 * upcoming: before auction_start. active: from auction_start up to (not
 * including) auction_start + 24h. ended: at or after that. The 24h duration
 * is an unconfirmed assumption carried from the source requirements — see
 * the README's Assumptions and Scope section.
 */
export function deriveLifecycle(auctionStart: string, effectiveNowMs: number): Lifecycle {
  const startMs = new Date(auctionStart).getTime()
  const endMs = startMs + AUCTION_DURATION_MS

  if (startMs > effectiveNowMs) return 'upcoming'
  if (effectiveNowMs < endMs) return 'active'
  return 'ended'
}
