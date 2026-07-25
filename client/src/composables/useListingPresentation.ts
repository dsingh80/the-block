import { computed, toValue, watchEffect, type MaybeRefOrGetter } from 'vue'
import { useClockStore } from '@/stores/clock'
import { useBidsStore } from '@/stores/bids'
import { useWatchlistStore } from '@/stores/watchlist'
import { useCompareStore } from '@/stores/compare'
import { useInventoryFiltersStore } from '@/stores/inventoryFilters'
import { deriveLifecycle } from '@/utils/lifecycle'
import { getBidIncrement } from '@/utils/bidding'
import { currency, formatKm } from '@/utils/format'
import { gradeVariant, gradeLabel, gradeTooltip, titleLabel } from '@/utils/grading'
import { timeLabelFor } from '@/utils/time'
import { AUCTION_DURATION_HOURS, URGENT_THRESHOLD_HOURS } from '@/utils/constants'
import type { Vehicle } from '@/types/vehicle'
import type { AugmentedListing, BadgeVariant, BidOverride, BidStatusVariant, CtaVariant, TimeVariant } from '@/types/listing'

const HOUR_MS = 60 * 60 * 1000

interface AugmentContext {
  effectiveNow: number
  override: BidOverride | undefined
  justBoughtId: string | null
  isWatched: boolean
  isCompareSelected: boolean
  compareDisabledAdd: boolean
}

function computeBadge(
  lifecycle: 'upcoming' | 'active' | 'ended',
  hasUserBid: boolean,
  isUserHighBidder: boolean,
  isUserOutbid: boolean,
): { variant: BadgeVariant; text: string } {
  if (!hasUserBid) return { variant: null, text: '' }
  if (lifecycle === 'active') {
    if (isUserHighBidder) return { variant: 'winning', text: 'Winning' }
    if (isUserOutbid) return { variant: 'outbid', text: 'Outbid' }
    return { variant: 'bidding', text: 'Bidding' }
  }
  return isUserHighBidder ? { variant: 'won', text: 'Won' } : { variant: 'lost', text: 'Lost' }
}

function computeBidStatus(
  isUpcoming: boolean,
  isEnded: boolean,
  hasUserBid: boolean,
  isUserHighBidder: boolean,
  isUserOutbid: boolean,
): { text: string; variant: BidStatusVariant } {
  if (isUpcoming) return { text: 'Auction Not Started', variant: 'muted' }
  if (isEnded) {
    if (isUserHighBidder) return { text: 'You Won This Auction', variant: 'success' }
    if (hasUserBid) return { text: 'You Did Not Win', variant: 'muted' }
    return { text: 'Auction Ended', variant: 'muted' }
  }
  if (isUserHighBidder) return { text: 'You Are Winning', variant: 'success' }
  if (isUserOutbid) return { text: 'You Have Been Outbid', variant: 'danger' }
  return { text: 'Live Auction', variant: 'accent' }
}

/**
 * The single source of presentation truth for a listing — ports the
 * approved design mock's augment() method against the real vehicle schema.
 * Every component reads its fields; nothing re-derives a color, badge, or
 * label inline. See guidelines/02-design-patterns.md #2 and
 * guidelines/03-guardrails.md.
 */
export function augment(vehicle: Vehicle, ctx: AugmentContext): AugmentedListing {
  const purchasedByMe = ctx.override?.purchased ?? false
  // vehicle.purchased_at is the server's authority on an early end (a Buy Now,
  // possibly by another session) -- something the client can no longer derive
  // from auction_start + a fixed duration alone once other sessions can end a
  // listing early. Local time-based derivation still owns the ordinary
  // upcoming -> active -> ended-by-time-passing transitions between fetches.
  const purchasedAtMs = vehicle.purchased_at != null ? new Date(vehicle.purchased_at).getTime() : null
  const purchased = purchasedByMe || purchasedAtMs != null
  const lifecycle = purchased ? 'ended' : deriveLifecycle(vehicle.auction_start, ctx.effectiveNow)
  const isUpcoming = lifecycle === 'upcoming'
  const isEnded = lifecycle === 'ended'
  const canBid = !isUpcoming && !isEnded

  const hasUserBid = ctx.override?.hasUserBid ?? false
  const isUserHighBidder = ctx.override?.isUserHighBidder ?? false
  const isUserOutbid = ctx.override?.isUserOutbid ?? false

  const priceValue = ctx.override?.currentPrice ?? vehicle.current_bid ?? vehicle.starting_bid
  const bidCount = ctx.override?.bidCount ?? vehicle.bid_count
  const hasAnyBid = hasUserBid || vehicle.current_bid != null

  let priceLabelText: AugmentedListing['priceLabelText']
  if (isUpcoming) priceLabelText = 'Opening Bid'
  else if (!hasAnyBid) priceLabelText = 'Starting Bid'
  else if (isEnded) priceLabelText = 'Winning Bid'
  else priceLabelText = 'Current Bid'

  const bidCountLabel = isUpcoming ? 'Not started yet' : `${bidCount} bid${bidCount === 1 ? '' : 's'}`

  const nextBidValue = priceValue + getBidIncrement(priceValue)

  const startMs = new Date(vehicle.auction_start).getTime()
  // A purchase's real timestamp stands in for the fixed-24h assumption once
  // the listing actually ended that way -- otherwise "Ended Xh ago" would be
  // measured against a duration that was never the reason it ended.
  const endMs = purchasedAtMs ?? startMs + AUCTION_DURATION_HOURS * HOUR_MS
  const hoursRemaining = isUpcoming
    ? (startMs - ctx.effectiveNow) / HOUR_MS
    : (endMs - ctx.effectiveNow) / HOUR_MS

  const urgent = lifecycle === 'active' && hoursRemaining <= URGENT_THRESHOLD_HOURS
  let timeVariant: TimeVariant = 'default'
  if (urgent) timeVariant = 'urgent'
  else if (isUpcoming) timeVariant = 'upcoming'

  // "just now" is specifically about *this* session's own completed purchase,
  // not merely "the listing happens to be ended by a purchase" -- someone
  // else's earlier Buy Now still gets the ordinary "Ended Xh ago" label.
  const justBought = ctx.justBoughtId === vehicle.id
  const timeLabel = justBought ? 'Purchased just now' : timeLabelFor(lifecycle, hoursRemaining)

  const badge = computeBadge(lifecycle, hasUserBid, isUserHighBidder, isUserOutbid)
  const bidStatus = computeBidStatus(isUpcoming, isEnded, hasUserBid, isUserHighBidder, isUserOutbid)

  const ctaLabel = isEnded ? 'View Result' : hasUserBid ? 'Place New Bid' : 'View Auction'
  let ctaVariant: CtaVariant = 'primary'
  if (isEnded) ctaVariant = 'outline-navy'
  else if (isUpcoming) ctaVariant = 'outline-accent'

  return {
    id: vehicle.id,
    vehicle,

    lifecycle,
    isUpcoming,
    isEnded,
    canBid,

    mileageLabel: formatKm(vehicle.odometer_km),
    locationLabel: `${vehicle.city}, ${vehicle.province}`,

    gradeVariant: gradeVariant(vehicle.condition_grade),
    gradeLabel: gradeLabel(vehicle.condition_grade),
    gradeTooltip: gradeTooltip(vehicle.condition_grade),

    titleVariant: vehicle.title_status,
    titleLabel: titleLabel(vehicle.title_status),

    priceValue,
    priceFormatted: currency(priceValue),
    priceLabelText,
    bidCountLabel,
    nextBidValue,
    nextBidFormatted: currency(nextBidValue),

    hoursRemaining,
    timeLabel,
    timeVariant,

    hasUserBid,
    isUserHighBidder,
    isUserOutbid,

    badgeVariant: badge.variant,
    badgeText: badge.text,

    bidStatusText: bidStatus.text,
    bidStatusVariant: bidStatus.variant,

    showBuyNow: !isEnded && vehicle.buy_now_price != null,
    buyNowFormatted: vehicle.buy_now_price != null ? currency(vehicle.buy_now_price) : null,
    justBought,

    isWatched: ctx.isWatched,
    watchButtonLabel: ctx.isWatched ? 'Watching ✓' : 'Add to Watchlist',

    isCompareSelected: ctx.isCompareSelected,
    compareDisabledAdd: ctx.compareDisabledAdd,

    ctaLabel,
    ctaVariant,

    images: vehicle.images,
    damageList: vehicle.damage_notes.length > 0 ? vehicle.damage_notes : ['No reported damage.'],
  }
}

function buildContext(
  vehicleId: string,
  clock: ReturnType<typeof useClockStore>,
  bids: ReturnType<typeof useBidsStore>,
  watchlist: ReturnType<typeof useWatchlistStore>,
  compare: ReturnType<typeof useCompareStore>,
): AugmentContext {
  return {
    effectiveNow: clock.effectiveNow,
    override: bids.overrides[vehicleId],
    justBoughtId: bids.justBoughtId,
    isWatched: watchlist.ids.has(vehicleId),
    isCompareSelected: compare.ids.includes(vehicleId),
    compareDisabledAdd: compare.ids.length >= 2 && !compare.ids.includes(vehicleId),
  }
}

/**
 * Reactive list of every vehicle known so far (guidelines/06-backend-architecture.md's
 * client-integration phase): inventoryFilters.vehiclesById accumulates across
 * every inventory page fetched and every single-listing fetch, so watchlist/
 * compare (which need to render a listing regardless of whether it's in the
 * *current* filtered inventory page) keep working without their own fetch
 * logic. Must stay a computed() — it reads the clock store, which is what
 * makes badges/status flip live as time passes.
 */
export function useAugmentedListings() {
  const clock = useClockStore()
  const bids = useBidsStore()
  const watchlist = useWatchlistStore()
  const compare = useCompareStore()
  const filters = useInventoryFiltersStore()

  const list = computed<AugmentedListing[]>(() =>
    Object.values(filters.vehiclesById).map((vehicle) =>
      augment(vehicle, buildContext(vehicle.id, clock, bids, watchlist, compare)),
    ),
  )

  return { list }
}

/**
 * Reactive single augmented listing for a (possibly reactive) id — used by
 * ListingDetailsView, the Preview Modal, and Compare Modal. Triggers a fetch
 * for an id not already known (a direct/bookmarked link to a listing never
 * paginated into view) via inventoryFilters.ensureVehicleLoaded.
 */
export function useAugmentedListing(id: MaybeRefOrGetter<string | undefined>) {
  const clock = useClockStore()
  const bids = useBidsStore()
  const watchlist = useWatchlistStore()
  const compare = useCompareStore()
  const filters = useInventoryFiltersStore()

  watchEffect(() => {
    const vehicleId = toValue(id)
    if (vehicleId && !filters.vehiclesById[vehicleId]) void filters.ensureVehicleLoaded(vehicleId)
  })

  const listing = computed<AugmentedListing | undefined>(() => {
    const vehicleId = toValue(id)
    if (!vehicleId) return undefined
    const vehicle = filters.vehiclesById[vehicleId]
    if (!vehicle) return undefined
    return augment(vehicle, buildContext(vehicle.id, clock, bids, watchlist, compare))
  })

  return { listing }
}
