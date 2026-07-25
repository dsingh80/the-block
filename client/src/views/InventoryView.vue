<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import FilterBar from '@/components/inventory/FilterBar.vue'
import VehicleCard from '@/components/inventory/VehicleCard.vue'
import VehicleCardSkeleton from '@/components/inventory/VehicleCardSkeleton.vue'
import { useAugmentedListings } from '@/composables/useListingPresentation'
import { useRealtimeSubscription } from '@/composables/useRealtimeSync'
import { prependPreservingScroll, scrollToResults, waitForScrollSettle } from '@/composables/useScrollAnchor'
import {
  useInventoryFiltersStore,
  PAGE_SIZE,
  SEARCH_DEBOUNCE_MS,
  SCROLL_SYNC_DEBOUNCE_MS,
  type SortOption,
  type StatusFilter,
} from '@/stores/inventoryFilters'
import type { AugmentedListing } from '@/types/listing'

const route = useRoute()
const router = useRouter()
const filters = useInventoryFiltersStore()
const {
  search,
  makeFilter,
  statusFilter,
  sortBy,
  ids,
  hasNextPage,
  endCursor,
  hasPreviousPage,
  startCursor,
  loading,
  loadingPrevious,
  error,
  checkpoints,
} = storeToRefs(filters)
const { list } = useAugmentedListings()

// Keeps every listing currently in the grid live -- a bid landing on any of
// them updates its price/badge in place, no refetch needed
// (guidelines/06-backend-architecture.md, "WebSocket protocol").
useRealtimeSubscription(() => ids.value)

/** ids is the server's ordering for the current page(s); list is the full known-vehicle cache -- this joins them back into an ordered, augmented array without assuming list's own order. */
const listings = computed<AugmentedListing[]>(() => {
  const byId = new Map(list.value.map((listing) => [listing.id, listing]))
  return ids.value.map((id) => byId.get(id)).filter((listing): listing is AugmentedListing => !!listing)
})

/**
 * Writes whichever cursor is passed as the URL's resume checkpoint, omitting
 * it entirely when none is passed (a fresh filter change -- start over from
 * page one, no checkpoint) -- guidelines/06-backend-architecture.md's
 * client-integration phase, the "resumable checkpoint" half of infinite
 * scroll. Called immediately with the new edge cursor from loadMore/
 * loadPrevious, and lazily from syncUrlFromScrollPosition while the user
 * browses already-loaded content with no new fetch happening.
 */
function syncUrl(cursor?: string) {
  router.replace({
    query: {
      ...(statusFilter.value !== 'all' ? { status: statusFilter.value } : {}),
      ...(makeFilter.value !== 'all' ? { make: makeFilter.value } : {}),
      ...(search.value.trim() ? { q: search.value.trim() } : {}),
      ...(sortBy.value !== 'ending' ? { sort: sortBy.value } : {}),
      ...(cursor ? { after: cursor } : {}),
    },
  })
}

// Guards the watcher below from firing during the URL -> store seed in
// onMounted -- that seed is followed by its own explicit reset() (with the
// URL's checkpoint cursor, if any), so the watcher's own filter-changed
// reset (which never knows about a resume cursor) must stay silent until
// after that initial fetch actually happens.
const initializing = ref(true)

const sentinelRef = ref<HTMLElement | null>(null)
const topSentinelRef = ref<HTMLElement | null>(null)
const realGridRef = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | undefined
let topObserver: IntersectionObserver | undefined

async function loadMore() {
  await filters.loadNextPage()
  syncUrl(endCursor.value)
}

async function loadPrevious() {
  const scroller = document.scrollingElement
  if (scroller) await prependPreservingScroll(scroller, () => filters.loadPreviousPage())
  else await filters.loadPreviousPage()
  syncUrl(startCursor.value)
}

let scrollSyncDebounce: ReturnType<typeof setTimeout> | undefined

/**
 * Best-effort URL sync while browsing already-loaded content, not just when
 * a new fetch happens -- finds whichever loaded checkpoint is nearest the
 * top of the viewport and writes its cursor, so a refresh resumes near
 * wherever the user currently is. Deliberately lazy/debounced (checkpoints
 * are one per ~PAGE_SIZE items, not pixel-precise) rather than an
 * IntersectionObserver -- there's no large/growing set of elements to watch
 * efficiently here, just a handful of positions to check once scrolling
 * stops.
 */
function syncUrlFromScrollPosition() {
  clearTimeout(scrollSyncDebounce)
  scrollSyncDebounce = setTimeout(() => {
    const headerBuffer = 80
    const topById = new Map<string, number>()
    document.querySelectorAll<HTMLElement>('[data-listing-id]').forEach((card) => {
      const id = card.dataset.listingId
      if (id) topById.set(id, card.getBoundingClientRect().top)
    })

    let current: string | undefined
    for (const checkpoint of checkpoints.value) {
      const top = topById.get(checkpoint.id)
      if (top !== undefined && top <= headerBuffer) current = checkpoint.cursor
    }
    if (current) syncUrl(current)
  }, SCROLL_SYNC_DEBOUNCE_MS)
}

onMounted(async () => {
  const q = route.query
  if (typeof q.status === 'string') statusFilter.value = q.status as StatusFilter
  if (typeof q.make === 'string') makeFilter.value = q.make
  if (typeof q.q === 'string') search.value = q.q
  if (typeof q.sort === 'string') sortBy.value = q.sort as SortOption

  await filters.reset(typeof q.after === 'string' ? q.after : undefined)
  initializing.value = false

  observer = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) void loadMore()
  })
  if (sentinelRef.value) observer.observe(sentinelRef.value)

  // Only meaningful once the real grid actually exists (it's inside the
  // v-else results branch), so wait a tick for reset()'s DOM update to land
  // before touching realGridRef -- unlike sentinelRef/topSentinelRef, which
  // are unconditional and already valid before reset() ever resolves.
  if (hasPreviousPage.value) {
    await nextTick()
    if (realGridRef.value) {
      scrollToResults(realGridRef.value)
      await waitForScrollSettle(document.scrollingElement ?? window)
    }
  }

  // rootMargin extends the detection zone upward so a backward fetch can
  // start before the sentinel is literally on-screen -- the placeholders are
  // meant to rarely actually be seen. Not observed until here, after the
  // settle-scroll above has genuinely finished: observing any earlier could
  // sample "intersecting" while that animation is still mid-flight (still at
  // or near scrollTop 0), firing loadPrevious before the user ever scrolled
  // and fighting the animation with prependPreservingScroll's own scrollTop
  // adjustment.
  topObserver = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) void loadPrevious()
    },
    { rootMargin: '600px 0px 0px 0px' },
  )
  if (topSentinelRef.value) topObserver.observe(topSentinelRef.value)

  window.addEventListener('scroll', syncUrlFromScrollPosition, { passive: true })
})

onBeforeUnmount(() => {
  observer?.disconnect()
  topObserver?.disconnect()
  window.removeEventListener('scroll', syncUrlFromScrollPosition)
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

    <div ref="topSentinelRef" class="inventory-view__sentinel" aria-hidden="true"></div>

    <p v-if="error" class="inventory-view__error" role="alert">{{ error }}</p>

    <div v-if="listings.length === 0 && loading" class="inventory-view__empty">
      <div class="inventory-view__empty-title">Loading auctions…</div>
    </div>

    <div v-else-if="listings.length === 0" class="inventory-view__empty">
      <div class="inventory-view__empty-title">No auctions match those filters</div>
      <div>Try clearing the search or selecting a different make.</div>
    </div>

    <template v-else>
      <div v-if="hasPreviousPage" class="inventory-view__grid inventory-view__grid--skeleton" aria-hidden="true">
        <VehicleCardSkeleton v-for="n in PAGE_SIZE" :key="n" />
      </div>
      <p v-if="loadingPrevious" class="inventory-view__loading-more">Loading earlier…</p>

      <div ref="realGridRef" class="inventory-view__grid">
        <VehicleCard v-for="listing in listings" :key="listing.id" :listing="listing" />
      </div>
      <p v-if="loading" class="inventory-view__loading-more">Loading more…</p>
    </template>

    <div ref="sentinelRef" class="inventory-view__sentinel" aria-hidden="true"></div>
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
  scroll-margin-top: var(--header-height);
}

.inventory-view__loading-more {
  text-align: center;
  color: var(--color-muted);
  font-size: 13px;
  font-weight: 600;
  margin: 20px 0 0;
}

.inventory-view__sentinel {
  height: 1px;
}

@media (max-width: 640px) {
  .inventory-view {
    padding: 20px 16px 100px;
  }
}
</style>
