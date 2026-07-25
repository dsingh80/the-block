<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import ImageCarousel from '@/components/shared/ImageCarousel.vue'
import GradePill from '@/components/shared/GradePill.vue'
import BidBadge from '@/components/shared/BidBadge.vue'
import WatchlistButton from '@/components/shared/WatchlistButton.vue'
import MileageIcon from '@/components/shared/MileageIcon.vue'
import LocationIcon from '@/components/shared/LocationIcon.vue'
import { useAugmentedListing } from '@/composables/useListingPresentation'
import { usePreviewStore } from '@/stores/preview'
import { useBidsStore } from '@/stores/bids'

const preview = usePreviewStore()
const bids = useBidsStore()
const router = useRouter()

const { listing } = useAugmentedListing(() => preview.id ?? undefined)
const dialogRef = ref<HTMLElement | null>(null)

function close() {
  preview.close()
}

function handleBackdropClick(event: MouseEvent) {
  if (event.target === event.currentTarget) close()
}

async function buyNow() {
  if (listing.value) await bids.buyNow(listing.value.id)
}

function goDetails() {
  if (!listing.value) return
  const id = listing.value.id
  preview.close()
  router.push(`/inventory/${id}`)
}

watch(
  () => preview.id,
  async (id) => {
    if (id) {
      await nextTick()
      dialogRef.value?.focus()
    }
  },
)
</script>

<template>
  <div v-if="listing" class="preview-modal-overlay" role="presentation" @click="handleBackdropClick">
    <div
      ref="dialogRef"
      class="preview-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="preview-modal-title"
      tabindex="-1"
      @keydown.esc="close"
    >
      <div class="preview-modal__media">
        <ImageCarousel :images="listing.images" :height-px="320" />
        <button type="button" class="preview-modal__close" aria-label="Close preview" @click="close">
          ×
        </button>
        <BidBadge v-if="listing.badgeVariant" :listing="listing" class="preview-modal__badge" />
      </div>

      <div class="preview-modal__body">
        <div class="preview-modal__heading">
          <div>
            <div id="preview-modal-title" class="preview-modal__title">
              {{ listing.vehicle.year }} {{ listing.vehicle.make }} {{ listing.vehicle.model }}
            </div>
            <div class="preview-modal__trim">{{ listing.vehicle.trim }}</div>
            <div class="preview-modal__meta">
              <span><MileageIcon />{{ listing.mileageLabel }}</span>
              <span><LocationIcon />{{ listing.locationLabel }}</span>
            </div>
          </div>
          <GradePill :grade="listing.vehicle.condition_grade" />
        </div>

        <div class="preview-modal__price-row">
          <div>
            <div class="preview-modal__price-label">{{ listing.priceLabelText }}</div>
            <div class="preview-modal__price">{{ listing.priceFormatted }}</div>
          </div>
          <div class="preview-modal__time-block">
            <div class="preview-modal__time" :class="`preview-modal__time--${listing.timeVariant}`">
              {{ listing.timeLabel }}
            </div>
            <div class="preview-modal__bid-count">{{ listing.bidCountLabel }}</div>
          </div>
        </div>

        <div v-if="listing.justBought" class="preview-modal__success">
          Purchase confirmed — you bought this vehicle for {{ listing.priceFormatted }}.
        </div>

        <button v-if="listing.showBuyNow" type="button" class="preview-modal__buy-now" @click="buyNow">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M13 2 3 14h7l-1 8 10-12h-7l1-8Z" />
          </svg>
          Buy Now for {{ listing.buyNowFormatted }}
        </button>

        <div class="preview-modal__footer">
          <WatchlistButton :listing="listing" variant="labeled" block />
          <button
            type="button"
            class="preview-modal__cta"
            :class="`preview-modal__cta--${listing.ctaVariant}`"
            @click="goDetails"
          >
            {{ listing.ctaLabel }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.preview-modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--color-overlay-dark);
  backdrop-filter: blur(2px);
  z-index: var(--z-preview-modal);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.preview-modal {
  background: var(--color-surface);
  border-radius: 16px;
  max-width: min(720px, calc(100vw - 32px));
  width: 100%;
  max-height: 90vh;
  overflow: auto;
  outline: none;
}

.preview-modal__media {
  position: relative;
}

.preview-modal__close {
  position: absolute;
  top: 12px;
  right: 12px;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: none;
  background: var(--color-overlay-light);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
}

.preview-modal__badge {
  position: absolute;
  top: 12px;
  left: 12px;
}

.preview-modal__body {
  padding: 22px 24px 24px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.preview-modal__heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  flex-wrap: wrap;
}

.preview-modal__title {
  font-size: 20px;
  font-weight: 800;
  color: var(--color-navy);
}

.preview-modal__trim {
  font-size: 13px;
  color: var(--color-muted);
  margin-top: 4px;
}

.preview-modal__meta {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 13px;
  color: var(--color-muted);
  margin-top: 4px;
  flex-wrap: wrap;
}

.preview-modal__meta span {
  display: flex;
  align-items: center;
  gap: 4px;
}

.preview-modal__price-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-top: 1px solid var(--color-divider);
  border-bottom: 1px solid var(--color-divider);
  padding: 12px 0;
}

.preview-modal__price-label {
  font-size: 11px;
  color: var(--color-faint);
  text-transform: uppercase;
  font-weight: 700;
}

.preview-modal__price {
  font-size: 22px;
  font-weight: 800;
  color: var(--color-navy);
}

.preview-modal__time-block {
  text-align: right;
}

.preview-modal__time {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-muted);
}

.preview-modal__time--urgent {
  color: var(--color-red-text);
}

.preview-modal__time--upcoming {
  color: var(--color-accent);
}

.preview-modal__bid-count {
  font-size: 12px;
  color: var(--color-muted);
}

.preview-modal__success {
  background: var(--color-green-bg);
  border: 1px solid var(--color-green-text);
  color: var(--color-green-text);
  font-weight: 700;
  font-size: 13.5px;
  padding: 12px 14px;
  border-radius: 10px;
}

.preview-modal__buy-now {
  width: 100%;
  border: none;
  background: var(--color-navy);
  color: var(--color-surface);
  font-weight: 800;
  font-size: 14.5px;
  padding: 13px;
  border-radius: 9px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.preview-modal__footer {
  display: flex;
  gap: 10px;
}

.preview-modal__footer > :first-child {
  flex: 1;
}

.preview-modal__cta {
  flex: 1.4;
  border: 1px solid transparent;
  font-weight: 800;
  font-size: 14px;
  padding: 12px;
  border-radius: 9px;
  cursor: pointer;
}

.preview-modal__cta--primary {
  background: var(--color-accent);
  color: var(--color-surface);
  border-color: var(--color-accent);
}

.preview-modal__cta--outline-navy {
  background: var(--color-surface);
  color: var(--color-navy);
  border-color: var(--color-navy);
}

.preview-modal__cta--outline-accent {
  background: var(--color-surface);
  color: var(--color-accent);
  border-color: var(--color-accent);
}
</style>
