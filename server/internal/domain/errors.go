package domain

// DomainError is a business-rule rejection with a stable, machine-readable code --
// the same vocabulary the Redis Lua accept path (a later commit) returns and the
// HTTP error envelope surfaces verbatim (guidelines/06-backend-architecture.md).
// Message text is a transport concern (formatting, currency symbols) and is built
// by the transport layer from Code + Details, not stored here.
type DomainError struct {
	Code    string
	Details map[string]any
}

func (e *DomainError) Error() string { return e.Code }

// Is compares by Code so errors.Is matches both a fixed sentinel and a
// dynamically-constructed error of the same kind (e.g. NewBidTooLowError with a
// different Details each time) without needing pointer identity.
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

var (
	ErrNotFound           = &DomainError{Code: "not_found"}
	ErrAuctionNotStarted  = &DomainError{Code: "auction_not_started"}
	ErrAuctionEnded       = &DomainError{Code: "auction_ended"}
	ErrBidTooLow          = &DomainError{Code: "bid_too_low"}
	ErrBuyNowUnavailable  = &DomainError{Code: "buy_now_unavailable"}
	ErrInvalidCursor      = &DomainError{Code: "invalid_cursor"}
	ErrCursorSortMismatch = &DomainError{Code: "cursor_sort_mismatch"}
)

// NewBidTooLowError is the one domain error that carries extra data -- the minimum
// acceptable amount, echoed back so a client can show it inline without a second round trip.
func NewBidTooLowError(minimum int64) *DomainError {
	return &DomainError{Code: ErrBidTooLow.Code, Details: map[string]any{"minimum": minimum}}
}
