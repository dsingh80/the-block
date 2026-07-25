import { reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchListings, fetchListing, fetchFacets } from '@/services/api/listings'
import { useBidsStore } from './bids'
import type { Vehicle } from '@/types/vehicle'
import type { ApiListingSummary } from '@/services/api/types'

export type SortOption = 'ending' | 'price-low' | 'price-high' | 'year'
export type StatusFilter = 'all' | 'upcoming' | 'active' | 'ended'

export const PAGE_SIZE = 24
/** How long InventoryView waits after the last keystroke before refetching on a search change. */
export const SEARCH_DEBOUNCE_MS = 300
/** How long InventoryView waits after scrolling stops before syncing the URL's resume cursor to the current scroll position. */
export const SCROLL_SYNC_DEBOUNCE_MS = 500

function toVehicle(item: ApiListingSummary): Vehicle {
  const { viewer: _viewer, ...vehicle } = item
  return vehicle
}

/** One fetched batch's leading id and the `after` cursor that resumes forward starting near it -- see loadPreviousPage's own comment for why a backward page's own start cursor works here too. */
interface PageCheckpoint {
  id: string
  cursor: string | undefined
}

/**
 * A store, not local view state, specifically because InventoryView
 * unmounts when navigating to a listing's detail page — plain local refs
 * would silently reset on every round trip.
 *
 * Also doubles as the app's vehicle cache: `vehiclesById` accumulates every
 * vehicle ever fetched (any inventory page, any single-listing fetch) and is
 * never cleared on a filter change, unlike `ids` (the current page's ordered
 * display list, which IS reset). This is what lets useAugmentedListings()
 * (watchlist/compare, which need to render a listing regardless of whether
 * it's in the *current* filtered page) keep working without its own
 * separate fetch machinery (guidelines/06-backend-architecture.md's
 * client-integration phase).
 */
export const useInventoryFiltersStore = defineStore('inventoryFilters', () => {
  const search = ref('')
  const makeFilter = ref('all')
  const statusFilter = ref<StatusFilter>('all')
  const sortBy = ref<SortOption>('ending')

  const ids = ref<string[]>([])
  const vehiclesById = reactive<Record<string, Vehicle>>({})
  const hasNextPage = ref(false)
  const endCursor = ref<string | undefined>(undefined)
  const hasPreviousPage = ref(false)
  const startCursor = ref<string | undefined>(undefined)
  const loading = ref(false)
  const loadingPrevious = ref(false)
  const error = ref<string | null>(null)
  const makeOptions = ref<string[]>([])
  /** One entry per fetched batch, ordered to match `ids` -- what InventoryView's scroll-position URL sync walks to find the resume cursor for wherever the user currently is. */
  const checkpoints = ref<PageCheckpoint[]>([])

  /** Fire-and-forget on first use of this store -- the filter dropdown's options don't need to block anything else. */
  async function loadFacets() {
    try {
      const facets = await fetchFacets()
      makeOptions.value = facets.makes
    } catch {
      // The make dropdown just stays empty; every other filter still works.
    }
  }
  void loadFacets()

  /**
   * Populates vehiclesById from a fetched page, and reconciles this session's
   * bid relationship into the bids store from each item's `viewer` object --
   * without this, a returning user (page refresh, a fresh page load) would
   * lose their "Winning"/"Outbid" badge, since bids.overrides is otherwise
   * only ever populated by this session's own placeBid/buyNow calls.
   */
  function cacheAndReconcile(items: ApiListingSummary[]) {
    const bids = useBidsStore()
    for (const item of items) {
      vehiclesById[item.id] = toVehicle(item)
      const existing = bids.overrides[item.id]
      // listings.high_bidder_session_id/bid_count only reach Postgres once the
      // async stream tailer drains them (guidelines/06-backend-architecture.md,
      // "Draining vs. broadcasting") -- has_bid, by contrast, is answered from
      // Redis and is immediately consistent. Right after this session's own
      // accepted bid, a refetch landing inside that drain window would
      // describe an already-superseded snapshot (has_bid true, is_high_bidder
      // still the pre-drain value) and silently flip a correct "you're
      // winning" override back to "outbid". bid_count is a version number
      // already in the data -- never let an older snapshot regress an
      // override this session's own placeBid/buyNow (or a WS confirmation)
      // has already moved past.
      if (existing && item.bid_count < existing.bidCount) continue
      if (item.viewer.has_bid) {
        bids.overrides[item.id] = {
          currentPrice: item.current_bid ?? item.starting_bid,
          bidCount: item.bid_count,
          hasUserBid: item.viewer.has_bid,
          isUserHighBidder: item.viewer.is_high_bidder,
          isUserOutbid: item.viewer.is_outbid,
          purchased: item.purchased_at != null,
          purchasedAt: item.purchased_at != null ? Date.parse(item.purchased_at) : null,
        }
      }
    }
  }

  function baseParams() {
    return {
      status: statusFilter.value === 'all' ? undefined : statusFilter.value,
      make: makeFilter.value === 'all' ? undefined : makeFilter.value,
      q: search.value.trim() || undefined,
      sort: sortBy.value,
    }
  }

  function currentParams(after?: string) {
    return { ...baseParams(), first: PAGE_SIZE, after }
  }

  function previousParams(before?: string) {
    return { ...baseParams(), last: PAGE_SIZE, before }
  }

  /** Fetches page one for the current filters, replacing `ids` entirely. */
  async function reset(after?: string) {
    loading.value = true
    error.value = null
    try {
      const page = await fetchListings(currentParams(after))
      cacheAndReconcile(page.data)
      ids.value = page.data.map((item) => item.id)
      hasNextPage.value = page.page_info.has_next_page
      endCursor.value = page.page_info.end_cursor
      hasPreviousPage.value = page.page_info.has_previous_page
      startCursor.value = page.page_info.start_cursor
      checkpoints.value = page.data.length > 0 ? [{ id: page.data[0]!.id, cursor: after }] : []
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Something went wrong loading listings.'
    } finally {
      loading.value = false
    }
  }

  /** Appends the next page after the current end cursor -- a no-op while already loading or once exhausted. */
  async function loadNextPage() {
    if (loading.value || !hasNextPage.value) return
    loading.value = true
    error.value = null
    try {
      const cursorUsed = endCursor.value
      const page = await fetchListings(currentParams(cursorUsed))
      cacheAndReconcile(page.data)
      ids.value = [...ids.value, ...page.data.map((item) => item.id)]
      hasNextPage.value = page.page_info.has_next_page
      endCursor.value = page.page_info.end_cursor
      if (page.data.length > 0) {
        checkpoints.value = [...checkpoints.value, { id: page.data[0]!.id, cursor: cursorUsed }]
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Something went wrong loading more listings.'
    } finally {
      loading.value = false
    }
  }

  /**
   * Fetches the page immediately before the current start cursor and
   * prepends it -- a no-op while already loading, once exhausted, or if a
   * previous page is indicated but there's no cursor to actually resume from
   * (an empty page's has_previous_page can be true with start_cursor unset,
   * since the server only sets start_cursor `if len(rows) > 0`). Safe to call
   * repeatedly -- each call re-checks hasPreviousPage/startCursor against the
   * latest response, so InventoryView can keep offering to load earlier
   * pages until it's genuinely exhausted with no extra state on its side.
   */
  async function loadPreviousPage() {
    if (loadingPrevious.value || !hasPreviousPage.value || !startCursor.value) return
    loadingPrevious.value = true
    error.value = null
    try {
      const page = await fetchListings(previousParams(startCursor.value))
      cacheAndReconcile(page.data)
      ids.value = [...page.data.map((item) => item.id), ...ids.value]
      hasPreviousPage.value = page.page_info.has_previous_page
      startCursor.value = page.page_info.start_cursor
      if (page.data.length > 0) {
        checkpoints.value = [{ id: page.data[0]!.id, cursor: page.page_info.start_cursor }, ...checkpoints.value]
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Something went wrong loading earlier listings.'
    } finally {
      loadingPrevious.value = false
    }
  }

  /** Fetches a single listing into the cache if it isn't already known -- for a listing details page reached without ever paginating through it (a direct/bookmarked link). */
  async function ensureVehicleLoaded(id: string): Promise<void> {
    if (vehiclesById[id]) return
    try {
      const item = await fetchListing(id)
      cacheAndReconcile([item])
    } catch {
      // Leave it uncached; callers already treat "not in vehiclesById" as "not found."
    }
  }

  /** Also resets makeFilter/statusFilter so a stale filter can't hide the seller's own listings. */
  function searchSeller(dealershipName: string) {
    search.value = dealershipName
    makeFilter.value = 'all'
    statusFilter.value = 'all'
  }

  return {
    search,
    makeFilter,
    statusFilter,
    sortBy,
    ids,
    vehiclesById,
    hasNextPage,
    endCursor,
    hasPreviousPage,
    startCursor,
    loading,
    loadingPrevious,
    error,
    makeOptions,
    checkpoints,
    reset,
    loadNextPage,
    loadPreviousPage,
    ensureVehicleLoaded,
    searchSeller,
  }
})
