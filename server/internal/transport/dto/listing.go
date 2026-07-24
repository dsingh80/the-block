package dto

import (
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// ListingSummary is the buyer-facing wire shape for a listing -- a superset of
// the client's existing Vehicle type, additive only. reserve_price has
// deliberately no field at all here (not null): the struct itself can't
// serialize a value it doesn't have, which is what turns "buyers don't see
// reserve amounts" from a client-side habit into an actual trust boundary
// (guidelines/06-backend-architecture.md). Raw numbers and ISO timestamps
// throughout, not pre-formatted strings -- the client's own augment() already
// does that formatting from exactly this shape of data.
type ListingSummary struct {
	ID                string   `json:"id"`
	VIN               string   `json:"vin"`
	Year              int      `json:"year"`
	Make              string   `json:"make"`
	Model             string   `json:"model"`
	Trim              string   `json:"trim"`
	BodyStyle         string   `json:"body_style"`
	ExteriorColor     string   `json:"exterior_color"`
	InteriorColor     string   `json:"interior_color"`
	Engine            string   `json:"engine"`
	Transmission      string   `json:"transmission"`
	Drivetrain        string   `json:"drivetrain"`
	OdometerKM        int      `json:"odometer_km"`
	FuelType          string   `json:"fuel_type"`
	ConditionGrade    float64  `json:"condition_grade"`
	ConditionReport   string   `json:"condition_report"`
	DamageNotes       []string `json:"damage_notes"`
	TitleStatus       string   `json:"title_status"`
	Province          string   `json:"province"`
	City              string   `json:"city"`
	AuctionStart      string   `json:"auction_start"` // RFC3339
	AuctionEnd        string   `json:"auction_end"`   // RFC3339
	Status            string   `json:"status"`        // upcoming | active | ended
	StartingBid       int64    `json:"starting_bid"`
	BuyNowPrice       *int64   `json:"buy_now_price"`
	Images            []string `json:"images"`
	SellingDealership string   `json:"selling_dealership"`
	Lot               string   `json:"lot"`
	CurrentBid        *int64   `json:"current_bid"` // null until bid_count > 0, matching the client's current_bid ?? starting_bid fallback
	BidCount          int      `json:"bid_count"`
	PurchasedAt       *string  `json:"purchased_at"` // RFC3339, null unless Buy Now closed it
	Viewer            Viewer   `json:"viewer"`
}

// NewListingSummary maps a domain.Listing to the wire shape at instant now.
// Status is computed here (Listing.Status(now)), not stored -- see
// guidelines/06-backend-architecture.md. viewer is computed by the caller
// (domain.ComputeViewer) rather than here, since it needs the requesting
// session's token and bid-listing set -- inputs this otherwise-pure mapping
// function has no other reason to take.
func NewListingSummary(l domain.Listing, now time.Time, viewer domain.Viewer) ListingSummary {
	var currentBid *int64
	if l.BidCount > 0 {
		v := l.CurrentPrice
		currentBid = &v
	}

	var purchasedAt *string
	if l.PurchasedAt != nil {
		s := l.PurchasedAt.UTC().Format(time.RFC3339)
		purchasedAt = &s
	}

	return ListingSummary{
		ID: l.ID, VIN: l.VIN, Year: l.Year, Make: l.Make, Model: l.Model, Trim: l.Trim,
		BodyStyle: l.BodyStyle, ExteriorColor: l.ExteriorColor, InteriorColor: l.InteriorColor,
		Engine: l.Engine, Transmission: l.Transmission, Drivetrain: l.Drivetrain,
		OdometerKM: l.OdometerKM, FuelType: string(l.FuelType), ConditionGrade: l.ConditionGrade,
		ConditionReport: l.ConditionReport, DamageNotes: l.DamageNotes,
		TitleStatus: string(l.TitleStatus), Province: l.Province, City: l.City,
		AuctionStart: l.AuctionStart.UTC().Format(time.RFC3339),
		AuctionEnd:   l.AuctionEnd.UTC().Format(time.RFC3339),
		Status:       string(l.Status(now)),
		StartingBid:  l.StartingBid, BuyNowPrice: l.BuyNowPrice,
		Images: l.Images, SellingDealership: l.SellingDealership, Lot: l.Lot,
		CurrentBid: currentBid, BidCount: l.BidCount, PurchasedAt: purchasedAt,
		Viewer: NewViewer(viewer),
	}
}

// PageInfo is the Relay-connection-style page metadata alongside a list
// response (guidelines/06-backend-architecture.md, "Cursor pagination").
type PageInfo struct {
	HasNextPage bool   `json:"has_next_page"`
	HasPrevPage bool   `json:"has_previous_page"`
	StartCursor string `json:"start_cursor,omitempty"`
	EndCursor   string `json:"end_cursor,omitempty"`
}

// ListingsPage is the full GET /v1/listings response shape.
type ListingsPage struct {
	Data     []ListingSummary `json:"data"`
	PageInfo PageInfo         `json:"page_info"`
}

// Facets is the wire shape for GET /v1/listings/facets -- distinct filter
// values a list-page filter dropdown needs, which a paginated list can't
// cheaply provide on its own (guidelines/06-backend-architecture.md).
type Facets struct {
	Makes []string `json:"makes"`
}
