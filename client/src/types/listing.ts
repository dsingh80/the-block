import type { TitleStatus, Vehicle } from './vehicle'

export type Lifecycle = 'upcoming' | 'active' | 'ended'
export type GradeVariant = 'good' | 'fair' | 'poor'
export type BadgeVariant = 'winning' | 'outbid' | 'bidding' | 'won' | 'lost' | null
export type CtaVariant = 'primary' | 'outline-navy' | 'outline-accent'
export type TimeVariant = 'urgent' | 'upcoming' | 'default'
export type BidStatusVariant = 'accent' | 'success' | 'muted' | 'danger'

export type PriceLabel = 'Opening Bid' | 'Starting Bid' | 'Current Bid' | 'Winning Bid'

/**
 * The user's session-local relationship to one listing's bidding state.
 * Lives in stores/bids.ts, keyed by vehicle id. Absence of an entry means
 * "no user interaction yet" — augment() falls back to the vehicle's own
 * current_bid/starting_bid/bid_count in that case.
 */
export interface BidOverride {
  currentPrice: number
  bidCount: number
  hasUserBid: boolean
  isUserHighBidder: boolean
  isUserOutbid: boolean
  purchased: boolean
  purchasedAt: number | null
}

/**
 * The single presentation model every component reads from. Produced by
 * composables/useListingPresentation.ts's augment() — never constructed or
 * re-derived anywhere else. See guidelines/02-design-patterns.md #2.
 */
export interface AugmentedListing {
  id: string
  vehicle: Vehicle

  lifecycle: Lifecycle
  isUpcoming: boolean
  isEnded: boolean
  canBid: boolean

  mileageLabel: string
  locationLabel: string

  gradeVariant: GradeVariant
  gradeLabel: string
  gradeTooltip: string

  titleVariant: TitleStatus
  titleLabel: string

  priceValue: number
  priceFormatted: string
  priceLabelText: PriceLabel
  bidCountLabel: string
  nextBidValue: number
  nextBidFormatted: string

  hoursRemaining: number
  timeLabel: string
  timeVariant: TimeVariant

  hasUserBid: boolean
  isUserHighBidder: boolean
  isUserOutbid: boolean

  badgeVariant: BadgeVariant
  badgeText: string

  bidStatusText: string
  bidStatusVariant: BidStatusVariant

  showBuyNow: boolean
  buyNowFormatted: string | null
  justBought: boolean

  isWatched: boolean
  watchButtonLabel: string

  isCompareSelected: boolean
  compareDisabledAdd: boolean

  ctaLabel: string
  ctaVariant: CtaVariant

  images: string[]
  damageList: string[]
  sellerOtherListingsCount: number
}
