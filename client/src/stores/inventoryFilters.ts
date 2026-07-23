import { ref } from 'vue'
import { defineStore } from 'pinia'

export type SortOption = 'ending' | 'price-low' | 'price-high' | 'year'
export type StatusFilter = 'all' | 'upcoming' | 'active' | 'ended'

/**
 * A store, not local view state, specifically because InventoryView
 * unmounts when navigating to a listing's detail page — plain local refs
 * would silently reset on every round trip.
 */
export const useInventoryFiltersStore = defineStore('inventoryFilters', () => {
  const search = ref('')
  const makeFilter = ref('all')
  const statusFilter = ref<StatusFilter>('all')
  const sortBy = ref<SortOption>('ending')

  /** Also resets makeFilter/statusFilter so a stale filter can't hide the seller's own listings. */
  function searchSeller(dealershipName: string) {
    search.value = dealershipName
    makeFilter.value = 'all'
    statusFilter.value = 'all'
  }

  return { search, makeFilter, statusFilter, sortBy, searchSeller }
})
