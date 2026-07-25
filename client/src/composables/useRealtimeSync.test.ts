import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { effectScope, nextTick, ref } from 'vue'
import { handleRealtimeMessage, useRealtimeSubscription } from './useRealtimeSync'
import { realtimeConnection } from '@/services/api/ws'
import { useBidsStore } from '@/stores/bids'
import { useInventoryFiltersStore } from '@/stores/inventoryFilters'
import * as listingsApi from '@/services/api/listings'
import type { ApiListingSummary } from '@/services/api/types'
import type { WsBidAccepted, WsListingEnded } from '@/services/api/ws'

vi.mock('@/services/api/listings')
vi.mock('@/services/api/ws', () => ({
  realtimeConnection: {
    connect: vi.fn(),
    subscribe: vi.fn(),
    unsubscribe: vi.fn(),
    onMessage: vi.fn(() => vi.fn()),
  },
}))

function apiListing(overrides: Partial<ApiListingSummary> = {}): ApiListingSummary {
  return {
    id: 'listing-1',
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
    current_bid: null,
    bid_count: 0,
    purchased_at: null,
    viewer: { has_bid: false, is_high_bidder: false, is_outbid: false },
    ...overrides,
  }
}

function bidAccepted(overrides: Partial<WsBidAccepted> = {}): WsBidAccepted {
  return {
    type: 'bid_accepted',
    listing_id: 'listing-1',
    bid_id: 'bid-9',
    current_bid: 21500,
    bid_count: 4,
    high_bidder_is_you: false,
    accepted_at: '2026-01-01T13:00:00Z',
    ...overrides,
  }
}

describe('handleRealtimeMessage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(listingsApi.fetchListings).mockReset()
    vi.mocked(listingsApi.fetchFacets).mockReset().mockResolvedValue({ makes: [] })
  })

  async function seedVehicle(overrides: Partial<ApiListingSummary> = {}) {
    vi.mocked(listingsApi.fetchListings).mockResolvedValue({
      data: [apiListing(overrides)],
      page_info: { has_next_page: false, has_previous_page: false },
    })
    const filters = useInventoryFiltersStore()
    await filters.reset()
    return filters
  }

  it('updates the vehicle cache price/count for any subscriber, win or not', async () => {
    const filters = await seedVehicle()

    handleRealtimeMessage(bidAccepted({ high_bidder_is_you: false }))

    expect(filters.vehiclesById['listing-1']?.current_bid).toBe(21500)
    expect(filters.vehiclesById['listing-1']?.bid_count).toBe(4)
  })

  it('marks this session as high bidder when the accepted bid is its own', async () => {
    await seedVehicle()
    const bids = useBidsStore()

    handleRealtimeMessage(bidAccepted({ high_bidder_is_you: true }))

    expect(bids.overrides['listing-1']).toMatchObject({
      currentPrice: 21500,
      bidCount: 4,
      hasUserBid: true,
      isUserHighBidder: true,
      isUserOutbid: false,
    })
  })

  it('flips an existing bidder to outbid when someone else\'s bid lands', async () => {
    await seedVehicle({ current_bid: 20000, bid_count: 3, viewer: { has_bid: true, is_high_bidder: true, is_outbid: false } })
    const bids = useBidsStore()
    expect(bids.overrides['listing-1']?.isUserHighBidder).toBe(true)

    handleRealtimeMessage(bidAccepted({ high_bidder_is_you: false, current_bid: 21500, bid_count: 4 }))

    expect(bids.overrides['listing-1']).toMatchObject({
      currentPrice: 21500,
      bidCount: 4,
      isUserHighBidder: false,
      isUserOutbid: true,
    })
  })

  it('never fabricates a bid override for a session that never bid on this listing', async () => {
    await seedVehicle()
    const bids = useBidsStore()

    handleRealtimeMessage(bidAccepted({ high_bidder_is_you: false }))

    expect(bids.overrides['listing-1']).toBeUndefined()
  })

  it('preserves an existing purchase override when this session becomes the high bidder on a later bid', async () => {
    await seedVehicle()
    const bids = useBidsStore()
    bids.overrides['listing-1'] = {
      currentPrice: 20000,
      bidCount: 2,
      hasUserBid: true,
      isUserHighBidder: false,
      isUserOutbid: true,
      purchased: false,
      purchasedAt: null,
    }

    handleRealtimeMessage(bidAccepted({ high_bidder_is_you: true, current_bid: 22000, bid_count: 3 }))

    expect(bids.overrides['listing-1']).toMatchObject({
      currentPrice: 22000,
      isUserHighBidder: true,
      isUserOutbid: false,
      purchased: false,
      purchasedAt: null,
    })
  })

  it('marks the vehicle ended at the final price when a listing_ended event arrives', async () => {
    const filters = await seedVehicle()

    const listingEnded: WsListingEnded = {
      type: 'listing_ended',
      listing_id: 'listing-1',
      reason: 'time_expired',
      final_price: 25000,
    }
    handleRealtimeMessage(listingEnded)

    expect(filters.vehiclesById['listing-1']?.current_bid).toBe(25000)
    expect(filters.vehiclesById['listing-1']?.purchased_at).not.toBeNull()
  })

  it('ignores a message for a listing this client has never cached', () => {
    expect(() => handleRealtimeMessage(bidAccepted({ listing_id: 'never-seen' }))).not.toThrow()
  })
})

describe('useRealtimeSubscription', () => {
  beforeEach(() => {
    vi.mocked(realtimeConnection.subscribe).mockClear()
    vi.mocked(realtimeConnection.unsubscribe).mockClear()
  })

  it('subscribes to added ids and unsubscribes removed ones as the id list changes, then unsubscribes everything on scope disposal', async () => {
    const ids = ref<string[]>(['a', 'b'])
    const scope = effectScope()
    scope.run(() => {
      useRealtimeSubscription(() => ids.value)
    })

    expect(realtimeConnection.subscribe).toHaveBeenCalledWith(['a', 'b'])

    ids.value = ['b', 'c']
    await nextTick()

    expect(realtimeConnection.subscribe).toHaveBeenCalledWith(['c'])
    expect(realtimeConnection.unsubscribe).toHaveBeenCalledWith(['a'])

    scope.stop()

    expect(realtimeConnection.unsubscribe).toHaveBeenCalledWith(['b', 'c'])
  })
})
