<script setup lang="ts">
import type { AugmentedListing } from '@/types/listing'

defineProps<{ listing: AugmentedListing }>()
</script>

<template>
  <span v-if="listing.badgeVariant" class="bid-badge" :class="`bid-badge--${listing.badgeVariant}`">
    {{ listing.badgeText }}
  </span>
</template>

<style scoped>
.bid-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 800;
  padding: 4px 10px;
  border-radius: 20px;
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.bid-badge--winning,
.bid-badge--won {
  background: var(--color-green-bg);
  color: var(--color-green-text);
}

.bid-badge--outbid {
  background: var(--color-red-bg);
  color: var(--color-red-text);
}

.bid-badge--bidding {
  background: var(--color-selected-bg);
  color: var(--color-accent);
}

.bid-badge--lost {
  background: var(--color-neutral-bg);
  color: var(--color-neutral-text);
}
</style>
