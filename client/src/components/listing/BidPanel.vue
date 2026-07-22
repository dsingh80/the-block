<script setup lang="ts">
import { ref } from 'vue'
import { useBidsStore } from '@/stores/bids'
import WatchlistButton from '@/components/shared/WatchlistButton.vue'
import { currency } from '@/utils/format'
import type { AugmentedListing } from '@/types/listing'

const props = defineProps<{ listing: AugmentedListing }>()

const bids = useBidsStore()
const bidInput = ref('')
const error = ref<string | null>(null)
const success = ref<string | null>(null)

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

  const result = bids.placeBid(props.listing.id, amount)
  if (!result.ok) {
    error.value = result.error
    return
  }

  success.value = `Bid placed at ${currency(amount)}.`
  bidInput.value = ''
}
</script>

<template>
  <div class="bid-panel">
    <div class="bid-panel__status" :class="`bid-panel__status--${listing.bidStatusVariant}`">
      {{ listing.bidStatusText }}
    </div>

    <div>
      <div class="bid-panel__price-label">{{ listing.priceLabelText }}</div>
      <div class="bid-panel__price">{{ listing.priceFormatted }}</div>
    </div>

    <div class="bid-panel__divider-row">
      <span>{{ listing.bidCountLabel }}</span>
      <span class="bid-panel__time" :class="`bid-panel__time--${listing.timeVariant}`">{{
        listing.timeLabel
      }}</span>
    </div>

    <template v-if="listing.canBid">
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

.bid-panel__ended {
  background: var(--color-page-bg);
  border-radius: 10px;
  padding: 14px;
  text-align: center;
  font-size: 13px;
  font-weight: 700;
  color: var(--color-muted);
}
</style>
