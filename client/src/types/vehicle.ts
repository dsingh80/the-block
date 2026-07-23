export type FuelType = 'gasoline' | 'hybrid' | 'electric' | 'diesel'
export type TitleStatus = 'clean' | 'rebuilt' | 'salvage'

/** Mirrors a record in data/vehicles.json exactly. */
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
  starting_bid: number
  reserve_price: number | null
  buy_now_price: number | null
  images: string[]
  selling_dealership: string
  lot: string
  current_bid: number | null
  bid_count: number
}
