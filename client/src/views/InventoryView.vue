<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import FilterBar from '@/components/inventory/FilterBar.vue'
import VehicleCard from '@/components/inventory/VehicleCard.vue'
import { useAugmentedListings } from '@/composables/useListingPresentation'
import { useInventoryFiltersStore } from '@/stores/inventoryFilters'
import type { AugmentedListing } from '@/types/listing'

const { list } = useAugmentedListings()
const filters = useInventoryFiltersStore()
const { search, makeFilter, statusFilter, sortBy } = storeToRefs(filters)

/** Active (soonest-ending first) -> upcoming (soonest-starting first) -> ended. The mock's 8 hardcoded listings never needed a cross-lifecycle tiebreak for "ending soonest." */
function sortKey(listing: AugmentedListing): number {
  if (listing.lifecycle === 'active') return listing.hoursRemaining
  if (listing.lifecycle === 'upcoming') return listing.hoursRemaining + 24
  return Infinity
}

const filteredListings = computed(() => {
  const query = search.value.trim().toLowerCase()

  const filtered = list.value.filter((listing) => {
    if (statusFilter.value !== 'all' && listing.lifecycle !== statusFilter.value) return false
    if (makeFilter.value !== 'all' && listing.vehicle.make !== makeFilter.value) return false
    if (query) {
      const haystack = [
        listing.vehicle.year,
        listing.vehicle.make,
        listing.vehicle.model,
        listing.vehicle.trim,
        listing.vehicle.vin,
        listing.vehicle.selling_dealership,
      ]
        .join(' ')
        .toLowerCase()
      if (!haystack.includes(query)) return false
    }
    return true
  })

  const sorted = [...filtered]
  if (sortBy.value === 'ending') sorted.sort((a, b) => sortKey(a) - sortKey(b))
  else if (sortBy.value === 'price-low') sorted.sort((a, b) => a.priceValue - b.priceValue)
  else if (sortBy.value === 'price-high') sorted.sort((a, b) => b.priceValue - a.priceValue)
  else if (sortBy.value === 'year') sorted.sort((a, b) => b.vehicle.year - a.vehicle.year)

  return sorted
})
</script>

<template>
  <main class="inventory-view">
    <div class="inventory-view__header">
      <h1>Inventory</h1>
      <p>{{ filteredListings.length }} auctions match your search</p>
    </div>

    <FilterBar />

    <div v-if="filteredListings.length === 0" class="inventory-view__empty">
      <div class="inventory-view__empty-title">No auctions match those filters</div>
      <div>Try clearing the search or selecting a different make.</div>
    </div>

    <div v-else class="inventory-view__grid">
      <VehicleCard v-for="listing in filteredListings" :key="listing.id" :listing="listing" />
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
