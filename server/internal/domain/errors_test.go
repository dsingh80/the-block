package domain

import (
	"errors"
	"testing"
)

func TestDomainErrorIs(t *testing.T) {
	t.Run("a fixed sentinel matches itself via errors.Is", func(t *testing.T) {
		var err error = ErrAuctionEnded
		if !errors.Is(err, ErrAuctionEnded) {
			t.Error("expected errors.Is(ErrAuctionEnded, ErrAuctionEnded) to be true")
		}
		if errors.Is(err, ErrNotFound) {
			t.Error("expected errors.Is(ErrAuctionEnded, ErrNotFound) to be false")
		}
	})

	t.Run("two distinct BidTooLow instances still match by code, not pointer identity", func(t *testing.T) {
		a := NewBidTooLowError(21_100)
		b := NewBidTooLowError(999_999) // different Details, same Code

		if !errors.Is(a, ErrBidTooLow) {
			t.Error("expected a dynamically-constructed bid_too_low error to match the ErrBidTooLow sentinel")
		}
		if !errors.Is(a, b) {
			t.Error("expected two bid_too_low errors with different Details to still match each other by code")
		}
		if a.Details["minimum"] != int64(21_100) {
			t.Errorf("expected Details[minimum] = 21100, got %v", a.Details["minimum"])
		}
	})
}
