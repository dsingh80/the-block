import { reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchListings, fetchListing, fetchFacets } from '@/services/api/listings'
import { useBidsStore } from './bids'
import type { Vehicle } from '@/types/vehicle'
import type { ApiListingSummary } from '@/services/api/types'

export type SortOption = 'ending' | 'price-low' | 'price-high' | 'year'
export type StatusFilter = 'all' | 'upcoming' | 'active' | 'ended'

const PAGE_SIZE = 24
/** How long InventoryView waits after the last keystroke before refetching on a search change. */
export const SEARCH_DEBOUNCE_MS = 300

function toVehicle(item: ApiListingSummary): Vehicle {
  const { viewer: _viewer, ...vehicle } = item
  return vehicle
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
  const loading = ref(false)
  const error = ref<string | null>(null)
  const makeOptions = ref<string[]>([])

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

  function currentParams(after?: string) {
    return {
      status: statusFilter.value === 'all' ? undefined : statusFilter.value,
      make: makeFilter.value === 'all' ? undefined : makeFilter.value,
      q: search.value.trim() || undefined,
      sort: sortBy.value,
      first: PAGE_SIZE,
      after,
    }
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
      const page = await fetchListings(currentParams(endCursor.value))
      cacheAndReconcile(page.data)
      ids.value = [...ids.value, ...page.data.map((item) => item.id)]
      hasNextPage.value = page.page_info.has_next_page
      endCursor.value = page.page_info.end_cursor
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Something went wrong loading more listings.'
    } finally {
      loading.value = false
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
    loading,
    error,
    makeOptions,
    reset,
    loadNextPage,
    ensureVehicleLoaded,
    searchSeller,
  }
})
