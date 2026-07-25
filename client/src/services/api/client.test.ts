import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost, ApiError } from './client'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('apiGet/apiPost', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('resolves with the parsed JSON body on a 2xx response', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { hello: 'world' }))

    const result = await apiGet<{ hello: string }>('/listings')

    expect(result).toEqual({ hello: 'world' })
  })

  it('requests a path relative to /v1, never an absolute origin', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, {}))

    await apiGet('/listings/facets')

    expect(fetch).toHaveBeenCalledWith('/v1/listings/facets', expect.anything())
  })

  it('always sends the CSRF header, even on a GET', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(200, {}))

    await apiGet('/listings')

    const [, init] = vi.mocked(fetch).mock.calls[0]!
    const headers = init!.headers as Record<string, string>
    expect(headers['X-Requested-With']).toBe('XHR')
  })

  it('POST sends a JSON body and Content-Type header', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(201, { data: { bid_id: 'abc' } }))

    await apiPost('/listings/1/bids', { amount: 21500 })

    const [, init] = vi.mocked(fetch).mock.calls[0]!
    expect(init!.method).toBe('POST')
    expect(init!.body).toBe(JSON.stringify({ amount: 21500 }))
    const headers = init!.headers as Record<string, string>
    expect(headers['Content-Type']).toBe('application/json')
  })

  it('POST with no body omits Content-Type and sends no body', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(201, {}))

    await apiPost('/listings/1/buy-now')

    const [, init] = vi.mocked(fetch).mock.calls[0]!
    expect(init!.body).toBeUndefined()
    const headers = init!.headers as Record<string, string>
    expect(headers['Content-Type']).toBeUndefined()
  })

  it('throws an ApiError with the envelope fields on a non-2xx response', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse(409, {
        error: { code: 'bid_too_low', message: 'Your bid is below the current minimum.', details: { minimum: 21500 }, request_id: 'req-1' },
      }),
    )

    await expect(apiGet('/listings/1')).rejects.toMatchObject({
      name: 'ApiError',
      status: 409,
      code: 'bid_too_low',
      message: 'Your bid is below the current minimum.',
      details: { minimum: 21500 },
    })
  })

  it('a non-2xx response with an unparseable body still throws a usable ApiError', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('not json', { status: 500 }))

    await expect(apiGet('/listings')).rejects.toBeInstanceOf(ApiError)
  })
})
