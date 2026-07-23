import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useBidsStore } from './bids'
import { useClockStore } from './clock'
import { vehicles } from '@/data/vehicles'
import { deriveLifecycle } from '@/utils/lifecycle'
import { getBidIncrement } from '@/utils/bidding'

const HOUR_MS = 60 * 60 * 1000

describe('useBidsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('accepts a bid at the tiered minimum on an active listing', () => {
    const vehicle = vehicles[0]
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS
    expect(deriveLifecycle(vehicle.auction_start, clock.effectiveNow)).toBe('active')

    const bids = useBidsStore()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)

    const result = bids.placeBid(vehicle.id, minimum)

    expect(result.ok).toBe(true)
    expect(bids.overrides[vehicle.id]).toMatchObject({
      currentPrice: minimum,
      hasUserBid: true,
      isUserHighBidder: true,
      isUserOutbid: false,
    })
  })

  it('rejects a bid below the tiered minimum', () => {
    const vehicle = vehicles[0]
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS

    const bids = useBidsStore()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)

    const result = bids.placeBid(vehicle.id, minimum - 1)

    expect(result.ok).toBe(false)
    expect(bids.overrides[vehicle.id]).toBeUndefined()
  })

  it('rejects a bid on a listing that is no longer active', () => {
    const vehicle = vehicles[0]
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + 48 * HOUR_MS
    expect(deriveLifecycle(vehicle.auction_start, clock.effectiveNow)).toBe('ended')

    const bids = useBidsStore()
    const current = vehicle.current_bid ?? vehicle.starting_bid

    const result = bids.placeBid(vehicle.id, current + 1_000_000)

    expect(result.ok).toBe(false)
  })

  it('rejects a bid on a listing that has not started yet', () => {
    const vehicle = vehicles[0]
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() - HOUR_MS
    expect(deriveLifecycle(vehicle.auction_start, clock.effectiveNow)).toBe('upcoming')

    const bids = useBidsStore()
    const result = bids.placeBid(vehicle.id, vehicle.starting_bid + 1_000_000)

    expect(result.ok).toBe(false)
  })

  it('buyNow marks the listing purchased and records justBoughtId', () => {
    const vehicle = vehicles.find((candidate) => candidate.buy_now_price != null)
    expect(vehicle).toBeDefined()

    const bids = useBidsStore()
    const result = bids.buyNow(vehicle!.id)

    expect(result.ok).toBe(true)
    expect(bids.overrides[vehicle!.id]).toMatchObject({
      currentPrice: vehicle!.buy_now_price,
      purchased: true,
      hasUserBid: true,
      isUserHighBidder: true,
    })
    expect(bids.justBoughtId).toBe(vehicle!.id)
  })

  it('rejects buyNow on a listing with no Buy Now price', () => {
    const vehicle = vehicles.find((candidate) => candidate.buy_now_price == null)
    expect(vehicle).toBeDefined()

    const bids = useBidsStore()
    const result = bids.buyNow(vehicle!.id)

    expect(result.ok).toBe(false)
  })
})
