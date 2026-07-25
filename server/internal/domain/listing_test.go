package domain

import (
	"testing"
	"time"
)

func TestListingStatus(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	base := Listing{AuctionStart: start, AuctionEnd: end}

	t.Run("delegates to DeriveLifecycle when not purchased", func(t *testing.T) {
		l := base
		if got := l.Status(start.Add(-time.Hour)); got != LifecycleUpcoming {
			t.Errorf("Status() = %v, want %v", got, LifecycleUpcoming)
		}
		if got := l.Status(start); got != LifecycleActive {
			t.Errorf("Status() = %v, want %v", got, LifecycleActive)
		}
		if got := l.Status(end); got != LifecycleEnded {
			t.Errorf("Status() = %v, want %v", got, LifecycleEnded)
		}
	})

	t.Run("a completed Buy Now forces ended even mid-auction", func(t *testing.T) {
		purchasedAt := start.Add(time.Hour)
		l := base
		l.PurchasedAt = &purchasedAt

		if got := l.Status(start.Add(time.Hour)); got != LifecycleEnded {
			t.Errorf("Status() = %v, want %v (purchased should force ended)", got, LifecycleEnded)
		}
	})

	t.Run("a completed Buy Now forces ended even before auction_start", func(t *testing.T) {
		purchasedAt := start.Add(-time.Minute)
		l := base
		l.PurchasedAt = &purchasedAt

		if got := l.Status(start.Add(-time.Hour)); got != LifecycleEnded {
			t.Errorf("Status() = %v, want %v (purchased should force ended even pre-start)", got, LifecycleEnded)
		}
	})
}
