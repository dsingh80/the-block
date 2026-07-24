package domain

import "testing"

func TestComputeViewer(t *testing.T) {
	me := "session-me"
	other := "session-other"

	tests := []struct {
		name                string
		highBidderSessionID *string
		bidListingIDs       map[string]struct{}
		want                Viewer
	}{
		{
			name:                "the high bidder who has bid is neither outbid nor a stranger",
			highBidderSessionID: &me,
			bidListingIDs:       map[string]struct{}{"listing-1": {}},
			want:                Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false},
		},
		{
			name:                "having bid but someone else is now the high bidder is outbid",
			highBidderSessionID: &other,
			bidListingIDs:       map[string]struct{}{"listing-1": {}},
			want:                Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
		},
		{
			name:                "never having bid at all is neither outbid nor high bidder, regardless of who is",
			highBidderSessionID: &other,
			bidListingIDs:       map[string]struct{}{},
			want:                Viewer{HasBid: false, IsHighBidder: false, IsOutbid: false},
		},
		{
			name:                "no high bidder yet on the listing at all",
			highBidderSessionID: nil,
			bidListingIDs:       map[string]struct{}{"listing-1": {}},
			want:                Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := Listing{ID: "listing-1", HighBidderSessionID: tt.highBidderSessionID}
			got := ComputeViewer(l, me, tt.bidListingIDs)
			if got != tt.want {
				t.Errorf("ComputeViewer() = %+v, want %+v", got, tt.want)
			}
		})
	}

	t.Run("an empty session token (no cookie resolved yet) is never the high bidder", func(t *testing.T) {
		l := Listing{ID: "listing-1", HighBidderSessionID: nil}
		got := ComputeViewer(l, "", map[string]struct{}{})
		if got.IsHighBidder {
			t.Errorf("IsHighBidder = true for an empty session token, want false")
		}
	})
}
