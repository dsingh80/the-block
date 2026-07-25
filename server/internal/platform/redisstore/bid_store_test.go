package redisstore

import (
	"errors"
	"testing"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// scriptErrorToDomain is pure -- the Lua scripts' actual behavior (which code
// they emit for which situation) is covered by test/integration's real-Redis
// bidding/buy_now tests; this only covers the code<->domain.DomainError
// mapping table itself, including the code Redis will never actually send.
func TestScriptErrorToDomain(t *testing.T) {
	cases := []struct {
		code string
		want error
	}{
		{"not_found", domain.ErrNotFound},
		{"auction_not_started", domain.ErrAuctionNotStarted},
		{"auction_ended", domain.ErrAuctionEnded},
		{"buy_now_unavailable", domain.ErrBuyNowUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			err := scriptErrorToDomain(scriptResult{Error: tc.code})
			if !errors.Is(err, tc.want) {
				t.Errorf("scriptErrorToDomain(%q) = %v, want %v", tc.code, err, tc.want)
			}
		})
	}

	t.Run("bid_too_low carries Minimum through into Details", func(t *testing.T) {
		err := scriptErrorToDomain(scriptResult{Error: "bid_too_low", Minimum: 21_100})
		if !errors.Is(err, domain.ErrBidTooLow) {
			t.Fatalf("err = %v, want domain.ErrBidTooLow", err)
		}
		var de *domain.DomainError
		if !errors.As(err, &de) {
			t.Fatal("errors.As(err, *domain.DomainError) = false, want true")
		}
		if got := de.Details["minimum"]; got != int64(21_100) {
			t.Errorf("Details[minimum] = %v, want int64(21100)", got)
		}
	})

	t.Run("an unrecognized code is a plain error, not a silent zero value", func(t *testing.T) {
		err := scriptErrorToDomain(scriptResult{Error: "some_future_code_the_lua_script_might_send"})
		if err == nil {
			t.Fatal("expected a non-nil error for an unrecognized script error code")
		}
		var de *domain.DomainError
		if errors.As(err, &de) {
			t.Errorf("errors.As(err, *domain.DomainError) = true, want false -- an unrecognized code has no envelope to map to")
		}
	})
}
