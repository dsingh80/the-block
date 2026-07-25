import { describe, expect, it } from 'vitest'
import { formatDuration, timeLabelFor, timeSince } from './time'

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

describe('timeSince', () => {
  it('reads as "just now" under a minute', () => {
    expect(timeSince(0)).toBe('just now')
    expect(timeSince(59_000)).toBe('just now')
  })

  it('clamps a negative elapsed time to "just now" instead of going negative', () => {
    expect(timeSince(-5000)).toBe('just now')
  })

  it('shows whole minutes under an hour', () => {
    expect(timeSince(5 * 60_000)).toBe('5m ago')
  })

  it('shows whole hours under a day', () => {
    expect(timeSince(3 * 60 * 60_000)).toBe('3h ago')
  })

  it('shows whole days at 24h and above', () => {
    expect(timeSince(2 * 24 * 60 * 60_000)).toBe('2d ago')
  })
})
