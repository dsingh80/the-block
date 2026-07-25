import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import BidPanel from './BidPanel.vue'
import { useClockStore } from '@/stores/clock'
import { augment } from '@/composables/useListingPresentation'
import { vehicles } from '@/data/vehicles'
import { getBidIncrement } from '@/utils/bidding'
import * as listingsApi from '@/services/api/listings'

const HOUR_MS = 60 * 60 * 1000

vi.mock('@/services/api/listings')

function mountActive() {
  // stubActions: false so placeBid runs its real logic — this component's
  // whole job is to be a thin wrapper around that real validation gate. The
  // network call underneath it is mocked instead (@/services/api/listings),
  // not the store action itself.
  const pinia = createTestingPinia({ stubActions: false, createSpy: vi.fn })
  const vehicle = vehicles[0]
  const clock = useClockStore()
  clock.effectiveNow = new Date(vehicle.auction_start).getTime() + HOUR_MS

  const listing = augment(vehicle, {
    effectiveNow: clock.effectiveNow,
    override: undefined,
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
})
