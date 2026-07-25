import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useInventoryFiltersStore, PAGE_SIZE } from './inventoryFilters'
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
        page_info: {
          has_next_page: true,
          end_cursor: 'cursor-b',
          has_previous_page: true,
          start_cursor: 'cursor-a',
        },
      })

      const filters = useInventoryFiltersStore()
      await filters.reset()

      expect(filters.ids).toEqual(['a', 'b'])
      expect(filters.vehiclesById['a']?.id).toBe('a')
      expect(filters.vehiclesById['b']?.id).toBe('b')
      expect(filters.hasNextPage).toBe(true)
      expect(filters.hasPreviousPage).toBe(true)
      expect(filters.startCursor).toBe('cursor-a')
      expect(filters.checkpoints).toEqual([{ id: 'a', cursor: undefined }])
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

    it('never lets a refetch regress an override to an older bid_count -- the async stream-tailer drain lagging behind this session\'s own just-accepted bid must not flip a winning override back to outbid', async () => {
      const filters = useInventoryFiltersStore()
      const bids = useBidsStore()
      // Simulates what bids.placeBid already wrote from the synchronous
      // accept response -- ahead of anything Postgres has drained yet.
      bids.overrides['mine'] = {
        currentPrice: 22000,
        bidCount: 4,
        hasUserBid: true,
        isUserHighBidder: true,
        isUserOutbid: false,
        purchased: false,
        purchasedAt: null,
      }

      const stale = apiListing({
        id: 'mine',
        current_bid: 20000,
        bid_count: 3,
        viewer: { has_bid: true, is_high_bidder: false, is_outbid: true },
      })
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [stale],
        page_info: { has_next_page: false, has_previous_page: false },
      })

      await filters.reset()

      expect(bids.overrides['mine']).toMatchObject({
        currentPrice: 22000,
        bidCount: 4,
        isUserHighBidder: true,
        isUserOutbid: false,
      })
    })

    it('does adopt a refetch that reports a genuinely newer bid_count, e.g. someone else outbidding this session since', async () => {
      const filters = useInventoryFiltersStore()
      const bids = useBidsStore()
      bids.overrides['mine'] = {
        currentPrice: 22000,
        bidCount: 4,
        hasUserBid: true,
        isUserHighBidder: true,
        isUserOutbid: false,
        purchased: false,
        purchasedAt: null,
      }

      const newer = apiListing({
        id: 'mine',
        current_bid: 23000,
        bid_count: 5,
        viewer: { has_bid: true, is_high_bidder: false, is_outbid: true },
      })
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [newer],
        page_info: { has_next_page: false, has_previous_page: false },
      })

      await filters.reset()

      expect(bids.overrides['mine']).toMatchObject({
        currentPrice: 23000,
        bidCount: 5,
        isUserHighBidder: false,
        isUserOutbid: true,
      })
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
      expect(filters.checkpoints).toEqual([
        { id: 'a', cursor: undefined },
        { id: 'b', cursor: 'cursor-a' },
      ])
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

  describe('loadPreviousPage', () => {
    it('prepends to ids and merges into the vehicle cache, using the current start cursor', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'b' })],
        page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-b' },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: true, has_previous_page: false },
      })
      await filters.loadPreviousPage()

      expect(filters.ids).toEqual(['a', 'b'])
      expect(filters.vehiclesById['a']?.id).toBe('a')
      expect(listingsApi.fetchListings).toHaveBeenLastCalledWith(
        expect.objectContaining({ before: 'cursor-b', last: PAGE_SIZE }),
      )
    })

    it('sends only last/before, not first/after, when paginating backward', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-a' },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'z' })],
        page_info: { has_next_page: true, has_previous_page: false },
      })
      await filters.loadPreviousPage()

      expect(listingsApi.fetchListings).toHaveBeenLastCalledWith({
        status: undefined,
        make: undefined,
        q: undefined,
        sort: 'ending',
        last: PAGE_SIZE,
        before: 'cursor-a',
      })
    })

    it('is a no-op once hasPreviousPage is false', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: false, has_previous_page: false },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockClear()
      await filters.loadPreviousPage()

      expect(listingsApi.fetchListings).not.toHaveBeenCalled()
      expect(filters.ids).toEqual(['a'])
    })

    it('is a no-op when hasPreviousPage is true but there is no start cursor', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValue({
        data: [],
        page_info: { has_next_page: false, has_previous_page: true },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset('stale-cursor')

      vi.mocked(listingsApi.fetchListings).mockClear()
      await filters.loadPreviousPage()

      expect(listingsApi.fetchListings).not.toHaveBeenCalled()
    })

    it('does not touch hasNextPage or endCursor when a backward page resolves', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'b' })],
        page_info: {
          has_next_page: true,
          end_cursor: 'tail-cursor',
          has_previous_page: true,
          start_cursor: 'head-cursor',
        },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: false, has_previous_page: false },
      })
      await filters.loadPreviousPage()

      expect(filters.hasNextPage).toBe(true)
      expect(filters.endCursor).toBe('tail-cursor')
    })

    it('reconciles bids.overrides from a prepended page, same as a forward page', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'b' })],
        page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-b' },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      const mine = apiListing({
        id: 'mine',
        current_bid: 21500,
        bid_count: 3,
        viewer: { has_bid: true, is_high_bidder: true, is_outbid: false },
      })
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [mine],
        page_info: { has_next_page: true, has_previous_page: false },
      })
      await filters.loadPreviousPage()

      const bids = useBidsStore()
      expect(bids.overrides['mine']).toMatchObject({
        currentPrice: 21500,
        bidCount: 3,
        hasUserBid: true,
        isUserHighBidder: true,
      })
    })

    it('sets a readable error and clears loadingPrevious without touching loading, when the fetch rejects', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-a' },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockRejectedValueOnce(new Error('network down'))
      await filters.loadPreviousPage()

      expect(filters.error).toBe('network down')
      expect(filters.loadingPrevious).toBe(false)
      expect(filters.loading).toBe(false)
      expect(filters.ids).toEqual(['a'])
    })

    it('prepends a new checkpoint on success', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'b' })],
        page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-b' },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: true, has_previous_page: false, start_cursor: 'cursor-a' },
      })
      await filters.loadPreviousPage()

      expect(filters.checkpoints).toEqual([
        { id: 'a', cursor: 'cursor-a' },
        { id: 'b', cursor: undefined },
      ])
    })

    it('keeps working across repeated calls until truly exhausted -- recheck-until-no-more-cursors', async () => {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'c' })],
        page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-c' },
      })
      const filters = useInventoryFiltersStore()
      await filters.reset()

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'b' })],
        page_info: { has_next_page: true, has_previous_page: true, start_cursor: 'cursor-b' },
      })
      await filters.loadPreviousPage()
      expect(filters.hasPreviousPage).toBe(true)

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: true, has_previous_page: false },
      })
      await filters.loadPreviousPage()

      expect(filters.ids).toEqual(['a', 'b', 'c'])
      expect(filters.hasPreviousPage).toBe(false)
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
