import raw from './vehicles.json'
import type { Vehicle } from '@/types/vehicle'

export const vehicles: Vehicle[] = raw as Vehicle[]

export const vehiclesById: Map<string, Vehicle> = new Map(vehicles.map((vehicle) => [vehicle.id, vehicle]))

export const MAKE_OPTIONS: string[] = [...new Set(vehicles.map((vehicle) => vehicle.make))].sort()

export const sellerCounts: Map<string, number> = vehicles.reduce((counts, vehicle) => {
  counts.set(vehicle.selling_dealership, (counts.get(vehicle.selling_dealership) ?? 0) + 1)
  return counts
}, new Map<string, number>())
