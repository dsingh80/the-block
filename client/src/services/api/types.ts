import type { FuelType, TitleStatus } from '@/types/vehicle'
import type { Lifecycle } from '@/types/listing'

/**
 * Wire shapes for server/'s JSON responses. Field names match the server's
 * DTOs exactly (snake_case), since these are what `JSON.parse` actually
 * produces -- mapping to camelCase happens at the store layer, not here.
 */

export interface ApiViewer {
  has_bid: boolean
  is_high_bidder: boolean
  is_outbid: boolean
}

/**
 * Mirrors server/internal/transport/dto.ListingSummary. No `reserve_price`
 * field at all -- the server never sends it (guidelines/06-backend-architecture.md,
 * "reserve_price").
 */
export interface ApiListingSummary {
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
  viewer: ApiViewer
}

export interface ApiPageInfo {
  has_next_page: boolean
  has_previous_page: boolean
  start_cursor?: string
  end_cursor?: string
}

export interface ApiListingsPage {
  data: ApiListingSummary[]
  page_info: ApiPageInfo
}

export interface ApiFacets {
  makes: string[]
}

export interface ApiBidHistoryEntry {
  handle: string
  type: 'bid' | 'buy_now'
  amount: number
  bid_count: number
  accepted_at: string
  is_viewer: boolean
}

/** Mirrors server/internal/transport/dto.BidAccept -- the unwrapped `data` of a bid/buy-now response. */
export interface ApiBidAccept {
  bid_id: string
  current_bid: number
  bid_count: number
  accepted_at: string
  viewer: ApiViewer
}

/** Mirrors server/internal/transport/dto.ErrorBody. */
export interface ApiErrorBody {
  code: string
  message: string
  details?: Record<string, unknown>
  request_id: string
}
