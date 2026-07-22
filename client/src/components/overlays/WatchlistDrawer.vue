<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useWatchlistStore } from '@/stores/watchlist'
import { useBidsStore } from '@/stores/bids'
import { useWatchlistSections } from '@/composables/useWatchlistSections'
import BidBadge from '@/components/shared/BidBadge.vue'
import type { AugmentedListing } from '@/types/listing'

const watchlist = useWatchlistStore()
const bids = useBidsStore()
const router = useRouter()
const { activeBidItems, watchingItems, drawerCount } = useWatchlistSections()

/**
 * Drawer rows go straight to the details page and close the drawer,
 * rather than opening a second Preview Modal stacked on top of an
 * already-open drawer — a deliberate, minor deviation from strict
 * card-CTA parity (cards always go through the preview modal first).
 */
function openListing(id: string) {
  watchlist.drawerOpen = false
  router.push(`/inventory/${id}`)
}

function quickBid(listing: AugmentedListing) {
  bids.placeBid(listing.id, listing.nextBidValue)
}
</script>

<template>
  <button
    type="button"
    class="watchlist-tab"
    :class="{ 'watchlist-tab--shifted': watchlist.drawerOpen }"
    :aria-expanded="watchlist.drawerOpen"
    aria-controls="watchlist-drawer-panel"
    aria-label="Toggle watchlist"
    @click="watchlist.toggleDrawer"
  >
    <span class="watchlist-tab__count">{{ drawerCount }}</span>
    <svg
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M6 3h12v18l-6-4-6 4V3Z" />
    </svg>
    <span class="watchlist-tab__label">WATCHLIST</span>
  </button>

  <div id="watchlist-drawer-panel" class="watchlist-drawer" :class="{ 'watchlist-drawer--open': watchlist.drawerOpen }">
    <div class="watchlist-drawer__panel">
      <div class="watchlist-drawer__header">
        <div class="watchlist-drawer__title">Watchlist</div>
        <div class="watchlist-drawer__subtitle">
          Live status for auctions you're bidding on or watching
        </div>
      </div>

      <div class="watchlist-drawer__body">
        <template v-if="activeBidItems.length > 0">
          <div class="watchlist-drawer__section-label">Needs Your Attention</div>
          <div
            v-for="item in activeBidItems"
            :key="item.id"
            class="watchlist-drawer__row"
            :class="{ 'watchlist-drawer__row--highlight': item.id === watchlist.recentlyAddedId }"
          >
            <div class="watchlist-drawer__row-main" @click="openListing(item.id)">
              <img
                v-if="item.images[0]"
                :src="item.images[0]"
                :alt="`${item.vehicle.year} ${item.vehicle.make} ${item.vehicle.model}`"
                class="watchlist-drawer__thumb"
                loading="lazy"
              />
              <div class="watchlist-drawer__row-info">
                <div class="watchlist-drawer__row-title">
                  {{ item.vehicle.year }} {{ item.vehicle.make }} {{ item.vehicle.model }}
                </div>
                <BidBadge :listing="item" class="watchlist-drawer__badge" />
              </div>
            </div>
            <div class="watchlist-drawer__row-footer">
              <div>
                <div class="watchlist-drawer__price">{{ item.priceFormatted }}</div>
                <div class="watchlist-drawer__time" :class="`watchlist-drawer__time--${item.timeVariant}`">
                  {{ item.timeLabel }}
                </div>
              </div>
              <div class="watchlist-drawer__row-actions">
                <button type="button" class="watchlist-drawer__quick-bid" @click="quickBid(item)">
                  Bid {{ item.nextBidFormatted }}
                </button>
                <button type="button" class="watchlist-drawer__view" @click="openListing(item.id)">
                  View
                </button>
              </div>
            </div>
          </div>
        </template>

        <template v-if="watchingItems.length > 0">
          <div class="watchlist-drawer__section-label">Watching</div>
          <div
            v-for="item in watchingItems"
            :key="item.id"
            class="watchlist-drawer__row"
            :class="{ 'watchlist-drawer__row--highlight': item.id === watchlist.recentlyAddedId }"
          >
            <div class="watchlist-drawer__row-main" @click="openListing(item.id)">
              <img
                v-if="item.images[0]"
                :src="item.images[0]"
                :alt="`${item.vehicle.year} ${item.vehicle.make} ${item.vehicle.model}`"
                class="watchlist-drawer__thumb"
                loading="lazy"
              />
              <div class="watchlist-drawer__row-info">
                <div class="watchlist-drawer__row-title">
                  {{ item.vehicle.year }} {{ item.vehicle.make }} {{ item.vehicle.model }}
                </div>
                <div class="watchlist-drawer__bid-count">{{ item.bidCountLabel }}</div>
              </div>
            </div>
            <div class="watchlist-drawer__row-footer">
              <div>
                <div class="watchlist-drawer__price">{{ item.priceFormatted }}</div>
                <div class="watchlist-drawer__time" :class="`watchlist-drawer__time--${item.timeVariant}`">
                  {{ item.timeLabel }}
                </div>
              </div>
              <button
                type="button"
                class="watchlist-drawer__cta"
                :class="`watchlist-drawer__cta--${item.ctaVariant}`"
                @click="openListing(item.id)"
              >
                {{ item.ctaLabel }}
              </button>
            </div>
          </div>
        </template>

        <div v-if="drawerCount === 0" class="watchlist-drawer__empty">
          Bid on or watch an auction to see live updates here.
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.watchlist-tab {
  position: fixed;
  top: 50%;
  right: 0;
  transform: translateY(-50%);
  width: 44px;
  height: 200px;
  background: var(--color-navy);
  color: var(--color-surface);
  border: none;
  border-radius: 10px 0 0 10px;
  box-shadow: var(--shadow-drawer-tab);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  cursor: pointer;
  padding: 14px 0;
  transition: right 0.25s ease;
  z-index: var(--z-watchlist-tab);
  outline: none;
}

/* Matches the drawer's own min(340px, 100vw) width, so the tab tracks the panel edge exactly on any viewport. */
.watchlist-tab--shifted {
  right: min(340px, 100vw);
}

.watchlist-tab:focus-visible {
  box-shadow:
    var(--shadow-drawer-tab),
    var(--shadow-focus-ring);
}

.watchlist-tab__count {
  background: var(--color-logo-accent);
  color: var(--color-surface);
  font-size: 10.5px;
  font-weight: 800;
  border-radius: 20px;
  padding: 2px 7px;
  flex-shrink: 0;
}

.watchlist-tab__label {
  writing-mode: vertical-rl;
  text-orientation: mixed;
  white-space: nowrap;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.04em;
}

.watchlist-drawer {
  position: fixed;
  top: 62px;
  right: 0;
  bottom: 0;
  width: min(340px, 100vw);
  transform: translateX(100%);
  transition: transform 0.25s ease;
  z-index: var(--z-watchlist-drawer);
  display: flex;
}

.watchlist-drawer--open {
  transform: translateX(0);
}

.watchlist-drawer__panel {
  flex: 1;
  background: var(--color-surface);
  border-left: 1px solid var(--color-border);
  box-shadow: var(--shadow-drawer-panel);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.watchlist-drawer__header {
  padding: 16px 18px;
  border-bottom: 1px solid var(--color-divider);
}

.watchlist-drawer__title {
  font-size: 16px;
  font-weight: 800;
  color: var(--color-navy);
}

.watchlist-drawer__subtitle {
  font-size: 12px;
  color: var(--color-muted);
  margin-top: 2px;
}

.watchlist-drawer__body {
  flex: 1;
  overflow-y: auto;
  padding: 10px 14px 20px;
}

.watchlist-drawer__section-label {
  font-size: 11px;
  font-weight: 800;
  color: var(--color-faint);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 10px 4px 6px;
}

.watchlist-drawer__row {
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 10px;
  margin-bottom: 10px;
}

.watchlist-drawer__row--highlight {
  animation: watchlist-highlight 1.5s ease;
}

.watchlist-drawer__row-main {
  display: flex;
  gap: 10px;
  cursor: pointer;
}

.watchlist-drawer__thumb {
  width: 52px;
  height: 40px;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
  object-fit: cover;
  background: var(--color-divider);
}

.watchlist-drawer__row-info {
  min-width: 0;
  flex: 1;
}

.watchlist-drawer__row-title {
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.watchlist-drawer__badge {
  display: inline-block;
  margin-top: 3px;
}

.watchlist-drawer__bid-count {
  font-size: 11.5px;
  font-weight: 700;
  color: var(--color-muted);
  margin-top: 2px;
}

.watchlist-drawer__row-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 8px;
}

.watchlist-drawer__price {
  font-size: 15px;
  font-weight: 800;
  color: var(--color-navy);
}

.watchlist-drawer__time {
  font-size: 11px;
  font-weight: 700;
  color: var(--color-muted);
}

.watchlist-drawer__time--urgent {
  color: var(--color-red-text);
}

.watchlist-drawer__time--upcoming {
  color: var(--color-accent);
}

.watchlist-drawer__row-actions {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-end;
}

.watchlist-drawer__quick-bid {
  border: none;
  background: var(--color-accent);
  color: var(--color-surface);
  font-size: 11.5px;
  font-weight: 700;
  padding: 6px 10px;
  border-radius: 6px;
  cursor: pointer;
  white-space: nowrap;
}

.watchlist-drawer__view {
  border: none;
  background: none;
  padding: 0;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-muted);
  cursor: pointer;
  text-decoration: underline;
}

.watchlist-drawer__cta {
  border: 1px solid transparent;
  font-weight: 700;
  font-size: 11.5px;
  border-radius: 6px;
  padding: 6px 10px;
  cursor: pointer;
  white-space: nowrap;
}

.watchlist-drawer__cta--primary {
  background: var(--color-accent);
  color: var(--color-surface);
  border-color: var(--color-accent);
}

.watchlist-drawer__cta--outline-navy {
  background: var(--color-surface);
  color: var(--color-navy);
  border-color: var(--color-navy);
}

.watchlist-drawer__cta--outline-accent {
  background: var(--color-surface);
  color: var(--color-accent);
  border-color: var(--color-accent);
}

.watchlist-drawer__empty {
  padding: 30px 10px;
  text-align: center;
  font-size: 13px;
  color: var(--color-muted);
}

@media (max-width: 480px) {
  .watchlist-drawer {
    width: 100vw;
  }
}
</style>
