import { reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { deriveLifecycle } from '@/utils/lifecycle'
import { getBidIncrement } from '@/utils/bidding'
import { currency } from '@/utils/format'
import { useClockStore } from './clock'
import { useInventoryFiltersStore } from './inventoryFilters'
import { placeBid as apiPlaceBid, buyNow as apiBuyNow } from '@/services/api/listings'
import { ApiError } from '@/services/api/client'
import type { ApiBidAccept } from '@/services/api/types'
import type { BidOverride } from '@/types/listing'

export type BidResult = { ok: true } | { ok: false; error: string }

function overrideFromAccept(
  accepted: ApiBidAccept,
  purchased: boolean,
  purchasedAt: number | null,
): BidOverride {
  return {
    currentPrice: accepted.current_bid,
    bidCount: accepted.bid_count,
    hasUserBid: accepted.viewer.has_bid,
    isUserHighBidder: accepted.viewer.is_high_bidder,
    isUserOutbid: accepted.viewer.is_outbid,
    purchased,
    purchasedAt,
  }
}

function messageFor(err: unknown): string {
  return err instanceof ApiError ? err.message : 'Something went wrong. Please try again.'
}

/**
 * The single global, always-subscribed source of truth for the user's
 * bidding relationship to every listing — not scoped to whichever view
 * happens to be mounted. See guidelines/02-design-patterns.md #3.
 */
export const useBidsStore = defineStore('bids', () => {
  const overrides = reactive<Record<string, BidOverride>>({})
  const justBoughtId = ref<string | null>(null)

  function currentPriceFor(id: string): number {
    const override = overrides[id]
    if (override) return override.currentPrice
    const vehicle = useInventoryFiltersStore().vehiclesById[id]
    return vehicle ? (vehicle.current_bid ?? vehicle.starting_bid) : 0
  }

  /**
   * The client-side checks below are fast UX feedback only, so a doomed
   * request never even reaches the network — the server's Lua accept path is
   * what actually validates and accepts/rejects (guidelines/06-backend-architecture.md,
   * "Idempotency"; guidelines/03-guardrails.md's "validate at the boundary,
   * trust nothing else"). `overrides[id]` is populated from the server's
   * response, not from what the client asked for.
   */
  async function placeBid(id: string, amount: number): Promise<BidResult> {
    const vehicle = useInventoryFiltersStore().vehiclesById[id]
    if (!vehicle) return { ok: false, error: 'Vehicle not found.' }

    const clock = useClockStore()
    if (deriveLifecycle(vehicle.auction_start, clock.effectiveNow) !== 'active') {
      return { ok: false, error: 'This auction is no longer active.' }
    }

    const current = currentPriceFor(id)
    const minimum = current + getBidIncrement(current)
    if (!Number.isFinite(amount) || amount < minimum) {
      return { ok: false, error: `Enter at least ${currency(minimum)}.` }
    }

    try {
      const accepted = await apiPlaceBid(id, amount)
      overrides[id] = overrideFromAccept(accepted, overrides[id]?.purchased ?? false, overrides[id]?.purchasedAt ?? null)
      return { ok: true }
    } catch (err) {
      return { ok: false, error: messageFor(err) }
    }
  }

  async function buyNow(id: string): Promise<BidResult> {
    const vehicle = useInventoryFiltersStore().vehiclesById[id]
    if (!vehicle) return { ok: false, error: 'Vehicle not found.' }
    if (vehicle.buy_now_price == null) {
      return { ok: false, error: 'This listing has no Buy Now price.' }
    }

    try {
      const accepted = await apiBuyNow(id)
      overrides[id] = overrideFromAccept(accepted, true, Date.parse(accepted.accepted_at))
      justBoughtId.value = id
      return { ok: true }
    } catch (err) {
      return { ok: false, error: messageFor(err) }
    }
  }

  return { overrides, justBoughtId, placeBid, buyNow }
})
