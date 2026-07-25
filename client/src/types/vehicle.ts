import type { Lifecycle } from './listing'

export type FuelType = 'gasoline' | 'hybrid' | 'electric' | 'diesel'
export type TitleStatus = 'clean' | 'rebuilt' | 'salvage'

/**
 * Mirrors server/'s ListingSummary DTO (guidelines/06-backend-architecture.md)
 * once fetched from the real API. `auction_end`/`status`/`purchased_at` are
 * optional because the static seed data (data/vehicles.json, used only until
 * the static-import-removal commit) never populates them -- the server
 * computes/derives all three, the generator script has no equivalent. No
 * `reserve_price` at all: the server never sends it either
 * (guidelines/06-backend-architecture.md, "reserve_price").
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
  auction_end?: string
  status?: Lifecycle
  starting_bid: number
  buy_now_price: number | null
  images: string[]
  selling_dealership: string
  lot: string
  current_bid: number | null
  bid_count: number
  purchased_at?: string | null
}
