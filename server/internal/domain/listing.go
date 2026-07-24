package domain

import "time"

type FuelType string

const (
	FuelGasoline FuelType = "gasoline"
	FuelHybrid   FuelType = "hybrid"
	FuelElectric FuelType = "electric"
	FuelDiesel   FuelType = "diesel"
)

type TitleStatus string

const (
	TitleClean   TitleStatus = "clean"
	TitleRebuilt TitleStatus = "rebuilt"
	TitleSalvage TitleStatus = "salvage"
)

// Listing mirrors a row in the `listings` table (guidelines/06-backend-architecture.md) --
// the server's authoritative vehicle+auction record. Money fields are whole integers;
// there are no cents anywhere in this domain. ReservePrice is stored here but is
// deliberately absent from the buyer-facing wire DTO (internal/transport/dto), not from
// this type -- exclusion is a transport-layer concern, not a data-layer one.
type Listing struct {
	ID                string
	VIN               string
	Year              int
	Make              string
	Model             string
	Trim              string
	BodyStyle         string
	ExteriorColor     string
	InteriorColor     string
	Engine            string
	Transmission      string
	Drivetrain        string
	OdometerKM        int
	FuelType          FuelType
	ConditionGrade    float64
	ConditionReport   string
	DamageNotes       []string
	TitleStatus       TitleStatus
	Province          string
	City              string
	AuctionStart      time.Time
	AuctionDuration   time.Duration
	AuctionEnd        time.Time
	StartingBid       int64
	ReservePrice      *int64
	BuyNowPrice       *int64
	Images            []string
	SellingDealership string
	Lot               string

	CurrentPrice        int64
	BidCount            int
	HighBidderSessionID *string
	PurchasedAt         *time.Time
}

// Status derives upcoming/active/ended for this listing at instant now. A completed
// Buy Now forces ended regardless of the clock, mirroring the client's existing
// purchased -> ended override (client/src/composables/useListingPresentation.ts) --
// once any session can end a listing early for every other viewer, the server has to
// be the one to say so.
func (l Listing) Status(now time.Time) Lifecycle {
	if l.PurchasedAt != nil {
		return LifecycleEnded
	}
	return DeriveLifecycle(l.AuctionStart, l.AuctionEnd, now)
}
