import { describe, expect, it } from 'vitest'
import { deriveLifecycle } from './lifecycle'

const HOUR_MS = 60 * 60 * 1000
const start = new Date('2026-01-01T12:00:00Z').getTime()
const auctionStart = new Date(start).toISOString()

describe('deriveLifecycle', () => {
  it('is upcoming before the start time', () => {
    expect(deriveLifecycle(auctionStart, start - HOUR_MS)).toBe('upcoming')
  })

  it('is active exactly at the start time', () => {
    expect(deriveLifecycle(auctionStart, start)).toBe('active')
  })

  it('is active in the middle of the 24h window', () => {
    expect(deriveLifecycle(auctionStart, start + 12 * HOUR_MS)).toBe('active')
  })

  it('is active just before the 24h end boundary', () => {
    expect(deriveLifecycle(auctionStart, start + 24 * HOUR_MS - 1)).toBe('active')
  })

  it('is ended exactly at the 24h end boundary', () => {
    expect(deriveLifecycle(auctionStart, start + 24 * HOUR_MS)).toBe('ended')
  })

  it('is ended well after the end boundary', () => {
    expect(deriveLifecycle(auctionStart, start + 48 * HOUR_MS)).toBe('ended')
  })
})
