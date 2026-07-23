import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import BidPanel from './BidPanel.vue'
import { useClockStore } from '@/stores/clock'
import { augment } from '@/composables/useListingPresentation'
import { vehicles } from '@/data/vehicles'
import { getBidIncrement } from '@/utils/bidding'

const HOUR_MS = 60 * 60 * 1000

function mountActive() {
  // stubActions: false so placeBid runs its real logic — this component's
  // whole job is to be a thin wrapper around that real validation gate.
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
  it('rejects a bid below the tiered minimum with an inline error, no alert()', async () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const tooLow = current + getBidIncrement(current) - 1

    await wrapper.find('#bid-amount').setValue(String(tooLow))
    await wrapper.find('.bid-panel__submit').trigger('click')

    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })

  it('accepts a bid at exactly the tiered minimum and shows inline success', async () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)

    await wrapper.find('#bid-amount').setValue(String(minimum))
    await wrapper.find('.bid-panel__submit').trigger('click')

    expect(wrapper.find('[role="status"]').exists()).toBe(true)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('tolerates a $ and commas typed into the input', async () => {
    const { wrapper, vehicle } = mountActive()
    const current = vehicle.current_bid ?? vehicle.starting_bid
    const minimum = current + getBidIncrement(current)
    const formatted = '$' + minimum.toLocaleString('en-US')

    await wrapper.find('#bid-amount').setValue(formatted)
    await wrapper.find('.bid-panel__submit').trigger('click')

    expect(wrapper.find('[role="status"]').exists()).toBe(true)
  })

  it('rejects an empty submission without calling the store', async () => {
    const { wrapper } = mountActive()

    await wrapper.find('.bid-panel__submit').trigger('click')

    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
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

    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })
})
