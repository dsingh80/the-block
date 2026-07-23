<script setup lang="ts">
import { useRouter } from 'vue-router'
import ImageCarousel from '@/components/shared/ImageCarousel.vue'
import BidBadge from '@/components/shared/BidBadge.vue'
import MileageIcon from '@/components/shared/MileageIcon.vue'
import LocationIcon from '@/components/shared/LocationIcon.vue'
import ConditionCard from '@/components/listing/ConditionCard.vue'
import VehicleDataCard from '@/components/listing/VehicleDataCard.vue'
import SellerCard from '@/components/listing/SellerCard.vue'
import BidPanel from '@/components/listing/BidPanel.vue'
import { useAugmentedListing } from '@/composables/useListingPresentation'

const props = defineProps<{ id: string }>()
const router = useRouter()

/** Reachable via the Preview Modal's "View Auction" action and by direct URL — an id that doesn't resolve renders a typed not-found state below, distinct from the router's catch-all for garbage paths. */
const { listing } = useAugmentedListing(() => props.id)

function goInventory() {
  router.push('/inventory')
}
</script>

<template>
  <main class="listing-details-view">
    <button type="button" class="listing-details-view__back" @click="goInventory">
      ← Back to Inventory
    </button>

    <div v-if="!listing" class="listing-details-view__not-found">
      <div class="listing-details-view__not-found-title">Vehicle not found</div>
      <p>We couldn't find an auction with that id.</p>
      <button type="button" class="listing-details-view__back-link" @click="goInventory">
        Back to Inventory
      </button>
    </div>

    <div v-else class="listing-details-view__layout">
      <div class="listing-details-view__main">
        <ImageCarousel :images="listing.images" :height-px="420" />

        <div>
          <BidBadge v-if="listing.badgeVariant" :listing="listing" class="listing-details-view__badge" />
          <h1>{{ listing.vehicle.year }} {{ listing.vehicle.make }} {{ listing.vehicle.model }}</h1>
          <div class="listing-details-view__trim">{{ listing.vehicle.trim }}</div>
          <div class="listing-details-view__meta">
            <span><MileageIcon />{{ listing.mileageLabel }}</span>
            <span><LocationIcon />{{ listing.locationLabel }}</span>
          </div>
        </div>

        <ConditionCard :listing="listing" />
        <VehicleDataCard :vehicle="listing.vehicle" />
        <SellerCard :listing="listing" />
      </div>

      <div class="listing-details-view__side">
        <BidPanel :listing="listing" />
      </div>
    </div>
  </main>
</template>

<style scoped>
.listing-details-view {
  padding: 24px 56px 80px 28px;
  max-width: 1280px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
}

.listing-details-view__back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: none;
  padding: 0;
  color: var(--color-accent);
  font-weight: 600;
  font-size: 14px;
  cursor: pointer;
  margin-bottom: 16px;
}

.listing-details-view__not-found {
  background: var(--color-surface);
  border: 1px dashed var(--color-inactive);
  border-radius: 12px;
  padding: 60px 20px;
  text-align: center;
  color: var(--color-muted);
}

.listing-details-view__not-found-title {
  font-size: 18px;
  font-weight: 800;
  color: var(--color-navy);
  margin-bottom: 8px;
}

.listing-details-view__back-link {
  margin-top: 12px;
  border: none;
  background: none;
  color: var(--color-accent);
  font-weight: 700;
  cursor: pointer;
}

.listing-details-view__layout {
  display: flex;
  flex-wrap: wrap;
  gap: 28px;
  align-items: flex-start;
}

.listing-details-view__main {
  flex: 2;
  min-width: 380px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.listing-details-view__badge {
  display: inline-block;
  margin-bottom: 8px;
}

.listing-details-view__main h1 {
  margin: 0 0 4px;
  font-size: 26px;
  font-weight: 800;
  color: var(--color-navy);
}

.listing-details-view__trim {
  font-size: 14px;
  color: var(--color-muted);
  margin-bottom: 2px;
}

.listing-details-view__meta {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 14px;
  color: var(--color-muted);
  flex-wrap: wrap;
}

.listing-details-view__meta span {
  display: flex;
  align-items: center;
  gap: 5px;
}

.listing-details-view__side {
  flex: 1;
  min-width: 300px;
  position: sticky;
  top: 76px;
  z-index: var(--z-sticky-column);
}

@media (max-width: 768px) {
  .listing-details-view__side {
    order: -1;
    position: static;
  }
}

@media (max-width: 640px) {
  .listing-details-view {
    padding: 20px 16px 60px;
  }
}
</style>
