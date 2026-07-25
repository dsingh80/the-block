import { apiGet, apiPost } from './client'
import type { ApiBidAccept, ApiBidHistoryEntry, ApiFacets, ApiListingsPage, ApiListingSummary } from './types'

export interface FetchListingsParams {
  status?: 'upcoming' | 'active' | 'ended'
  make?: string
  q?: string
  sort?: 'ending' | 'price-low' | 'price-high' | 'year'
  first?: number
  after?: string
  last?: number
  before?: string
}

function toQueryString(params: FetchListingsParams): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

export function fetchListings(params: FetchListingsParams = {}): Promise<ApiListingsPage> {
  return apiGet<ApiListingsPage>(`/listings${toQueryString(params)}`)
}

export function fetchFacets(): Promise<ApiFacets> {
  return apiGet<ApiFacets>('/listings/facets')
}

export function fetchListing(id: string): Promise<ApiListingSummary> {
  return apiGet<ApiListingSummary>(`/listings/${id}`)
}

export async function fetchBidHistory(id: string): Promise<ApiBidHistoryEntry[]> {
  const page = await apiGet<{ data: ApiBidHistoryEntry[] }>(`/listings/${id}/bids`)
  return page.data
}

/**
 * No idempotency-key parameter -- the server derives one from the request
 * itself (session, listing id, amount), so there's nothing for the client to
 * generate or track (guidelines/06-backend-architecture.md, "Idempotency").
 */
export async function placeBid(id: string, amount: number): Promise<ApiBidAccept> {
  const response = await apiPost<{ data: ApiBidAccept }>(`/listings/${id}/bids`, { amount })
  return response.data
}

export async function buyNow(id: string): Promise<ApiBidAccept> {
  const response = await apiPost<{ data: ApiBidAccept }>(`/listings/${id}/buy-now`)
  return response.data
}
