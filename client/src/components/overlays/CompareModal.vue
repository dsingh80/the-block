<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import { useRouter } from 'vue-router'
import GradePill from '@/components/shared/GradePill.vue'
import TitleStatusPill from '@/components/shared/TitleStatusPill.vue'
import WatchlistButton from '@/components/shared/WatchlistButton.vue'
import MileageIcon from '@/components/shared/MileageIcon.vue'
import LocationIcon from '@/components/shared/LocationIcon.vue'
import { useAugmentedListings } from '@/composables/useListingPresentation'
import { useCompareStore } from '@/stores/compare'
import { capitalize } from '@/utils/format'
import type { AugmentedListing } from '@/types/listing'

const compare = useCompareStore()
const router = useRouter()
const { list } = useAugmentedListings()
const dialogRef = ref<HTMLElement | null>(null)

const compareListings = computed<AugmentedListing[]>(() =>
  compare.ids
    .map((id) => list.value.find((listing) => listing.id === id))
    .filter((listing): listing is AugmentedListing => !!listing),
)

function close() {
  compare.closeModal()
}

function handleBackdropClick(event: MouseEvent) {
  if (event.target === event.currentTarget) close()
}

function viewAuction(id: string) {
  compare.closeModal()
  router.push(`/inventory/${id}`)
}

watch(
  () => compare.open,
  async (open) => {
    if (open) {
      await nextTick()
      dialogRef.value?.focus()
    }
  },
)
</script>

<template>
  <div v-if="compare.open" class="compare-modal-overlay" role="presentation" @click="handleBackdropClick">
    <div
      ref="dialogRef"
      class="compare-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="compare-modal-title"
      tabindex="-1"
      @keydown.esc="close"
    >
      <div class="compare-modal__header">
        <div id="compare-modal-title" class="compare-modal__title">Compare Listings</div>
        <button type="button" class="compare-modal__close" aria-label="Close compare" @click="close">
          ×
        </button>
      </div>

      <div class="compare-modal__grid">
        <div v-for="item in compareListings" :key="item.id" class="compare-modal__item">
          <img
            v-if="item.images[0]"
            :src="item.images[0]"
            :alt="`${item.vehicle.year} ${item.vehicle.make} ${item.vehicle.model}`"
            class="compare-modal__image"
            loading="lazy"
          />

          <div class="compare-modal__item-body">
            <div class="compare-modal__item-title">
              {{ item.vehicle.year }} {{ item.vehicle.make }} {{ item.vehicle.model }}
            </div>
            <div class="compare-modal__row compare-modal__row--muted">
              <span>{{ item.vehicle.trim }}</span>
              <span class="compare-modal__inline-icon"><MileageIcon />{{ item.mileageLabel }}</span>
            </div>

            <div class="compare-modal__section">
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Grade</span>
                <GradePill :grade="item.vehicle.condition_grade" />
              </div>
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Title</span>
                <TitleStatusPill :status="item.titleVariant" />
              </div>
            </div>

            <div class="compare-modal__section">
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">{{ item.priceLabelText }}</span>
                <span class="compare-modal__row-value compare-modal__row-value--price">{{
                  item.priceFormatted
                }}</span>
              </div>
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Bids</span>
                <span class="compare-modal__row-value">{{ item.bidCountLabel }}</span>
              </div>
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Time</span>
                <span
                  class="compare-modal__row-value"
                  :class="`compare-modal__time--${item.timeVariant}`"
                  >{{ item.timeLabel }}</span
                >
              </div>
            </div>

            <div class="compare-modal__section">
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Location</span>
                <span class="compare-modal__row-value compare-modal__inline-icon"
                  ><LocationIcon />{{ item.locationLabel }}</span
                >
              </div>
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Seller</span>
                <span class="compare-modal__row-value">{{ item.vehicle.selling_dealership }}</span>
              </div>
            </div>

            <div v-if="compare.detailsOpen" class="compare-modal__section">
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Engine</span>
                <span class="compare-modal__row-value">{{ item.vehicle.engine }}</span>
              </div>
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Transmission</span>
                <span class="compare-modal__row-value">{{ capitalize(item.vehicle.transmission) }}</span>
              </div>
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">Drivetrain</span>
                <span class="compare-modal__row-value">{{ item.vehicle.drivetrain }}</span>
              </div>
              <div class="compare-modal__row">
                <span class="compare-modal__row-label">VIN</span>
                <span class="compare-modal__row-value">{{ item.vehicle.vin }}</span>
              </div>
            </div>

            <button type="button" class="compare-modal__toggle-details" @click="compare.toggleDetails">
              {{ compare.detailsOpen ? 'Hide details −' : 'Show more details +' }}
            </button>

            <div class="compare-modal__actions">
              <WatchlistButton :listing="item" variant="labeled" block />
              <button
                type="button"
                class="compare-modal__cta"
                :class="`compare-modal__cta--${item.ctaVariant}`"
                @click="viewAuction(item.id)"
              >
                {{ item.ctaLabel }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.compare-modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--color-overlay-dark);
  z-index: var(--z-compare-modal);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.compare-modal {
  background: var(--color-surface);
  border-radius: 16px;
  max-width: min(780px, calc(100vw - 32px));
  width: 100%;
  max-height: 88vh;
  overflow: auto;
  padding: 24px;
  box-sizing: border-box;
  outline: none;
}

.compare-modal__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 18px;
}

.compare-modal__title {
  font-size: 18px;
  font-weight: 800;
  color: var(--color-navy);
}

.compare-modal__close {
  border: none;
  background: var(--color-page-bg);
  width: 32px;
  height: 32px;
  border-radius: 50%;
  font-size: 16px;
  cursor: pointer;
}

.compare-modal__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 18px;
}

.compare-modal__item {
  border: 1px solid var(--color-border);
  border-radius: 12px;
  overflow: hidden;
}

.compare-modal__image {
  width: 100%;
  height: 150px;
  display: block;
  object-fit: cover;
}

.compare-modal__item-body {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.compare-modal__item-title {
  font-size: 15px;
  font-weight: 800;
  color: var(--color-navy);
}

.compare-modal__row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  gap: 8px;
}

.compare-modal__row--muted {
  color: var(--color-muted);
  margin-bottom: 8px;
}

.compare-modal__inline-icon {
  display: flex;
  align-items: center;
  gap: 4px;
}

.compare-modal__row-label {
  color: var(--color-faint);
  flex-shrink: 0;
}

.compare-modal__row-value {
  font-weight: 600;
  text-align: right;
}

.compare-modal__row-value--price {
  font-weight: 800;
  color: var(--color-navy);
}

.compare-modal__time--urgent {
  color: var(--color-red-text);
  font-weight: 700;
}

.compare-modal__time--upcoming {
  color: var(--color-accent);
  font-weight: 700;
}

.compare-modal__section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 14px;
}

.compare-modal__toggle-details {
  border: none;
  background: none;
  padding: 0;
  text-align: left;
  font-size: 12px;
  font-weight: 700;
  color: var(--color-accent);
  cursor: pointer;
  margin-top: 10px;
}

.compare-modal__actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

.compare-modal__actions > :first-child {
  flex: 1;
}

.compare-modal__cta {
  flex: 1;
  border: 1px solid transparent;
  font-size: 12.5px;
  font-weight: 700;
  padding: 9px 6px;
  border-radius: 8px;
  cursor: pointer;
}

.compare-modal__cta--primary {
  background: var(--color-accent);
  color: var(--color-surface);
  border-color: var(--color-accent);
}

.compare-modal__cta--outline-navy {
  background: var(--color-surface);
  color: var(--color-navy);
  border-color: var(--color-navy);
}

.compare-modal__cta--outline-accent {
  background: var(--color-surface);
  color: var(--color-accent);
  border-color: var(--color-accent);
}
</style>
