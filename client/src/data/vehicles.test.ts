import { describe, expect, it } from 'vitest'
import { MAKE_OPTIONS, sellerCounts, vehicles, vehiclesById } from './vehicles'

describe('vehicles data loader', () => {
  it('loads all 200 records from the dataset', () => {
    expect(vehicles).toHaveLength(200)
  })

  it('indexes every vehicle by id', () => {
    expect(vehiclesById.size).toBe(vehicles.length)
    expect(vehiclesById.get(vehicles[0].id)).toBe(vehicles[0])
  })

  it('produces sorted, deduplicated make options', () => {
    expect(MAKE_OPTIONS).toEqual([...MAKE_OPTIONS].sort())
    expect(new Set(MAKE_OPTIONS).size).toBe(MAKE_OPTIONS.length)
  })

  it('sums seller counts to the full dataset size', () => {
    const total = [...sellerCounts.values()].reduce((sum, count) => sum + count, 0)
    expect(total).toBe(vehicles.length)
  })

  it('counts one dealership correctly against a direct filter', () => {
    const dealership = vehicles[0].selling_dealership
    const expected = vehicles.filter((vehicle) => vehicle.selling_dealership === dealership).length
    expect(sellerCounts.get(dealership)).toBe(expected)
  })
})
