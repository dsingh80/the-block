import { onScopeDispose, watchEffect } from 'vue'
import { realtimeConnection, type WsServerMessage } from '@/services/api/ws'
import { useBidsStore } from '@/stores/bids'
import { useInventoryFiltersStore } from '@/stores/inventoryFilters'

/**
 * Routes an incoming WS message into whichever store(s) it's actually about
 * (guidelines/06-backend-architecture.md, "WebSocket protocol"). Bidding
 * itself is confirmed to the acting session synchronously over REST
 * (services/api/listings.ts's placeBid/buyNow) -- this is for keeping every
 * *other* subscribed connection's view of a listing live.
 */
export function handleRealtimeMessage(message: WsServerMessage): void {
  const filters = useInventoryFiltersStore()
  const bids = useBidsStore()

  if (message.type === 'bid_accepted') {
    const vehicle = filters.vehiclesById[message.listing_id]
    if (vehicle) {
      vehicle.current_bid = message.current_bid
      vehicle.bid_count = message.bid_count
    }

    const existingOverride = bids.overrides[message.listing_id]
    if (message.high_bidder_is_you) {
      bids.overrides[message.listing_id] = {
        currentPrice: message.current_bid,
        bidCount: message.bid_count,
        hasUserBid: true,
        isUserHighBidder: true,
        isUserOutbid: false,
        purchased: existingOverride?.purchased ?? false,
        purchasedAt: existingOverride?.purchasedAt ?? null,
      }
    } else if (existingOverride && existingOverride.hasUserBid) {
      // This session had bid on this listing and someone else's bid just
      // landed -- they've gone from winning/bidding to outbid.
      bids.overrides[message.listing_id] = {
        ...existingOverride,
        currentPrice: message.current_bid,
        bidCount: message.bid_count,
        isUserHighBidder: false,
        isUserOutbid: true,
      }
    }
  } else if (message.type === 'listing_ended') {
    // No high_bidder_is_you-equivalent field here: a Buy Now is confirmed to
    // its own buyer synchronously over REST already (guidelines/06-backend-architecture.md,
    // "viewer on a bid/buy-now acceptance response"); this broadcast is for
    // every *other* subscriber, so only the listing's own state needs updating.
    const vehicle = filters.vehiclesById[message.listing_id]
    if (vehicle) {
      vehicle.current_bid = message.final_price
      vehicle.purchased_at = new Date().toISOString()
    }
  }
}

let started = false

/** Starts the shared connection + message routing exactly once, regardless of how many components call this. */
function ensureStarted() {
  if (started) return
  started = true
  realtimeConnection.connect()
  realtimeConnection.onMessage(handleRealtimeMessage)
}

/**
 * Subscribes to whatever `ids()` currently returns for as long as the calling
 * component/composable's effect scope is alive, unsubscribing individual ids
 * as they drop out and everything on scope disposal -- the client-side
 * mirror of the server's own per-connection subscription set.
 */
export function useRealtimeSubscription(ids: () => string[]): void {
  ensureStarted()

  let current: string[] = []
  const stop = watchEffect(() => {
    const next = ids()
    const toAdd = next.filter((id) => !current.includes(id))
    const toRemove = current.filter((id) => !next.includes(id))
    if (toAdd.length > 0) realtimeConnection.subscribe(toAdd)
    if (toRemove.length > 0) realtimeConnection.unsubscribe(toRemove)
    current = next
  })

  onScopeDispose(() => {
    stop()
    if (current.length > 0) realtimeConnection.unsubscribe(current)
  })
}
