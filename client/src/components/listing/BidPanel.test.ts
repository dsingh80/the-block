import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import BidPanel from './BidPanel.vue'
import { useClockStore } from '@/stores/clock'
import { useInventoryFiltersStore } from '@/stores/inventoryFilters'
import { augment } from '@/composables/useListingPresentation'
import { getBidIncrement } from '@/utils/bidding'
import { BID_UPDATE_HIGHLIGHT_MS } from '@/utils/constants'
import * as listingsApi from '@/services/api/listings'
import type { Vehicle } from '@/types/vehicle'

const HOUR_MS = 60 * 60 * 1000

vi.mock('@/services/api/listings')

function vehicleFixture(overrides: Partial<Vehicle> = {}): Vehicle {
  return {
    id: 'vehicle-1',
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
    auction_start: '2026-01-01T12:00:00Z',
    auction_end: '2026-01-02T12:00:00Z',
    status: 'active',
    starting_bid: 10000,
    buy_now_price: null,
    images: [],
    selling_dealership: 'Test Motors',
    lot: 'A-0001',
    current_bid: 20000,
    bid_count: 5,
    purchased_at: null,
    ...overrides,
  }
}

function mountActive(overrides: { isUserHighBidder?: boolean } = {}) {
  // stubActions: false so placeBid runs its real logic — this component's
  // whole job is to be a thin wrapper around that real validation gate. The
  // network call underneath it is mocked instead (@/services/api/listings),
  // not the store action itself.
  const pinia = createTestingPinia({ stubActions: false, createSpy: vi.fn })
  const vehicle = vehicleFixture()
  // bids.ts reads a vehicle's own price/lifecycle from the live inventory
  // cache, not a static import -- seed it the same way a real fetched page
  // (or ensureVehicleLoaded) would.
  useInventoryFiltersStore().vehiclesById[vehicle.id] = vehicle
  const clock = useClockStore()
  clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS

  const listing = augment(vehicle, {
    effectiveNow: clock.effectiveNow,
    override: overrides.isUserHighBidder
      ? {
          currentPrice: vehicle.current_bid ?? vehicle.starting_bid,
          bidCount: vehicle.bid_count,
          hasUserBid: true,
          isUserHighBidder: true,
          isUserOutbid: false,
          purchased: false,
          purchasedAt: null,
        }
      : undefined,
    justBoughtId: null,
    isWatched: false,
    isCompareSelected: false,
    compareDisabledAdd: false,
  })

  const wrapper = mount(BidPanel, {
    props: { listing },
    global: { plugins: [pinia] },
  })

  return { wrapper, vehicle, clock }
}

describe('BidPanel', () => {
  beforeEach(() => {
    vi.mocked(listingsApi.placeBid).mockReset()
  })

  it('rejects a bid below the tiered minimum with an inline error, no alert()', async () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const tooLow = current + getBidIncrement(current) - 1

    await wrapper.find('#bid-amount').setValue(String(tooLow))
    await wrapper.find('.bid-panel__submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('accepts a bid at exactly the tiered minimum and shows inline success', async () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)
    vi.mocked(listingsApi.placeBid).mockResolvedValue({
      bid_id: 'bid-1',
      current_bid: minimum,
      bid_count: (vehicle.bid_count ?? 0) + 1,
      accepted_at: '2026-01-01T00:00:00Z',
      viewer: { has_bid: true, is_high_bidder: true, is_outbid: false },
    })

    await wrapper.find('#bid-amount').setValue(String(minimum))
    await wrapper.find('.bid-panel__submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="status"]').exists()).toBe(true)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('tolerates a $ and commas typed into the input', async () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)
    const formatted = '$' + minimum.toLocaleString('en-US')
    vi.mocked(listingsApi.placeBid).mockResolvedValue({
      bid_id: 'bid-1',
      current_bid: minimum,
      bid_count: (vehicle.bid_count ?? 0) + 1,
      accepted_at: '2026-01-01T00:00:00Z',
      viewer: { has_bid: true, is_high_bidder: true, is_outbid: false },
    })

    await wrapper.find('#bid-amount').setValue(formatted)
    await wrapper.find('.bid-panel__submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="status"]').exists()).toBe(true)
  })

  it('rejects an empty submission without calling the store', async () => {
    const { wrapper } = mountActive()

    // The field is prepopulated with the tiered minimum by default -- clear
    // it to exercise the empty-input guard a user hits after deleting it.
    await wrapper.find('#bid-amount').setValue('')
    await wrapper.find('.bid-panel__submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('rejects a bid once the listing is no longer active, even though the store re-checks live', async () => {
    const { wrapper, vehicle, clock } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid

    // The listing prop was snapshotted while active; the clock moves past
    // the end boundary after mount. placeBid must re-derive lifecycle from
    // the live clock, not trust the stale prop — this is the guardrail
    // from guidelines/03-guardrails.md, exercised end-to-end here.
    clock.effectiveNow = new Date(vehicle.auction_start).getTime() + 48 * HOUR_MS

    await wrapper.find('#bid-amount').setValue(String(current + 1_000_000))
    await wrapper.find('.bid-panel__submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('shows the server-rejected error inline when the API rejects an otherwise valid-looking bid', async () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)
    const { ApiError } = await import('@/services/api/client')
    vi.mocked(listingsApi.placeBid).mockRejectedValue(
      new ApiError(409, 'bid_too_low', 'Your bid is below the current minimum.'),
    )

    await wrapper.find('#bid-amount').setValue(String(minimum))
    await wrapper.find('.bid-panel__submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').text()).toBe('Your bid is below the current minimum.')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })

  it('prepopulates the bid amount with the tiered minimum instead of starting empty', () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)

    expect((wrapper.find('#bid-amount').element as HTMLInputElement).value).toBe(String(minimum))
  })

  it('still shows the bid form, plus a winning note, when the user is already the high bidder', () => {
    const { wrapper } = mountActive({ isUserHighBidder: true })

    expect(wrapper.find('#bid-amount').exists()).toBe(true)
    expect(wrapper.find('.bid-panel__submit').exists()).toBe(true)
    expect(wrapper.find('.bid-panel__winning').text()).toBe(
      'You have the highest bid — you can still raise it below.',
    )
  })

  it('asks for confirmation instead of bidding immediately when the user is already the high bidder', async () => {
    const { wrapper } = mountActive({ isUserHighBidder: true })

    await wrapper.find('.bid-panel__submit').trigger('click')
    await flushPromises()

    expect(wrapper.find('.bid-panel__confirm').exists()).toBe(true)
    expect(wrapper.find('[role="dialog"]').text()).toContain(
      'You are already the winning bidder. Would you still like to raise your bid?',
    )
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('places the raised bid once the confirmation is accepted', async () => {
    const { wrapper, vehicle } = mountActive({ isUserHighBidder: true })
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)
    vi.mocked(listingsApi.placeBid).mockResolvedValue({
      bid_id: 'bid-1',
      current_bid: minimum,
      bid_count: (vehicle.bid_count ?? 0) + 1,
      accepted_at: '2026-01-01T00:00:00Z',
      viewer: { has_bid: true, is_high_bidder: true, is_outbid: false },
    })

    await wrapper.find('.bid-panel__submit').trigger('click')
    await wrapper.find('.bid-panel__confirm-yes').trigger('click')
    await flushPromises()

    expect(listingsApi.placeBid).toHaveBeenCalledWith(vehicle.id, minimum)
    expect(wrapper.find('.bid-panel__confirm').exists()).toBe(false)
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
  })

  it('cancels without bidding when the confirmation is declined', async () => {
    const { wrapper } = mountActive({ isUserHighBidder: true })

    await wrapper.find('.bid-panel__submit').trigger('click')
    await wrapper.find('.bid-panel__confirm-no').trigger('click')
    await flushPromises()

    expect(wrapper.find('.bid-panel__confirm').exists()).toBe(false)
    expect(listingsApi.placeBid).not.toHaveBeenCalled()
  })

  it('flashes a "New bid" indicator when the current bid changes, then clears it after the highlight window', async () => {
    vi.useFakeTimers()
    const { wrapper, vehicle, clock } = mountActive()

    expect(wrapper.find('.bid-panel__update-badge').exists()).toBe(false)

    const current = vehicle.current_bid ?? vehicle.starting_bid
    const bumped = augment(vehicle, {
      effectiveNow: clock.effectiveNow,
      override: {
        currentPrice: current + getBidIncrement(current),
        bidCount: (vehicle.bid_count ?? 0) + 1,
        hasUserBid: false,
        isUserHighBidder: false,
        isUserOutbid: false,
        purchased: false,
        purchasedAt: null,
      },
      justBoughtId: null,
      isWatched: false,
      isCompareSelected: false,
      compareDisabledAdd: false,
    })
    await wrapper.setProps({ listing: bumped })

    expect(wrapper.find('.bid-panel__update-badge').exists()).toBe(true)

    vi.advanceTimersByTime(BID_UPDATE_HIGHLIGHT_MS)
    await nextTick()
    expect(wrapper.find('.bid-panel__update-badge').exists()).toBe(false)

    vi.useRealTimers()
  })

  it('does not flash on initial mount, only on a later change', () => {
    const { wrapper } = mountActive()
    expect(wrapper.find('.bid-panel__update-badge').exists()).toBe(false)
  })

  it('shows a live "time since" label next to the bid count that advances with the clock', async () => {
    const { wrapper, clock } = mountActive()

    expect(wrapper.find('.bid-panel__last-bid').text()).toContain('just now')

    clock.effectiveNow += 5 * 60 * 1000
    await nextTick()

    expect(wrapper.find('.bid-panel__last-bid').text()).toContain('5m ago')
  })
})
