package domain

import "testing"

// Mirrors client/src/utils/bidding.test.ts's exact boundary pairs.
func TestGetBidIncrement(t *testing.T) {
	tests := []struct {
		name  string
		price int64
		want  int64
	}{
		{"is $100 at $0", 0, 100},
		{"is $100 at $4,999", 4_999, 100},
		{"is $250 at $5,000", 5_000, 250},
		{"is $250 at $14,999", 14_999, 250},
		{"is $500 at $15,000", 15_000, 500},
		{"is $500 at $1,000,000", 1_000_000, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetBidIncrement(tt.price); got != tt.want {
				t.Errorf("GetBidIncrement(%d) = %d, want %d", tt.price, got, tt.want)
			}
		})
	}
}
