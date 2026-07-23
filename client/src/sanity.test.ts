import { describe, expect, it } from 'vitest'

// Confirms the Vitest harness itself runs (config, jsdom environment, TS
// transform) before any real logic depends on it in later commits.
describe('test harness', () => {
  it('runs', () => {
    expect(1 + 1).toBe(2)
  })
})
