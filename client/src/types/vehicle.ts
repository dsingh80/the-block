import type { Lifecycle } from './listing'

export type FuelType = 'gasoline' | 'hybrid' | 'electric' | 'diesel'
export type TitleStatus = 'clean' | 'rebuilt' | 'salvage'

/**
 * Mirrors server/'s ListingSummary DTO (guidelines/06-backend-architecture.md)
 * -- every Vehicle in the app is now sourced from the real API
 * (services/api/types.ts's ApiListingSummary, minus `viewer`), so
 * `auction_end`/`status`/`purchased_at` are as required here as they are
 * there; the server always computes/derives all three. No `reserve_price` at
 * all: the server never sends it either (guidelines/06-backend-architecture.md,
 * "reserve_price").
 */
export interface Vehicle {
  id: string
  vin: string
  year: number
  make: string
  model: string
  trim: string
  body_style: string
  exterior_color: string
  interior_color: string
  engine: string
  transmission: string
  drivetrain: string
  odometer_km: number
  fuel_type: FuelType
  condition_grade: number
  condition_report: string
  damage_notes: string[]
  title_status: TitleStatus
  province: string
  city: string
  auction_start: string
  auction_end: string
  status: Lifecycle
  starting_bid: number
  buy_now_price: number | null
  images: string[]
  selling_dealership: string
  lot: string
  current_bid: number | null
  bid_count: number
  purchased_at: string | null
}
