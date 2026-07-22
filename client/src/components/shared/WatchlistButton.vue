<script setup lang="ts">
import { useWatchlistStore } from '@/stores/watchlist'
import type { AugmentedListing } from '@/types/listing'

const props = withDefaults(
  defineProps<{
    listing: AugmentedListing
    variant?: 'icon' | 'labeled'
    block?: boolean
  }>(),
  { variant: 'icon', block: false },
)

const watchlist = useWatchlistStore()

function toggle() {
  watchlist.toggle(props.listing.id)
}
</script>

<template>
  <button
    v-if="variant === 'icon'"
    type="button"
    class="watchlist-icon-button"
    :class="{ 'watchlist-icon-button--active': listing.isWatched }"
    :aria-label="listing.watchButtonLabel"
    :aria-pressed="listing.isWatched"
    @click="toggle"
  >
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      :fill="listing.isWatched ? 'currentColor' : 'none'"
      stroke="currentColor"
      stroke-width="2"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M6 3h12v18l-6-4-6 4V3Z" />
    </svg>
  </button>
  <button
    v-else
    type="button"
    class="watchlist-labeled-button"
    :class="{ 'watchlist-labeled-button--block': block }"
    :aria-pressed="listing.isWatched"
    @click="toggle"
  >
    {{ listing.watchButtonLabel }}
  </button>
</template>

<style scoped>
.watchlist-icon-button {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  border: none;
  background: var(--color-overlay-light);
  color: var(--color-inactive);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.watchlist-icon-button--active {
  color: var(--color-accent);
}

.watchlist-labeled-button {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-navy);
  font-weight: 700;
  font-size: 14px;
  padding: 12px;
  border-radius: 9px;
  cursor: pointer;
}

.watchlist-labeled-button--block {
  width: 100%;
}
</style>
