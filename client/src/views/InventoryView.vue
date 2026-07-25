<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import FilterBar from '@/components/inventory/FilterBar.vue'
import VehicleCard from '@/components/inventory/VehicleCard.vue'
import { useAugmentedListings } from '@/composables/useListingPresentation'
import {
  useInventoryFiltersStore,
  SEARCH_DEBOUNCE_MS,
  type SortOption,
  type StatusFilter,
} from '@/stores/inventoryFilters'
import type { AugmentedListing } from '@/types/listing'

const route = useRoute()
const router = useRouter()
const filters = useInventoryFiltersStore()
const { search, makeFilter, statusFilter, sortBy, ids, loading, error, hasNextPage } = storeToRefs(filters)
const { list } = useAugmentedListings()

/** ids is the server's ordering for the current page(s); list is the full known-vehicle cache -- this joins them back into an ordered, augmented array without assuming list's own order. */
const listings = computed<AugmentedListing[]>(() => {
  const byId = new Map(list.value.map((listing) => [listing.id, listing]))
  return ids.value.map((id) => byId.get(id)).filter((listing): listing is AugmentedListing => !!listing)
})

function syncUrl() {
  router.replace({
    query: {
      ...(statusFilter.value !== 'all' ? { status: statusFilter.value } : {}),
      ...(makeFilter.value !== 'all' ? { make: makeFilter.value } : {}),
      ...(search.value.trim() ? { q: search.value.trim() } : {}),
      ...(sortBy.value !== 'ending' ? { sort: sortBy.value } : {}),
    },
  })
}

// Guards the watcher below from firing during the URL -> store seed in
// onMounted -- that seed is followed by its own explicit reset() (with the
// URL's checkpoint cursor, if any), so the watcher's own filter-changed
// reset (which never knows about a resume cursor) must stay silent until
// after that initial fetch actually happens.
const initializing = ref(true)

onMounted(async () => {
  const q = route.query
  if (typeof q.status === 'string') statusFilter.value = q.status as StatusFilter
  if (typeof q.make === 'string') makeFilter.value = q.make
  if (typeof q.q === 'string') search.value = q.q
  if (typeof q.sort === 'string') sortBy.value = q.sort as SortOption

  await filters.reset(typeof q.after === 'string' ? q.after : undefined)
  initializing.value = false
})

let searchDebounce: ReturnType<typeof setTimeout> | undefined

watch(search, () => {
  if (initializing.value) return
  clearTimeout(searchDebounce)
  searchDebounce = setTimeout(() => {
    void filters.reset()
    syncUrl()
  }, SEARCH_DEBOUNCE_MS)
})

watch([makeFilter, statusFilter, sortBy], () => {
  if (initializing.value) return
  void filters.reset()
  syncUrl()
})
</script>

<template>
  <main class="inventory-view">
    <div class="inventory-view__header">
      <h1>Inventory</h1>
      <p>{{ listings.length }} auction{{ listings.length === 1 ? '' : 's' }} loaded{{ hasNextPage ? ', more available' : '' }}</p>
    </div>

    <FilterBar />

    <p v-if="error" class="inventory-view__error" role="alert">{{ error }}</p>

    <div v-if="listings.length === 0 && loading" class="inventory-view__empty">
      <div class="inventory-view__empty-title">Loading auctions…</div>
    </div>

    <div v-else-if="listings.length === 0" class="inventory-view__empty">
      <div class="inventory-view__empty-title">No auctions match those filters</div>
      <div>Try clearing the search or selecting a different make.</div>
    </div>

    <div v-else class="inventory-view__grid">
      <VehicleCard v-for="listing in listings" :key="listing.id" :listing="listing" />
    </div>
  </main>
</template>

<style scoped>
.inventory-view {
  padding: 28px 56px 130px 28px;
  max-width: 1400px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
}

.inventory-view__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 14px;
  margin-bottom: 20px;
}

.inventory-view__header h1 {
  margin: 0 0 4px;
  font-size: 26px;
  font-weight: 800;
  color: var(--color-navy);
}

.inventory-view__header p {
  margin: 0;
  font-size: 14px;
  color: var(--color-muted);
}

.inventory-view__error {
  background: var(--color-red-bg, #fdecec);
  color: var(--color-red-text);
  border-radius: 10px;
  padding: 12px 16px;
  margin: 0 0 16px;
  font-size: 14px;
  font-weight: 600;
}

.inventory-view__empty {
  background: var(--color-surface);
  border: 1px dashed var(--color-inactive);
  border-radius: 12px;
  padding: 60px 20px;
  text-align: center;
  color: var(--color-muted);
  margin-top: 22px;
}

.inventory-view__empty-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-navy);
  margin-bottom: 6px;
}

.inventory-view__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(272px, 1fr));
  gap: 20px;
  margin-top: 22px;
}

@media (max-width: 640px) {
  .inventory-view {
    padding: 20px 16px 100px;
  }
}
</style>
