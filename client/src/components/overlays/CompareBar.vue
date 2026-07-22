<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useCompareStore } from '@/stores/compare'
import { useAugmentedListings } from '@/composables/useListingPresentation'
import type { AugmentedListing } from '@/types/listing'

const compare = useCompareStore()
const route = useRoute()
const { list } = useAugmentedListings()

const compareListings = computed<AugmentedListing[]>(() =>
  compare.ids
    .map((id) => list.value.find((listing) => listing.id === id))
    .filter((listing): listing is AugmentedListing => !!listing),
)
</script>

<template>
  <div v-if="compare.ids.length > 0 && route.path === '/inventory'" class="compare-bar">
    <div class="compare-bar__thumbs">
      <div v-for="item in compareListings" :key="item.id" class="compare-bar__thumb">
        <img
          v-if="item.images[0]"
          :src="item.images[0]"
          :alt="`${item.vehicle.year} ${item.vehicle.make} ${item.vehicle.model}`"
          loading="lazy"
        />
        <button
          type="button"
          class="compare-bar__remove"
          :aria-label="`Remove ${item.vehicle.year} ${item.vehicle.make} ${item.vehicle.model} from comparison`"
          @click="compare.remove(item.id)"
        >
          ×
        </button>
      </div>
      <div v-if="compare.ids.length === 1" class="compare-bar__placeholder" aria-hidden="true">+</div>
    </div>
    <button
      type="button"
      class="compare-bar__compare"
      :disabled="compare.ids.length < 2"
      @click="compare.openModal"
    >
      Compare
    </button>
    <button type="button" class="compare-bar__clear" @click="compare.clear">Clear</button>
  </div>
</template>

<style scoped>
.compare-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 16px;
  z-index: var(--z-compare-bar);
  display: flex;
  justify-content: center;
  gap: 16px;
  align-items: center;
  background: var(--color-navy);
  color: var(--color-surface);
  border-radius: 14px;
  padding: 12px 20px;
  box-shadow: var(--shadow-compare-bar);
  flex-wrap: wrap;
  max-width: calc(100vw - 32px);
  margin: 0 auto;
  width: fit-content;
}

.compare-bar__thumbs {
  display: flex;
  align-items: center;
  gap: 10px;
}

.compare-bar__thumb {
  position: relative;
  width: 44px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 6px;
  overflow: hidden;
  background: var(--color-divider);
}

.compare-bar__thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.compare-bar__remove {
  position: absolute;
  top: -7px;
  right: -7px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.compare-bar__placeholder {
  width: 44px;
  height: 36px;
  border-radius: 6px;
  border: 1.5px dashed var(--color-overlay-light-faint);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-overlay-light-faint);
  font-size: 18px;
  flex-shrink: 0;
}

.compare-bar__compare {
  border: none;
  background: var(--color-accent);
  color: var(--color-surface);
  font-weight: 700;
  font-size: 13px;
  padding: 9px 18px;
  border-radius: 8px;
  cursor: pointer;
}

.compare-bar__compare:disabled {
  background: var(--color-muted);
  cursor: not-allowed;
}

.compare-bar__clear {
  border: none;
  background: none;
  color: var(--color-overlay-light-muted);
  font-size: 13px;
  cursor: pointer;
  text-decoration: underline;
}
</style>
