import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buyNow, fetchBidHistory, fetchFacets, fetchListing, fetchListings, placeBid } from './listings'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

function lastRequestedUrl(): string {
  return vi.mocked(fetch).mock.calls[0]![0] as string
}

describe('listings service', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('fetchListings with no params requests the bare /listings path', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { data: [], page_info: { has_next_page: false, has_previous_page: false } }))

    await fetchListings()

    expect(lastRequestedUrl()).toBe('/v1/listings')
  })

  it('fetchListings encodes every provided param and omits unset ones', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { data: [], page_info: { has_next_page: false, has_previous_page: false } }))

    await fetchListings({ status: 'active', make: 'Mazda', sort: 'price-low', first: 24, after: 'cursor-abc' })

    const url = new URL(lastRequestedUrl(), 'https://example.test')
    expect(url.pathname).toBe('/v1/listings')
    expect(url.searchParams.get('status')).toBe('active')
    expect(url.searchParams.get('make')).toBe('Mazda')
    expect(url.searchParams.get('sort')).toBe('price-low')
    expect(url.searchParams.get('first')).toBe('24')
    expect(url.searchParams.get('after')).toBe('cursor-abc')
    expect(url.searchParams.has('before')).toBe(false)
    expect(url.searchParams.has('q')).toBe(false)
  })

  it('fetchFacets requests /listings/facets', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { makes: ['Mazda', 'Toyota'] }))

    const facets = await fetchFacets()

    expect(lastRequestedUrl()).toBe('/v1/listings/facets')
    expect(facets.makes).toEqual(['Mazda', 'Toyota'])
  })

  it('fetchListing requests /listings/{id}', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { id: 'abc-123' }))

    await fetchListing('abc-123')

    expect(lastRequestedUrl()).toBe('/v1/listings/abc-123')
  })

  it('fetchBidHistory unwraps the data array', async () => {
    const entries = [{ handle: 'Bidder 1', type: 'bid', amount: 21000, bid_count: 1, accepted_at: '2026-01-01T00:00:00Z', is_viewer: false }]
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { data: entries }))

    const result = await fetchBidHistory('abc-123')

    expect(lastRequestedUrl()).toBe('/v1/listings/abc-123/bids')
    expect(result).toEqual(entries)
  })

  it('placeBid posts the amount and unwraps the accepted data', async () => {
    const accepted = { bid_id: 'bid-1', current_bid: 21500, bid_count: 3, accepted_at: '2026-01-01T00:00:00Z', viewer: { has_bid: true, is_high_bidder: true, is_outbid: false } }
    vi.mocked(fetch).mockResolvedValue(jsonResponse(201, { data: accepted }))

    const result = await placeBid('abc-123', 21500)

    expect(lastRequestedUrl()).toBe('/v1/listings/abc-123/bids')
    const [, init] = vi.mocked(fetch).mock.calls[0]!
    expect(init!.body).toBe(JSON.stringify({ amount: 21500 }))
    expect(result).toEqual(accepted)
  })

  it('buyNow posts with no body and unwraps the accepted data', async () => {
    const accepted = { bid_id: 'bid-2', current_bid: 50000, bid_count: 1, accepted_at: '2026-01-01T00:00:00Z', viewer: { has_bid: true, is_high_bidder: true, is_outbid: false } }
    vi.mocked(fetch).mockResolvedValue(jsonResponse(201, { data: accepted }))

    const result = await buyNow('abc-123')

    expect(lastRequestedUrl()).toBe('/v1/listings/abc-123/buy-now')
    const [, init] = vi.mocked(fetch).mock.calls[0]!
    expect(init!.body).toBeUndefined()
    expect(result).toEqual(accepted)
  })
})
