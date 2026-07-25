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

func TestViewer_ReconcileHighBidder(t *testing.T) {
	me := "session-me"
	other := "session-other"

	tests := []struct {
		name     string
		v        Viewer
		liveHigh string
		session  string
		want     Viewer
	}{
		{
			name:     "Postgres-stale outbid is corrected once Redis confirms this session is the live high bidder",
			v:        Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
			liveHigh: me,
			session:  me,
			want:     Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false},
		},
		{
			name:     "a genuine outbid by someone else stays outbid -- Redis agrees with Postgres",
			v:        Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
			liveHigh: other,
			session:  me,
			want:     Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
		},
		{
			name:     "no live high bidder recorded in Redis at all leaves a genuine outbid alone",
			v:        Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
			liveHigh: "",
			session:  me,
			want:     Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
		},
		{
			name:     "never having bid is left alone regardless of who Redis says is winning",
			v:        Viewer{HasBid: false, IsHighBidder: false, IsOutbid: false},
			liveHigh: me,
			session:  me,
			want:     Viewer{HasBid: false, IsHighBidder: false, IsOutbid: false},
		},
		{
			name:     "already the high bidder is a no-op, not just an equal result -- never re-derives what ComputeViewer already settled",
			v:        Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false},
			liveHigh: other,
			session:  me,
			want:     Viewer{HasBid: true, IsHighBidder: true, IsOutbid: false},
		},
		{
			name:     "an empty session token never reconciles to high bidder",
			v:        Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
			liveHigh: "",
			session:  "",
			want:     Viewer{HasBid: true, IsHighBidder: false, IsOutbid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.ReconcileHighBidder(tt.liveHigh, tt.session)
			if got != tt.want {
				t.Errorf("ReconcileHighBidder(%q, %q) = %+v, want %+v", tt.liveHigh, tt.session, got, tt.want)
			}
		})
	}
}
