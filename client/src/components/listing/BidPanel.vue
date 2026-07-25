<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useBidsStore } from '@/stores/bids'
import { useClockStore } from '@/stores/clock'
import WatchlistButton from '@/components/shared/WatchlistButton.vue'
import { currency } from '@/utils/format'
import { getBidIncrement } from '@/utils/bidding'
import { timeSince } from '@/utils/time'
import { BID_UPDATE_HIGHLIGHT_MS } from '@/utils/constants'
import type { AugmentedListing } from '@/types/listing'

const props = defineProps<{ listing: AugmentedListing }>()

const bids = useBidsStore()
const clock = useClockStore()
const bidInput = ref(String(props.listing.nextBidValue))
const error = ref<string | null>(null)
const success = ref<string | null>(null)
const showRaiseConfirm = ref(false)
const pendingAmount = ref<number | null>(null)
const confirmDialogRef = ref<HTMLElement | null>(null)

/**
 * Fires on any change to the price this component is handed, regardless of
 * source -- a WS-broadcast bid from another session (useRealtimeSync) and
 * this session's own accepted bid both land the same way, as a new
 * `listing.priceValue` prop. lastBidAt is read from the clock store rather
 * than Date.now() so "time since" stays on the same simulated-or-real clock
 * as every other lifecycle read (guidelines/03-guardrails.md).
 */
const justUpdated = ref(false)
const lastBidAt = ref(clock.effectiveNow)
let flashTimer: ReturnType<typeof setTimeout> | undefined

watch(
  () => props.listing.priceValue,
  () => {
    lastBidAt.value = clock.effectiveNow
    justUpdated.value = true
    clearTimeout(flashTimer)
    flashTimer = setTimeout(() => {
      justUpdated.value = false
    }, BID_UPDATE_HIGHLIGHT_MS)
  },
)

onUnmounted(() => clearTimeout(flashTimer))

const lastBidLabel = computed(() => timeSince(clock.effectiveNow - lastBidAt.value))

// Tracks the live minimum as the price moves, but only while the field still
// holds the auto-filled value -- once the user types their own amount it's
// theirs to keep, not something a background price update should overwrite.
let lastAutofilled = props.listing.nextBidValue
watch(
  () => props.listing.nextBidValue,
  (next) => {
    if (bidInput.value === String(lastAutofilled)) bidInput.value = String(next)
    lastAutofilled = next
  },
)

watch(showRaiseConfirm, async (open) => {
  if (!open) return
  await nextTick()
  confirmDialogRef.value?.focus()
})

function parseAmount(raw: string): number {
  return Number(raw.replace(/[^0-9.]/g, ''))
}

/**
 * The UI-level checks here are just fast feedback — the real gate is
 * bids.placeBid itself, which re-derives the minimum and re-checks the
 * listing is still active. See guidelines/03-guardrails.md.
 */
function submitBid() {
  error.value = null
  success.value = null

  if (!bidInput.value.trim()) {
    error.value = 'Enter a bid amount.'
    return
  }

  const amount = parseAmount(bidInput.value)
  if (!Number.isFinite(amount) || amount <= 0) {
    error.value = 'Enter a valid bid amount.'
    return
  }

  if (props.listing.isUserHighBidder) {
    pendingAmount.value = amount
    showRaiseConfirm.value = true
    return
  }

  void placeBid(amount)
}

async function placeBid(amount: number) {
  const result = await bids.placeBid(props.listing.id, amount)
  if (!result.ok) {
    error.value = result.error
    return
  }

  success.value = `Bid placed at ${currency(amount)}.`
  // The server confirms `amount` as the new current_bid (guidelines/06-backend-architecture.md's
  // `viewer` on a bid acceptance is a fixed constant, not a live lookup) --
  // deriving the next minimum from it directly is instant and doesn't wait on
  // the parent's prop update to flow back down.
  lastAutofilled = amount + getBidIncrement(amount)
  bidInput.value = String(lastAutofilled)
}

function confirmRaise() {
  showRaiseConfirm.value = false
  const amount = pendingAmount.value
  pendingAmount.value = null
  if (amount != null) void placeBid(amount)
}

function cancelRaise() {
  showRaiseConfirm.value = false
  pendingAmount.value = null
}

function handleConfirmBackdropClick(event: MouseEvent) {
  if (event.target === event.currentTarget) cancelRaise()
}
</script>

<template>
  <div class="bid-panel">
    <div class="bid-panel__status" :class="`bid-panel__status--${listing.bidStatusVariant}`">
      {{ listing.bidStatusText }}
    </div>

    <div>
      <div class="bid-panel__price-label">{{ listing.priceLabelText }}</div>
      <div class="bid-panel__price-row">
        <div class="bid-panel__price" :class="{ 'bid-panel__price--flash': justUpdated }">
          {{ listing.priceFormatted }}
        </div>
        <span v-if="justUpdated" class="bid-panel__update-badge" role="status">
          <span class="bid-panel__update-dot" aria-hidden="true"></span>New bid
        </span>
      </div>
    </div>

    <div class="bid-panel__divider-row">
      <span class="bid-panel__bid-meta">
        {{ listing.bidCountLabel }}
        <span v-if="!listing.isUpcoming" class="bid-panel__last-bid">· {{ lastBidLabel }}</span>
      </span>
      <span class="bid-panel__time" :class="`bid-panel__time--${listing.timeVariant}`">{{
        listing.timeLabel
      }}</span>
    </div>

    <template v-if="listing.canBid">
      <div v-if="listing.isUserHighBidder" class="bid-panel__winning">
        You have the highest bid — you can still raise it below.
      </div>
      <div class="bid-panel__form">
        <label class="bid-panel__label" for="bid-amount">Your max proxy bid</label>
        <input
          id="bid-amount"
          v-model="bidInput"
          type="text"
          inputmode="numeric"
          placeholder="$ Enter amount"
          @keydown.enter="submitBid"
        />
      </div>
      <button type="button" class="bid-panel__submit" @click="submitBid">Place Bid</button>
      <p v-if="error" class="bid-panel__feedback bid-panel__feedback--error" role="alert">
        {{ error }}
      </p>
      <p v-if="success" class="bid-panel__feedback bid-panel__feedback--success" role="status">
        {{ success }}
      </p>
      <p class="bid-panel__disclaimer">
        Proxy bidding: you're only charged one increment above the next highest bid, up to your
        max. Your max is never shown to other buyers.
      </p>
    </template>

    <template v-else-if="listing.isUpcoming">
      <WatchlistButton :listing="listing" variant="labeled" block />
      <p class="bid-panel__disclaimer">
        This auction hasn't started yet. Watch it to get notified when bidding opens.
      </p>
    </template>

    <template v-else>
      <div class="bid-panel__ended">Auction Ended</div>
    </template>

    <div
      v-if="showRaiseConfirm"
      class="bid-panel__confirm-overlay"
      role="presentation"
      @click="handleConfirmBackdropClick"
    >
      <div
        ref="confirmDialogRef"
        class="bid-panel__confirm"
        role="dialog"
        aria-modal="true"
        aria-labelledby="raise-confirm-title"
        tabindex="-1"
        @keydown.esc="cancelRaise"
      >
        <p id="raise-confirm-title" class="bid-panel__confirm-title">
          You are already the winning bidder. Would you still like to raise your bid?
        </p>
        <div class="bid-panel__confirm-actions">
          <button type="button" class="bid-panel__confirm-no" @click="cancelRaise">No</button>
          <button type="button" class="bid-panel__confirm-yes" @click="confirmRaise">Yes</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bid-panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  padding: 22px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.bid-panel__status {
  font-size: 13px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.bid-panel__status--accent {
  color: var(--color-accent);
}

.bid-panel__status--success {
  color: var(--color-green-text);
}

.bid-panel__status--muted {
  color: var(--color-muted);
}

.bid-panel__status--danger {
  color: var(--color-red-text);
}

.bid-panel__price-label {
  font-size: 11px;
  color: var(--color-faint);
  text-transform: uppercase;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.bid-panel__price {
  font-size: 32px;
  font-weight: 800;
  color: var(--color-navy);
}

.bid-panel__divider-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  color: var(--color-muted);
  border-top: 1px solid var(--color-divider);
  border-bottom: 1px solid var(--color-divider);
  padding: 10px 0;
}

.bid-panel__time--urgent {
  color: var(--color-red-text);
  font-weight: 700;
}

.bid-panel__time--upcoming {
  color: var(--color-accent);
  font-weight: 700;
}

.bid-panel__label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted);
  margin-bottom: 6px;
}

.bid-panel__form input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 12px;
  font-size: 15px;
  font-weight: 600;
  outline: none;
}

.bid-panel__submit {
  width: 100%;
  border: none;
  background: var(--color-accent);
  color: var(--color-surface);
  font-weight: 800;
  font-size: 15px;
  padding: 14px;
  border-radius: 10px;
  cursor: pointer;
}

.bid-panel__feedback {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}

.bid-panel__feedback--error {
  color: var(--color-red-text);
}

.bid-panel__feedback--success {
  color: var(--color-green-text);
}

.bid-panel__disclaimer {
  font-size: 11.5px;
  color: var(--color-faint);
  line-height: 1.5;
  margin: 0;
}

.bid-panel__winning {
  background: var(--color-green-bg);
  border: 1px solid var(--color-green-text);
  color: var(--color-green-text);
  border-radius: 10px;
  padding: 14px;
  text-align: center;
  font-size: 13px;
  font-weight: 700;
}

.bid-panel__ended {
  background: var(--color-page-bg);
  border-radius: 10px;
  padding: 14px;
  text-align: center;
  font-size: 13px;
  font-weight: 700;
  color: var(--color-muted);
}

.bid-panel__price-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.bid-panel__price--flash {
  animation: bid-price-flash 2s ease;
  transform-origin: left center;
}

.bid-panel__update-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: var(--color-selected-bg);
  color: var(--color-accent);
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.02em;
  padding: 3px 8px;
  border-radius: 20px;
  animation: bid-badge-fade 2s ease;
}

.bid-panel__update-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-accent);
  animation: bid-dot-pulse 0.8s ease-in-out infinite;
}

.bid-panel__bid-meta {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
}

.bid-panel__last-bid {
  color: var(--color-faint);
  font-weight: 500;
}

.bid-panel__confirm-overlay {
  position: fixed;
  inset: 0;
  background: var(--color-overlay-dark);
  backdrop-filter: blur(2px);
  z-index: var(--z-confirm-modal);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.bid-panel__confirm {
  background: var(--color-surface);
  border-radius: 14px;
  max-width: min(360px, calc(100vw - 32px));
  width: 100%;
  padding: 22px;
  display: flex;
  flex-direction: column;
  gap: 18px;
  outline: none;
}

.bid-panel__confirm-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--color-navy);
  line-height: 1.4;
  text-align: center;
}

.bid-panel__confirm-actions {
  display: flex;
  gap: 10px;
}

.bid-panel__confirm-no,
.bid-panel__confirm-yes {
  flex: 1;
  border-radius: 9px;
  padding: 12px;
  font-weight: 800;
  font-size: 14px;
  cursor: pointer;
}

.bid-panel__confirm-no {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-navy);
}

.bid-panel__confirm-yes {
  border: 1px solid var(--color-accent);
  background: var(--color-accent);
  color: var(--color-surface);
}
</style>
