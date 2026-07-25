import type { ApiErrorBody } from './types'

/**
 * Thin fetch wrapper -- the one seam components/stores call through instead
 * of `fetch` directly (guidelines/02-design-patterns.md #4). Base path is
 * relative (`/v1/...`), not an absolute origin: Caddy proxies the client and
 * the API under one origin, so a relative path always resolves correctly
 * with no CORS config, and the browser's default `credentials: 'same-origin'`
 * already attaches the session cookie -- no `credentials: 'include'`
 * special-casing needed (guidelines/06-backend-architecture.md, "Deployment & HTTPS").
 */
const BASE_PATH = '/v1'

/**
 * The custom-header half of the CSRF defense (guidelines/06-backend-architecture.md,
 * "Sessions & security") -- must match middleware.CSRFHeaderName/Value on the
 * server exactly. Sent on every request, not just state-changing ones: simpler
 * than conditioning it on method, and harmless on a GET (the server only ever
 * checks it for POST/PUT/PATCH/DELETE).
 */
const CSRF_HEADER = { 'X-Requested-With': 'XHR' } as const

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details?: Record<string, unknown>

  constructor(status: number, code: string, message: string, details?: Record<string, unknown>) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${BASE_PATH}${path}`, {
    ...init,
    headers: {
      ...CSRF_HEADER,
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  })

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: ApiErrorBody } | null
    const error = body?.error
    throw new ApiError(
      response.status,
      error?.code ?? 'unknown_error',
      error?.message ?? 'Something went wrong.',
      error?.details,
    )
  }

  return response.json() as Promise<T>
}

export function apiGet<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'GET' })
}

export function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) })
}
