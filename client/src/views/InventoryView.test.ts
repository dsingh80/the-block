import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import InventoryView from './InventoryView.vue'
import * as listingsApi from '@/services/api/listings'
import { PAGE_SIZE, SCROLL_SYNC_DEBOUNCE_MS } from '@/stores/inventoryFilters'
import type { ApiListingSummary, ApiListingsPage } from '@/services/api/types'

vi.mock('@/services/api/listings')

function emptyPage() {
  return { data: [], page_info: { has_next_page: false, has_previous_page: false } }
}

function apiListing(overrides: Partial<ApiListingSummary> = {}): ApiListingSummary {
  return {
    id: 'listing-1',
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
    current_bid: null,
    bid_count: 0,
    purchased_at: null,
    viewer: { has_bid: false, is_high_bidder: false, is_outbid: false },
    ...overrides,
  }
}

/** jsdom has no IntersectionObserver at all -- this stands in for it, and lets a test drive the sentinel by invoking the captured callback directly instead of needing a real observed intersection. */
class FakeIntersectionObserver {
  static instances: FakeIntersectionObserver[] = []
  readonly callback: IntersectionObserverCallback
  observe = vi.fn()
  unobserve = vi.fn()
  disconnect = vi.fn()

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    FakeIntersectionObserver.instances.push(this)
  }

  intersect() {
    this.callback([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
}

async function mountAt(initialUrl: string, options: { attachTo?: Element } = {}) {
  setActivePinia(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/inventory', name: 'inventory', component: InventoryView }],
  })
  await router.push(initialUrl)
  await router.isReady()

  const wrapper = mount(InventoryView, { global: { plugins: [router] }, ...options })
  await flushPromises()
  return { wrapper, router }
}

/**
 * Mounts at a URL whose initial reset() resolves with a previous page --
 * that kicks off the settle-scroll sequence in onMounted (scrollToResults
 * then waitForScrollSettle), which is still pending when flushPromises()
 * alone returns since it only flushes microtasks, not waitForScrollSettle's
 * real setTimeout fallback. Fake timers + advancing past that 1000ms
 * fallback lets the sequence -- and the top observer's own setup, which
 * comes after it -- actually finish before the test proceeds.
 */
async function mountResumedAt(url: string, firstPage: ApiListingsPage) {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce(firstPage)
  const result = await mountAt(url)
  await vi.advanceTimersByTimeAsync(1000)
  return result
}

describe('InventoryView URL sync', () => {
  beforeEach(() => {
    vi.mocked(listingsApi.fetchListings).mockReset().mockResolvedValue(emptyPage())
    vi.mocked(listingsApi.fetchFacets).mockReset().mockResolvedValue({ makes: [] })
    FakeIntersectionObserver.instances = []
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('seeds every filter from the URL on mount and fetches accordingly', async () => {
    await mountAt('/inventory?status=active&make=Mazda&q=turbo&sort=price-low')

    expect(listingsApi.fetchListings).toHaveBeenCalledWith({
      status: 'active',
      make: 'Mazda',
      q: 'turbo',
      sort: 'price-low',
      first: 24,
      after: undefined,
    })
  })

  it("passes the URL's after cursor as the initial resume checkpoint", async () => {
    await mountAt('/inventory?after=some-cursor')

    expect(listingsApi.fetchListings).toHaveBeenCalledWith(expect.objectContaining({ after: 'some-cursor' }))
  })

  it('defaults to no filters and the "ending" sort when the URL has none', async () => {
    await mountAt('/inventory')

    expect(listingsApi.fetchListings).toHaveBeenCalledWith({
      status: undefined,
      make: undefined,
      q: undefined,
      sort: 'ending',
      first: 24,
      after: undefined,
    })
  })

  it('updates the URL and refetches immediately when a status chip is clicked', async () => {
    const { wrapper, router } = await mountAt('/inventory')
    vi.mocked(listingsApi.fetchListings).mockClear()

    const activeChip = wrapper.findAll('.filter-bar__chip').find((chip) => chip.text() === 'Active')
    await activeChip!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.query.status).toBe('active')
    expect(listingsApi.fetchListings).toHaveBeenCalledWith(expect.objectContaining({ status: 'active' }))
  })

  it('does not touch the URL for the "all" status (the default is simply absent, not written as a literal value)', async () => {
    const { wrapper, router } = await mountAt('/inventory?status=active')
    vi.mocked(listingsApi.fetchListings).mockClear()

    const allChip = wrapper.findAll('.filter-bar__chip').find((chip) => chip.text() === 'All')
    await allChip!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.query.status).toBeUndefined()
  })

  it('debounces the URL update and refetch for a search change', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const { wrapper, router } = await mountAt('/inventory')
    vi.mocked(listingsApi.fetchListings).mockClear()

    await wrapper.find('#inventory-search').setValue('turbo')
    expect(listingsApi.fetchListings).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()

    expect(router.currentRoute.value.query.q).toBe('turbo')
    expect(listingsApi.fetchListings).toHaveBeenCalledWith(expect.objectContaining({ q: 'turbo' }))
  })
})

describe('InventoryView infinite-scroll sentinel', () => {
  beforeEach(() => {
    vi.mocked(listingsApi.fetchFacets).mockReset().mockResolvedValue({ makes: [] })
    FakeIntersectionObserver.instances = []
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('observes both the bottom and top sentinel elements on mount', async () => {
    vi.mocked(listingsApi.fetchListings).mockResolvedValue(emptyPage())
    await mountAt('/inventory')

    expect(FakeIntersectionObserver.instances).toHaveLength(2)
    expect(FakeIntersectionObserver.instances[0]!.observe).toHaveBeenCalledTimes(1)
    expect(FakeIntersectionObserver.instances[1]!.observe).toHaveBeenCalledTimes(1)
  })

  it('calls loadNextPage when the sentinel intersects, and reflects the new cursor in the URL', async () => {
    vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
      data: [],
      page_info: { has_next_page: true, has_previous_page: false, end_cursor: 'page-1-cursor' },
    })
    const { router } = await mountAt('/inventory')

    vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
      data: [],
      page_info: { has_next_page: false, has_previous_page: true, end_cursor: 'page-2-cursor' },
    })
    FakeIntersectionObserver.instances[0]!.intersect()
    await flushPromises()

    expect(listingsApi.fetchListings).toHaveBeenLastCalledWith(expect.objectContaining({ after: 'page-1-cursor' }))
    expect(router.currentRoute.value.query.after).toBe('page-2-cursor')
  })

  it('disconnects both observers on unmount', async () => {
    vi.mocked(listingsApi.fetchListings).mockResolvedValue(emptyPage())
    const { wrapper } = await mountAt('/inventory')

    wrapper.unmount()

    expect(FakeIntersectionObserver.instances[0]!.disconnect).toHaveBeenCalledTimes(1)
    expect(FakeIntersectionObserver.instances[1]!.disconnect).toHaveBeenCalledTimes(1)
  })
})

describe('InventoryView backward pagination', () => {
  let scrollIntoView: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.mocked(listingsApi.fetchFacets).mockReset().mockResolvedValue({ makes: [] })
    FakeIntersectionObserver.instances = []
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
    scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView as unknown as typeof Element.prototype.scrollIntoView
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('renders no skeleton block when there is no previous page', async () => {
    vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
      data: [apiListing({ id: 'a' })],
      page_info: { has_next_page: false, has_previous_page: false },
    })

    const { wrapper } = await mountAt('/inventory')

    expect(wrapper.find('.inventory-view__grid--skeleton').exists()).toBe(false)
  })

  it('renders exactly PAGE_SIZE skeleton placeholders when resuming mid-list', async () => {
    const { wrapper } = await mountResumedAt('/inventory?after=some-cursor', {
      data: [apiListing({ id: 'a' })],
      page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-a' },
    })

    expect(wrapper.findAll('.vehicle-card-skeleton')).toHaveLength(PAGE_SIZE)
  })

  it('completes the settle-scroll sequence and only then observes the top sentinel', async () => {
    await mountResumedAt('/inventory?after=some-cursor', {
      data: [apiListing({ id: 'a' })],
      page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-a' },
    })

    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    expect(FakeIntersectionObserver.instances[1]!.observe).toHaveBeenCalledTimes(1)
  })

  it('loading an earlier page via the top sentinel prepends results and keeps the skeleton if more remain', async () => {
    const { wrapper } = await mountResumedAt('/inventory?after=some-cursor', {
      data: [apiListing({ id: 'b' })],
      page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-b' },
    })

    vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
      data: [apiListing({ id: 'a' })],
      page_info: { has_next_page: true, has_previous_page: true, start_cursor: 'cursor-a' },
    })
    FakeIntersectionObserver.instances[1]!.intersect()
    await flushPromises()

    expect(listingsApi.fetchListings).toHaveBeenLastCalledWith(expect.objectContaining({ before: 'cursor-b' }))
    expect(wrapper.findAll('.vehicle-card')).toHaveLength(2)
    expect(wrapper.find('.inventory-view__grid--skeleton').exists()).toBe(true)
  })

  it('clears the skeleton block once a backward load reports there is nothing earlier left', async () => {
    const { wrapper } = await mountResumedAt('/inventory?after=some-cursor', {
      data: [apiListing({ id: 'a' })],
      page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-a' },
    })

    vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
      data: [],
      page_info: { has_next_page: true, has_previous_page: false },
    })
    FakeIntersectionObserver.instances[1]!.intersect()
    await flushPromises()

    expect(wrapper.find('.inventory-view__grid--skeleton').exists()).toBe(false)
  })

  it("writes the backward load's new start cursor to the URL", async () => {
    const { router } = await mountResumedAt('/inventory?after=some-cursor', {
      data: [apiListing({ id: 'b' })],
      page_info: { has_next_page: false, has_previous_page: true, start_cursor: 'cursor-b' },
    })

    vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
      data: [apiListing({ id: 'a' })],
      page_info: { has_next_page: true, has_previous_page: false, start_cursor: 'cursor-a' },
    })
    FakeIntersectionObserver.instances[1]!.intersect()
    await flushPromises()

    expect(router.currentRoute.value.query.after).toBe('cursor-a')
  })

  it('does not fetch when the top sentinel intersects and there is no previous page -- proves the settle-then-observe redesign actually closes the race, not just usually works', async () => {
    vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
      data: [apiListing({ id: 'a' })],
      page_info: { has_next_page: false, has_previous_page: false },
    })
    await mountAt('/inventory')
    vi.mocked(listingsApi.fetchListings).mockClear()

    FakeIntersectionObserver.instances[1]!.intersect()
    await flushPromises()

    expect(listingsApi.fetchListings).not.toHaveBeenCalled()
  })

  it('lazily syncs the URL to whichever loaded checkpoint is nearest the top once scrolling stops', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    try {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: true, has_previous_page: false, end_cursor: 'cursor-a' },
      })
      const { router } = await mountAt('/inventory', { attachTo: container })

      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'b' })],
        page_info: { has_next_page: false, has_previous_page: true },
      })
      FakeIntersectionObserver.instances[0]!.intersect()
      await flushPromises()

      // checkpoints are now [{id: 'a', cursor: undefined}, {id: 'b', cursor: 'cursor-a'}]
      const elA = document.querySelector<HTMLElement>('[data-listing-id="a"]')!
      const elB = document.querySelector<HTMLElement>('[data-listing-id="b"]')!
      vi.spyOn(elA, 'getBoundingClientRect').mockReturnValue({ top: 500 } as unknown as DOMRect)
      vi.spyOn(elB, 'getBoundingClientRect').mockReturnValue({ top: 10 } as unknown as DOMRect)

      vi.useFakeTimers({ shouldAdvanceTime: true })
      window.dispatchEvent(new Event('scroll'))
      await vi.advanceTimersByTimeAsync(SCROLL_SYNC_DEBOUNCE_MS)

      expect(router.currentRoute.value.query.after).toBe('cursor-a')
    } finally {
      document.body.removeChild(container)
    }
  })

  it('does not sync the URL from scroll position when nothing has scrolled past the header line', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    try {
      vi.mocked(listingsApi.fetchListings).mockResolvedValueOnce({
        data: [apiListing({ id: 'a' })],
        page_info: { has_next_page: false, has_previous_page: false },
      })
      const { router } = await mountAt('/inventory', { attachTo: container })
      const before = router.currentRoute.value.query.after

      const elA = document.querySelector<HTMLElement>('[data-listing-id="a"]')!
      vi.spyOn(elA, 'getBoundingClientRect').mockReturnValue({ top: 500 } as unknown as DOMRect)

      vi.useFakeTimers({ shouldAdvanceTime: true })
      window.dispatchEvent(new Event('scroll'))
      await vi.advanceTimersByTimeAsync(SCROLL_SYNC_DEBOUNCE_MS)

      expect(router.currentRoute.value.query.after).toBe(before)
    } finally {
      document.body.removeChild(container)
    }
  })
})
