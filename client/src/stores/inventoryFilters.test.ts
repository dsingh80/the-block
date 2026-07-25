import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useInventoryFiltersStore } from './inventoryFilters'
import { useBidsStore } from './bids'
import * as listingsApi from '@/services/api/listings'
import type { ApiListingSummary } from '@/services/api/types'

vi.mock('@/services/api/listings')

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

describe('useInventoryFiltersStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(listingsApi.fetchListings).mockReset()
    vi.mocked(listingsApi.fetchListing).mockReset()
    vi.mocked(listingsApi.fetchFacets).mockReset().mockResolvedValue({ makes: ['Mazda', 'Toyota'] })
  })

  it('loads facets on first use', async () => {
    const filters = useInventoryFiltersStore()
    await Promise.resolve() // let the store's own fire-and-forget loadFacets() settle
    expect(filters.makeOptions).toEqual(['Mazda', 'Toyota'])
  })

  describe('reset', () => {
    it('replaces ids and populates the vehicle cache from a fresh page', async () => {
      const a = apiListing({ id: 'a' })
      const b = apiListing({ id: 'b' })
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [a, b],
        page_info: { has_next_page: true, has_previous_page: false, end_cursor: 'cursor-b' },
      })

      const filters = useInventoryFiltersStore()
      await filters.reset()

      expect(filters.ids).toEqual(['a', 'b'])
      expect(filters.vehiclesById['a']?.id).toBe('a')
      expect(filters.vehiclesById['b']?.id).toBe('b')
      expect(filters.hasNextPage).toBe(true)
      expect(filters.loading).toBe(false)
      expect(filters.error).toBeNull()
    })

    it('passes the current filters and an optional resume cursor through to fetchListings', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [],
        page_info: { has_next_page: false, has_previous_page: false },
      })

      const filters = useInventoryFiltersStore()
      filters.statusFilter = 'active'
      filters.makeFilter = 'Mazda'
      filters.search = '  turbo  '
      filters.sortBy = 'price-low'

      await filters.reset('resume-cursor')

      expect(listingsApi.fetchListings).toHaveBeenCalledWith({
        status: 'active',
        make: 'Mazda',
        q: 'turbo',
        sort: 'price-low',
        first: 24,
        after: 'resume-cursor',
      })
    })

    it('"all" status/make map to no filter at all', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [],
        page_info: { has_next_page: false, has_previous_page: false },
      })

      const filters = useInventoryFiltersStore()
      await filters.reset()

      expect(listingsApi.fetchListings).toHaveBeenCalledWith(
        expect.objectContaining({ status: undefined, make: undefined, q: undefined }),
      )
    })

    it('sets a readable error and clears loading when the fetch rejects', async () => {
      vi.mocked(listingsApi.fetchListings).mockRejectedValue(new Error('network down'))

      const filters = useInventoryFiltersStore()
      await filters.reset()

      expect(filters.error).toBe('network down')
      expect(filters.loading).toBe(false)
      expect(filters.ids).toEqual([])
    })

    it('reconciles bids.overrides from each item that has_bid, so a refreshed page keeps the user\'s badge', async () => {
      const mine = apiListing({
        id: 'mine',
        current_bid: 21500,
        bid_count: 3,
        viewer: { has_bid: true, is_high_bidder: true, is_outbid: false },
      })
      const notMine = apiListing({ id: 'not-mine' })
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [mine, notMine],
        page_info: { has_next_page: false, has_previous_page: false },
      })

      const filters = useInventoryFiltersStore()
      await filters.reset()

      const bids = useBidsStore()
      expect(bids.overrides['mine']).toMatchObject({
        currentPrice: 21500,
        bidCount: 3,
        hasUserBid: true,
        isUserHighBidder: true,
      })
      expect(bids.overrides['not-mine']).toBeUndefined()
    })
  })

  describe('loadNextPage', () => {
    it('appends to ids and merges into the vehicle cache, using the current end cursor', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: true, has_previous_page: false, end_cursor: 'cursor-a' },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'b' })],
        page_info: { has_next_page: false, has_previous_page: true },
      })
      await filters.loadNextPage()

      expect(filters.ids).toEqual(['a', 'b'])
      expect(filters.vehiclesById['b']?.id).toBe('b')
      expect(filters.hasNextPage).toBe(false)
      expect(listingsApi.fetchListings).toHaveBeenLastCalledWith(expect.objectContaining({ after: 'cursor-a' }))
    })

    it('is a no-op once hasNextPage is false', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: false, has_previous_page: false },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockClear()
      await filters.loadNextPage()

      expect(listingsApi.fetchListings).not.toHaveBeenCalled()
      expect(filters.ids).toEqual(['a'])
    })
  })

  describe('ensureVehicleLoaded', () => {
    it('fetches and caches a vehicle not already known', async () => {
      vi.mocked(listingsApi.fetchListing).mockResolvedValue(apiListing({ id: 'direct-link' }))

      const filters = useInventoryFiltersStore()
      await filters.ensureVehicleLoaded('direct-link')

      expect(filters.vehiclesById['direct-link']?.id).toBe('direct-link')
      expect(listingsApi.fetchListing).toHaveBeenCalledWith('direct-link')
    })

    it('does not refetch a vehicle that is already cached', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [apiListing({ id: 'already-here' })],
        page_info: { has_next_page: false, has_previous_page: false },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      await filters.ensureVehicleLoaded('already-here')

      expect(listingsApi.fetchListing).not.toHaveBeenCalled()
    })
  })

  describe('searchSeller', () => {
    it('sets the search term and resets make/status filters', () => {
      const filters = useInventoryFiltersStore()
      filters.makeFilter = 'Mazda'
      filters.statusFilter = 'ended'

      filters.searchSeller('Highway 7 Auto Sales')

      expect(filters.search).toBe('Highway 7 Auto Sales')
      expect(filters.makeFilter).toBe('all')
      expect(filters.statusFilter).toBe('all')
    })
  })
})
