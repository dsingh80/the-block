import { describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { augment, useAugmentedListing } from './useListingPresentation'
import { useClockStore } from '@/stores/clock'
import { vehicles, sellerCounts } from '@/data/vehicles'
import type { Vehicle } from '@/types/vehicle'
import type { BidOverride } from '@/types/listing'

const HOUR_MS = 60 * 60 * 1000
const DEFAULT_AUCTION_START = '2026-01-01T12:00:00.000Z'

function makeVehicle(overrides: Partial<Vehicle> = {}): Vehicle {
  return {
    id: 'test-vehicle',
    vin: 'TESTVIN0000000001',
    year: 2022,
    make: 'Toyota',
    model: 'Camry',
    trim: 'SE',
    body_style: 'sedan',
    exterior_color: 'Black',
    interior_color: 'Black',
    engine: '2.5L I4',
    transmission: 'automatic',
    drivetrain: 'FWD',
    odometer_km: 40000,
    fuel_type: 'gasoline',
    condition_grade: 4.2,
    condition_report: 'Good condition.',
    damage_notes: [],
    title_status: 'clean',
    province: 'Ontario',
    city: 'Toronto',
    auction_start: DEFAULT_AUCTION_START,
    starting_bid: 10000,
    reserve_price: null,
    buy_now_price: null,
    images: ['a.jpg', 'b.jpg', 'c.jpg'],
    selling_dealership: 'Test Motors',
    lot: 'A-0001',
    current_bid: null,
    bid_count: 0,
    ...overrides,
  }
}

function makeOverride(overrides: Partial<BidOverride> = {}): BidOverride {
  return {
    currentPrice: 11000,
    bidCount: 1,
    hasUserBid: true,
    isUserHighBidder: false,
    isUserOutbid: false,
    purchased: false,
    purchasedAt: null,
    ...overrides,
  }
}

type Ctx = Parameters<typeof augment>[1]

function baseCtx(overrides: Partial<Ctx> = {}): Ctx {
  return {
    effectiveNow: new Date(DEFAULT_AUCTION_START).getTime() + HOUR_MS, // 1h into the auction
    override: undefined,
    justBoughtId: null,
    isWatched: false,
    isCompareSelected: false,
    compareDisabledAdd: false,
    ...overrides,
  }
}

describe('augment - price label', () => {
  it('is "Opening Bid" for an upcoming listing', () => {
    const vehicle = makeVehicle()
    const listing = augment(
      vehicle,
      baseCtx({ effectiveNow: new Date(DEFAULT_AUCTION_START).getTime() - HOUR_MS }),
    )
    expect(listing.priceLabelText).toBe('Opening Bid')
  })

  it('is "Starting Bid" for an active listing with no bids yet', () => {
    const vehicle = makeVehicle({ current_bid: null, bid_count: 0 })
    const listing = augment(vehicle, baseCtx())
    expect(listing.priceLabelText).toBe('Starting Bid')
    expect(listing.priceValue).toBe(vehicle.starting_bid)
  })

  it('is "Current Bid" for an active listing with a bid already placed', () => {
    const vehicle = makeVehicle({ current_bid: 12000, bid_count: 4 })
    const listing = augment(vehicle, baseCtx())
    expect(listing.priceLabelText).toBe('Current Bid')
    expect(listing.priceValue).toBe(12000)
  })

  it('is "Winning Bid" once the listing has ended', () => {
    const vehicle = makeVehicle({ current_bid: 12000, bid_count: 4 })
    const endedNow = new Date(DEFAULT_AUCTION_START).getTime() + 25 * HOUR_MS
    const listing = augment(vehicle, baseCtx({ effectiveNow: endedNow }))
    expect(listing.priceLabelText).toBe('Winning Bid')
  })
})

describe('augment - badge', () => {
  it('shows no badge without a user bid', () => {
    const listing = augment(makeVehicle(), baseCtx())
    expect(listing.badgeVariant).toBeNull()
  })

  it('shows Winning when active and the user is high bidder', () => {
    const override = makeOverride({ isUserHighBidder: true })
    const listing = augment(makeVehicle(), baseCtx({ override }))
    expect(listing.badgeVariant).toBe('winning')
  })

  it('shows Outbid when active and the user has been outbid', () => {
    const override = makeOverride({ isUserOutbid: true })
    const listing = augment(makeVehicle(), baseCtx({ override }))
    expect(listing.badgeVariant).toBe('outbid')
  })

  it('shows Bidding when active with a bid, neither winning nor outbid', () => {
    const override = makeOverride()
    const listing = augment(makeVehicle(), baseCtx({ override }))
    expect(listing.badgeVariant).toBe('bidding')
  })

  it('shows Won once ended if the user was high bidder', () => {
    const endedNow = new Date(DEFAULT_AUCTION_START).getTime() + 25 * HOUR_MS
    const override = makeOverride({ isUserHighBidder: true })
    const listing = augment(makeVehicle(), baseCtx({ effectiveNow: endedNow, override }))
    expect(listing.badgeVariant).toBe('won')
  })

  it('shows Lost once ended if the user bid but was not high bidder', () => {
    const endedNow = new Date(DEFAULT_AUCTION_START).getTime() + 25 * HOUR_MS
    const override = makeOverride()
    const listing = augment(makeVehicle(), baseCtx({ effectiveNow: endedNow, override }))
    expect(listing.badgeVariant).toBe('lost')
  })
})

describe('augment - damage notes', () => {
  it('falls back to a no-damage sentence when damage_notes is empty', () => {
    const listing = augment(makeVehicle({ damage_notes: [] }), baseCtx())
    expect(listing.damageList).toEqual(['No reported damage.'])
  })

  it('passes real damage notes through unchanged', () => {
    const vehicle = makeVehicle({ damage_notes: ['Scratch on hood', 'Worn tires'] })
    const listing = augment(vehicle, baseCtx())
    expect(listing.damageList).toEqual(['Scratch on hood', 'Worn tires'])
  })
})

describe('augment - seller listing count (against the real dataset)', () => {
  it('is 0 for a dealership with exactly one listing', () => {
    const [dealership] = [...sellerCounts.entries()].find(([, count]) => count === 1)!
    const vehicle = vehicles.find((candidate) => candidate.selling_dealership === dealership)!
    const listing = augment(
      vehicle,
      baseCtx({ effectiveNow: new Date(vehicle.auction_start).getTime() + HOUR_MS }),
    )
    expect(listing.sellerOtherListingsCount).toBe(0)
  })

  it('is count-minus-one for a dealership with multiple listings', () => {
    const [dealership, count] = [...sellerCounts.entries()].find(([, c]) => c > 1)!
    const vehicle = vehicles.find((candidate) => candidate.selling_dealership === dealership)!
    const listing = augment(
      vehicle,
      baseCtx({ effectiveNow: new Date(vehicle.auction_start).getTime() + HOUR_MS }),
    )
    expect(listing.sellerOtherListingsCount).toBe(count - 1)
  })
})

describe('useAugmentedListing reactivity', () => {
  it('flips lifecycle live as the clock store advances past the end boundary', () => {
    setActivePinia(createPinia())
    const vehicle = vehicles[0]
    const clock = useClockStore()
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS

    const { listing } = useAugmentedListing(() => vehicle.id)
    expect(listing.value?.lifecycle).toBe('active')

    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + 25 * HOUR_MS
    expect(listing.value?.lifecycle).toBe('ended')
  })
})
