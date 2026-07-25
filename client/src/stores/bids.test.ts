import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useBidsStore } from './bids'
import { useClockStore } from './clock'
import { useInventoryFiltersStore } from './inventoryFilters'
import { deriveLifecycle } from '@/utils/lifecycle'
import { getBidIncrement } from '@/utils/bidding'
import * as listingsApi from '@/services/api/listings'
import { ApiError } from '@/services/api/client'
import type { Vehicle } from '@/types/vehicle'

const HOUR_MS = 60 * 60 * 1000

vi.mock('@/services/api/listings')

function vehicleFixture(overrides: Partial<Vehicle> = {}): Vehicle {
  return {
    id: 'vehicle-1',
    vin: 'TESTVIN0000000001',
    year: 2022,
    make: 'Toyota',
    model: 'Camry',
    trim: 'SE',
    body_style: 'sedan',
    exterior_color: 'Black',
    interior_color: 'Black',
    engine: '2.5L I4',
    transmission: 'automatic',
    drivetrain: 'FWD',
    odometer_km: 40000,
    fuel_type: 'gasoline',
    condition_grade: 4.2,
    condition_report: 'Good condition.',
    damage_notes: [],
    title_status: 'clean',
    province: 'Ontario',
    city: 'Toronto',
    auction_start: '2026-01-01T12:00:00Z',
    auction_end: '2026-01-02T12:00:00Z',
    status: 'active',
    starting_bid: 10000,
    buy_now_price: null,
    images: [],
    selling_dealership: 'Test Motors',
    lot: 'A-0001',
    current_bid: 20000,
    bid_count: 5,
    purchased_at: null,
    ...overrides,
  }
}

/** bids.ts reads a vehicle's own price/lifecycle from the live inventory cache, not a static import -- this is what a real fetched page (or ensureVehicleLoaded) already populates it with. */
function cache(vehicle: Vehicle) {
  useInventoryFiltersStore().vehiclesById[vehicle.id] = vehicle
  return vehicle
}

describe('useBidsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(listingsApi.placeBid).mockReset()
    vi.mocked(listingsApi.buyNow).mockReset()
  })

  it('accepts a bid at the tiered minimum on an active listing, populating overrides from the server response', async () => {
    const vehicle = cache(vehicleFixture())
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS
    expect(deriveLifecycle(vehicle.auction_start, clock.effectiveNow)).toBe('active')

    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)
    vi.mocked(listingsApi.placeBid).mockResolvedValue({
      bid_id: 'bid-1',
      current_bid: minimum,
      bid_count: vehicle.bid_count + 1,
      accepted_at: '2026-01-01T00:00:00Z',
      viewer: { has_bid: true, is_high_bidder: true, is_outbid: false },
    })

    const bids = useBidsStore()
    const result = await bids.placeBid(vehicle.id, minimum)

    expect(result.ok).toBe(true)
    expect(listingsApi.placeBid).toHaveBeenCalledWith(vehicle.id, minimum)
    expect(bids.overrides[vehicle.id]).toMatchObject({
      currentPrice: minimum,
      hasUserBid: true,
      isUserHighBidder: true,
      isUserOutbid: false,
    })
  })

  it('rejects a bid on a vehicle not in the inventory cache without calling the API', async () => {
    const bids = useBidsStore()
    const result = await bids.placeBid('never-fetched', 999_999)

    expect(result).toEqual({ ok: false, error: 'Vehicle not found.' })
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('rejects a bid below the tiered minimum without calling the API', async () => {
    const vehicle = cache(vehicleFixture())
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS

    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)

    const bids = useBidsStore()
    const result = await bids.placeBid(vehicle.id, minimum - 1)

    expect(result.ok).toBe(false)
    expect(bids.overrides[vehicle.id]).toBeUndefined()
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('rejects a bid on a listing that is no longer active without calling the API', async () => {
    const vehicle = cache(vehicleFixture())
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + 48 * HOUR_MS
    expect(deriveLifecycle(vehicle.auction_start, clock.effectiveNow)).toBe('ended')

    const bids = useBidsStore()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const result = await bids.placeBid(vehicle.id, current + 1_000_000)

    expect(result.ok).toBe(false)
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('rejects a bid on a listing that has not started yet without calling the API', async () => {
    const vehicle = cache(vehicleFixture())
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() - HOUR_MS
    expect(deriveLifecycle(vehicle.auction_start, clock.effectiveNow)).toBe('upcoming')

    const bids = useBidsStore()
    const result = await bids.placeBid(vehicle.id, vehicle.starting_bid + 1_000_000)

    expect(result.ok).toBe(false)
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('surfaces the server-rejected error when the API rejects an otherwise valid-looking bid', async () => {
    const vehicle = cache(vehicleFixture())
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS

    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)
    vi.mocked(listingsApi.placeBid).mockRejectedValue(
      new ApiError(409, 'bid_too_low', 'Your bid is below the current minimum.', { minimum }),
    )

    const bids = useBidsStore()
    const result = await bids.placeBid(vehicle.id, minimum)

    expect(result).toEqual({ ok: false, error: 'Your bid is below the current minimum.' })
    expect(bids.overrides[vehicle.id]).toBeUndefined()
  })

  it('buyNow marks the listing purchased and records justBoughtId from the server response', async () => {
    const vehicle = cache(vehicleFixture({ id: 'vehicle-buy-now', buy_now_price: 45000 }))

    vi.mocked(listingsApi.buyNow).mockResolvedValue({
      bid_id: 'bid-2',
      current_bid: vehicle.buy_now_price!,
      bid_count: vehicle.bid_count + 1,
      accepted_at: '2026-01-01T00:00:00Z',
      viewer: { has_bid: true, is_high_bidder: true, is_outbid: false },
    })

    const bids = useBidsStore()
    const result = await bids.buyNow(vehicle.id)

    expect(result.ok).toBe(true)
    expect(listingsApi.buyNow).toHaveBeenCalledWith(vehicle.id)
    expect(bids.overrides[vehicle.id]).toMatchObject({
      currentPrice: vehicle.buy_now_price,
      purchased: true,
      hasUserBid: true,
      isUserHighBidder: true,
    })
    expect(bids.justBoughtId).toBe(vehicle.id)
  })

  it('rejects buyNow on a listing with no Buy Now price without calling the API', async () => {
    const vehicle = cache(vehicleFixture({ buy_now_price: null }))

    const bids = useBidsStore()
    const result = await bids.buyNow(vehicle.id)

    expect(result.ok).toBe(false)
    expect(listingsApi.buyNow).not.toHaveBeenCalled()
  })

  it('surfaces the server-rejected error when buyNow is rejected', async () => {
    const vehicle = cache(vehicleFixture({ id: 'vehicle-buy-now', buy_now_price: 45000 }))

    vi.mocked(listingsApi.buyNow).mockRejectedValue(
      new ApiError(409, 'buy_now_unavailable', 'Buy Now is no longer available for this listing.'),
    )

    const bids = useBidsStore()
    const result = await bids.buyNow(vehicle.id)

    expect(result).toEqual({ ok: false, error: 'Buy Now is no longer available for this listing.' })
  })
})
