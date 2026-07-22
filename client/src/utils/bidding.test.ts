import { describe, expect, it } from 'vitest'
import { getBidIncrement } from './bidding'

describe('getBidIncrement', () => {
  it('is $100 below $5,000', () => {
    expect(getBidIncrement(0)).toBe(100)
    expect(getBidIncrement(4999)).toBe(100)
  })

  it('is $250 from $5,000 up to just under $15,000', () => {
    expect(getBidIncrement(5000)).toBe(250)
    expect(getBidIncrement(14999)).toBe(250)
  })

  it('is $500 at $15,000 and above', () => {
    expect(getBidIncrement(15000)).toBe(500)
    expect(getBidIncrement(1_000_000)).toBe(500)
  })
})
