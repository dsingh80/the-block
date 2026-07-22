import { reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { vehiclesById } from '@/data/vehicles'
import { deriveLifecycle } from '@/utils/lifecycle'
import { getBidIncrement } from '@/utils/bidding'
import { currency } from '@/utils/format'
import { useClockStore } from './clock'
import type { BidOverride } from '@/types/listing'

export type BidResult = { ok: true } | { ok: false; error: string }

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
    const vehicle = vehiclesById.get(id)
    return vehicle ? (vehicle.current_bid ?? vehicle.starting_bid) : 0
  }

  function currentBidCountFor(id: string): number {
    const override = overrides[id]
    if (override) return override.bidCount
    return vehiclesById.get(id)?.bid_count ?? 0
  }

  /**
   * The real validation gate — re-derives the minimum from the tiered
   * schedule and re-checks the listing is still active right now, not just
   * at whatever moment a BidPanel happened to render. See
   * guidelines/03-guardrails.md.
   */
  function placeBid(id: string, amount: number): BidResult {
    const vehicle = vehiclesById.get(id)
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

    overrides[id] = {
      currentPrice: amount,
      bidCount: currentBidCountFor(id) + 1,
      hasUserBid: true,
      isUserHighBidder: true,
      isUserOutbid: false,
      purchased: overrides[id]?.purchased ?? false,
      purchasedAt: overrides[id]?.purchasedAt ?? null,
    }
    return { ok: true }
  }

  function buyNow(id: string): BidResult {
    const vehicle = vehiclesById.get(id)
    if (!vehicle) return { ok: false, error: 'Vehicle not found.' }
    if (vehicle.buy_now_price == null) {
      return { ok: false, error: 'This listing has no Buy Now price.' }
    }

    overrides[id] = {
      currentPrice: vehicle.buy_now_price,
      bidCount: currentBidCountFor(id) + 1,
      hasUserBid: true,
      isUserHighBidder: true,
      isUserOutbid: false,
      purchased: true,
      purchasedAt: Date.now(),
    }
    justBoughtId.value = id
    return { ok: true }
  }

  return { overrides, justBoughtId, placeBid, buyNow }
})
