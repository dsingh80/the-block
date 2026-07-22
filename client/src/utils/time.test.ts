import { describe, expect, it } from 'vitest'
import { formatDuration, timeLabelFor } from './time'

describe('formatDuration', () => {
  it('shows minutes only under an hour', () => {
    expect(formatDuration(0.5)).toBe('30m')
  })

  it('shows hours and zero-padded minutes under a day', () => {
    expect(formatDuration(6.2)).toBe('6h 12m')
    expect(formatDuration(11 + 5 / 60)).toBe('11h 05m')
  })

  it('shows days and hours at 24h and above', () => {
    expect(formatDuration(28)).toBe('1d 4h')
  })
})

describe('timeLabelFor', () => {
  it('labels an upcoming listing', () => {
    expect(timeLabelFor('upcoming', 28)).toBe('Starts in 1d 4h')
  })

  it('labels an active listing', () => {
    expect(timeLabelFor('active', 6.2)).toBe('Ends in 6h 12m')
  })

  it('labels an ended listing using the negated remaining hours', () => {
    expect(timeLabelFor('ended', -3)).toBe('Ended 3h 00m ago')
  })
})
