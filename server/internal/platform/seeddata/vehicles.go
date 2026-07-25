// Package seeddata loads data/vehicles.json (the generator's output, scripts/generate_vehicles.mjs)
// into domain.Listing values for cmd/reconcile to sync into Postgres
// (guidelines/06-backend-architecture.md, "Data lifecycle").
package seeddata

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// DefaultAuctionDuration matches the 24h assumption already locked in
// guidelines/00-overview.md -- the generator has no per-vehicle duration field,
// so every seeded listing gets the same default (auction_duration_sec on the row
// is still a real column an operator can change afterward -- guidelines/06-backend-architecture.md).
const DefaultAuctionDuration = 24 * time.Hour

// naiveDateTimeLayout matches the generator's auction_start format, which carries
// no timezone marker. Parsed as UTC: Go's time.Parse defaults to UTC when the
// layout has no zone, and that's the only unambiguous choice available here --
// the generator (scripts/generate_vehicles.mjs's formatLocalDateTime) actually
// writes the *local* wall-clock time of whatever machine ran it, so the
// originally-intended zone can't be recovered from a committed static file
// (guidelines/06-backend-architecture.md).
const naiveDateTimeLayout = "2006-01-02T15:04:05"

type rawVehicle struct {
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
	AuctionStart      string   `json:"auction_start"`
	StartingBid       int64    `json:"starting_bid"`
	ReservePrice      *int64   `json:"reserve_price"`
	BuyNowPrice       *int64   `json:"buy_now_price"`
	Images            []string `json:"images"`
	SellingDealership string   `json:"selling_dealership"`
	Lot               string   `json:"lot"`
	CurrentBid        *int64   `json:"current_bid"`
	BidCount          int      `json:"bid_count"`
}

// LoadVehicles reads and parses path (data/vehicles.json's shape) into domain.Listing
// values, ready for pgstore.InsertNewListings.
func LoadVehicles(path string) ([]domain.Listing, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("seeddata: read %s: %w", path, err)
	}

	var raw []rawVehicle
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("seeddata: parse %s: %w", path, err)
	}

	listings := make([]domain.Listing, 0, len(raw))
	for _, v := range raw {
		l, err := v.toListing()
		if err != nil {
			return nil, fmt.Errorf("seeddata: vehicle %s: %w", v.ID, err)
		}
		listings = append(listings, l)
	}
	return listings, nil
}

func (v rawVehicle) toListing() (domain.Listing, error) {
	start, err := time.Parse(naiveDateTimeLayout, v.AuctionStart)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("parse auction_start %q: %w", v.AuctionStart, err)
	}

	// Mirrors the client's own fallback chain (client/src/stores/bids.ts):
	// override?.currentPrice ?? vehicle.current_bid ?? vehicle.starting_bid -- at
	// seed time there's no override yet, so it's just current_bid ?? starting_bid.
	currentPrice := v.StartingBid
	if v.CurrentBid != nil {
		currentPrice = *v.CurrentBid
	}

	return domain.Listing{
		ID: v.ID, VIN: v.VIN, Year: v.Year, Make: v.Make, Model: v.Model, Trim: v.Trim,
		BodyStyle: v.BodyStyle, ExteriorColor: v.ExteriorColor, InteriorColor: v.InteriorColor,
		Engine: v.Engine, Transmission: v.Transmission, Drivetrain: v.Drivetrain,
		OdometerKM: v.OdometerKM, FuelType: domain.FuelType(v.FuelType), ConditionGrade: v.ConditionGrade,
		ConditionReport: v.ConditionReport, DamageNotes: nonNilStrings(v.DamageNotes),
		TitleStatus: domain.TitleStatus(v.TitleStatus), Province: v.Province, City: v.City,
		AuctionStart: start, AuctionDuration: DefaultAuctionDuration,
		StartingBid: v.StartingBid, ReservePrice: v.ReservePrice, BuyNowPrice: v.BuyNowPrice,
		Images: nonNilStrings(v.Images), SellingDealership: v.SellingDealership, Lot: v.Lot,
		CurrentPrice: currentPrice, BidCount: v.BidCount,
	}, nil
}

// nonNilStrings guards against the `text[] NOT NULL` columns (damage_notes, images)
// ever being handed a nil slice -- pgx encodes a nil Go slice as SQL NULL, which
// would violate the NOT NULL constraint even though every real record in the
// dataset always provides a (possibly empty) array, never a JSON null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
