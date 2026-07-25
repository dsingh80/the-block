<script setup lang="ts">
import { ref } from 'vue'
import GradePill from '@/components/shared/GradePill.vue'
import BidBadge from '@/components/shared/BidBadge.vue'
import WatchlistButton from '@/components/shared/WatchlistButton.vue'
import MileageIcon from '@/components/shared/MileageIcon.vue'
import LocationIcon from '@/components/shared/LocationIcon.vue'
import { useCompareStore } from '@/stores/compare'
import { usePreviewStore } from '@/stores/preview'
import type { AugmentedListing } from '@/types/listing'

const props = defineProps<{ listing: AugmentedListing }>()

const compare = useCompareStore()
const preview = usePreviewStore()
const imageFailed = ref(false)

function openPreview() {
  preview.open(props.listing.id)
}

function toggleCompare() {
  compare.toggle(props.listing.id)
}
</script>

<template>
  <div class="vehicle-card" :data-listing-id="listing.id">
    <div class="vehicle-card__media" @click="openPreview">
      <img
        v-if="listing.images[0] && !imageFailed"
        :src="listing.images[0]"
        :alt="`${listing.vehicle.year} ${listing.vehicle.make} ${listing.vehicle.model} photo`"
        class="vehicle-card__image"
        loading="lazy"
        @error="imageFailed = true"
      />
      <div v-else class="vehicle-card__image-fallback">Photo unavailable</div>

      <BidBadge v-if="listing.badgeVariant" :listing="listing" class="vehicle-card__badge" />
      <WatchlistButton :listing="listing" class="vehicle-card__watch" @click.stop />
    </div>

    <div class="vehicle-card__body">
      <div class="vehicle-card__heading">
        <div class="vehicle-card__title-block">
          <div class="vehicle-card__title">
            {{ listing.vehicle.year }} {{ listing.vehicle.make }} {{ listing.vehicle.model }}
          </div>
          <div class="vehicle-card__trim">{{ listing.vehicle.trim }}</div>
        </div>
        <GradePill :grade="listing.vehicle.condition_grade" />
      </div>

      <div class="vehicle-card__meta">
        <span class="vehicle-card__meta-item"><MileageIcon />{{ listing.mileageLabel }}</span>
        <span class="vehicle-card__meta-item"><LocationIcon />{{ listing.locationLabel }}</span>
      </div>

      <div class="vehicle-card__price-row">
        <div>
          <div class="vehicle-card__price-label">{{ listing.priceLabelText }}</div>
          <div class="vehicle-card__price">{{ listing.priceFormatted }}</div>
        </div>
        <div class="vehicle-card__time-block">
          <div class="vehicle-card__time" :class="`vehicle-card__time--${listing.timeVariant}`">
            {{ listing.timeLabel }}
          </div>
          <div class="vehicle-card__bid-count">{{ listing.bidCountLabel }}</div>
        </div>
      </div>

      <div class="vehicle-card__actions">
        <label class="vehicle-card__compare" :class="{ 'vehicle-card__compare--selected': listing.isCompareSelected }">
          <input
            type="checkbox"
            :checked="listing.isCompareSelected"
            :disabled="listing.compareDisabledAdd"
            @change="toggleCompare"
          />
          Compare
        </label>
        <button
          type="button"
          class="vehicle-card__cta"
          :class="`vehicle-card__cta--${listing.ctaVariant}`"
          @click="openPreview"
        >
          {{ listing.ctaLabel }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.vehicle-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  transition: box-shadow 0.15s ease;
}

.vehicle-card:hover {
  box-shadow: var(--shadow-card-hover);
}

.vehicle-card__media {
  position: relative;
  height: 170px;
  cursor: pointer;
}

.vehicle-card__image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.vehicle-card__image-fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-inactive);
  background: var(--color-page-bg);
  font-size: 13px;
}

.vehicle-card__badge {
  position: absolute;
  top: 10px;
  left: 10px;
}

.vehicle-card__watch {
  position: absolute;
  top: 8px;
  right: 8px;
}

.vehicle-card__body {
  padding: 14px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}

.vehicle-card__heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
}

.vehicle-card__title-block {
  min-width: 0;
}

.vehicle-card__title {
  font-weight: 700;
  font-size: 15px;
  line-height: 1.25;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.vehicle-card__trim {
  font-size: 12px;
  color: var(--color-muted);
  margin-top: 2px;
}

.vehicle-card__meta {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12px;
  color: var(--color-muted);
  flex-wrap: wrap;
}

.vehicle-card__meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.vehicle-card__price-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 8px;
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid var(--color-divider);
}

.vehicle-card__price-label {
  font-size: 10.5px;
  color: var(--color-faint);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  font-weight: 700;
}

.vehicle-card__price {
  font-size: 18px;
  font-weight: 800;
  color: var(--color-navy);
}

.vehicle-card__time-block {
  text-align: right;
}

.vehicle-card__time {
  font-size: 12px;
  font-weight: 700;
  color: var(--color-muted);
}

.vehicle-card__time--urgent {
  color: var(--color-red-text);
}

.vehicle-card__time--upcoming {
  color: var(--color-accent);
}

.vehicle-card__bid-count {
  font-size: 11px;
  color: var(--color-muted);
}

.vehicle-card__actions {
  display: flex;
  gap: 8px;
}

.vehicle-card__compare {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-navy);
  font-size: 12.5px;
  font-weight: 600;
  border-radius: 8px;
  padding: 9px 6px;
  cursor: pointer;
}

.vehicle-card__compare--selected {
  border-color: var(--color-accent);
  background: var(--color-selected-bg);
}

.vehicle-card__cta {
  flex: 1.4;
  border: 1px solid transparent;
  font-weight: 700;
  font-size: 13px;
  border-radius: 8px;
  padding: 9px 6px;
  cursor: pointer;
}

.vehicle-card__cta--primary {
  background: var(--color-accent);
  color: var(--color-surface);
  border-color: var(--color-accent);
}

.vehicle-card__cta--outline-navy {
  background: var(--color-surface);
  color: var(--color-navy);
  border-color: var(--color-navy);
}

.vehicle-card__cta--outline-accent {
  background: var(--color-surface);
  color: var(--color-accent);
  border-color: var(--color-accent);
}
</style>
