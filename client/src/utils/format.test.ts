import { describe, expect, it } from 'vitest'
import { currency, formatGrade, formatKm } from './format'

describe('currency', () => {
  it('formats and rounds to whole dollars with thousands separators', () => {
    expect(currency(22800)).toBe('$22,800')
    expect(currency(1499.6)).toBe('$1,500')
  })
})

describe('formatKm', () => {
  it('formats with thousands separators and a km suffix', () => {
    expect(formatKm(47731)).toBe('47,731 km')
  })
})

describe('formatGrade', () => {
  it('always shows exactly one decimal place', () => {
    expect(formatGrade(4)).toBe('4.0')
    expect(formatGrade(3.8)).toBe('3.8')
  })
})
