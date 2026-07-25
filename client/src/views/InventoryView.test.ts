import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import InventoryView from './InventoryView.vue'
import * as listingsApi from '@/services/api/listings'

vi.mock('@/services/api/listings')

function emptyPage() {
  return { data: [], page_info: { has_next_page: false, has_previous_page: false } }
}

async function mountAt(initialUrl: string) {
  setActivePinia(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/inventory', name: 'inventory', component: InventoryView }],
  })
  await router.push(initialUrl)
  await router.isReady()

  const wrapper = mount(InventoryView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('InventoryView URL sync', () => {
  beforeEach(() => {
    vi.mocked(listingsApi.fetchListings).mockReset().mockResolvedValue(emptyPage())
    vi.mocked(listingsApi.fetchFacets).mockReset().mockResolvedValue({ makes: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
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
