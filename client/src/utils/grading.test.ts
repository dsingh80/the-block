import { describe, expect, it } from 'vitest'
import { gradeVariant, gradeLabel, titleLabel } from './grading'

describe('gradeVariant', () => {
  it('is poor below 3.0', () => {
    expect(gradeVariant(1)).toBe('poor')
    expect(gradeVariant(2.9)).toBe('poor')
  })

  it('is fair from 3.0 up to just under 4.0', () => {
    expect(gradeVariant(3.0)).toBe('fair')
    expect(gradeVariant(3.9)).toBe('fair')
  })

  it('is good at 4.0 and above', () => {
    expect(gradeVariant(4.0)).toBe('good')
    expect(gradeVariant(5.0)).toBe('good')
  })
})

describe('gradeLabel', () => {
  it('labels every band', () => {
    expect(gradeLabel(5.0)).toBe('Excellent')
    expect(gradeLabel(3.6)).toBe('Good')
    expect(gradeLabel(2.6)).toBe('Fair')
    expect(gradeLabel(1.6)).toBe('Poor')
    expect(gradeLabel(1.0)).toBe('Very Poor')
  })
})

describe('titleLabel', () => {
  it('labels each title status', () => {
    expect(titleLabel('clean')).toBe('Clean Title')
    expect(titleLabel('rebuilt')).toBe('Rebuilt Title')
    expect(titleLabel('salvage')).toBe('Salvage Title')
  })
})
