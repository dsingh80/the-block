<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useInventoryFiltersStore } from '@/stores/inventoryFilters'
import type { AugmentedListing } from '@/types/listing'

const props = defineProps<{ listing: AugmentedListing }>()

const router = useRouter()
const filters = useInventoryFiltersStore()

/** Reuses InventoryView with a search prefill rather than a real seller-id filter or a separate seller page. */
function viewSellerListings() {
  filters.searchSeller(props.listing.vehicle.selling_dealership)
  router.push('/inventory')
}
</script>

<template>
  <div class="seller-card">
    <div>
      <div class="seller-card__name">{{ listing.vehicle.selling_dealership }}</div>
      <div class="seller-card__subtitle">Seller on OpenLane</div>
    </div>
    <button
      v-if="listing.sellerOtherListingsCount > 0"
      type="button"
      class="seller-card__link"
      @click="viewSellerListings"
    >
      {{ listing.sellerOtherListingsCount }} other listings →
    </button>
    <div v-else class="seller-card__solo">Only listing from this seller</div>
  </div>
</template>

<style scoped>
.seller-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.seller-card__name {
  font-size: 16px;
  font-weight: 800;
  color: var(--color-navy);
}

.seller-card__subtitle {
  font-size: 13px;
  color: var(--color-muted);
  margin-top: 2px;
}

.seller-card__link {
  border: none;
  background: none;
  padding: 0;
  font-size: 13px;
  font-weight: 700;
  color: var(--color-accent);
  cursor: pointer;
  white-space: nowrap;
}

.seller-card__solo {
  font-size: 13px;
  color: var(--color-muted);
  white-space: nowrap;
}
</style>
