package listings

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// SortMode is one of the four sort options the client's existing
// inventoryFilters store already knows about (client/src/stores/inventoryFilters.ts).
type SortMode string

const (
	SortEnding    SortMode = "ending"
	SortPriceLow  SortMode = "price-low"
	SortPriceHigh SortMode = "price-high"
	SortYear      SortMode = "year"
)

// Cursor is the decoded form of the opaque, base64url(JSON)-encoded token a
// client passes back to resume a walk (guidelines/06-backend-architecture.md).
// Value is always a string on the wire regardless of the underlying column's
// real type (int64 for price, RFC3339 for a timestamp) -- each sort mode's
// query builder parses it back into the right Go type; keeping the cursor's
// own shape uniform is simpler than a union type for one JSON field.
type Cursor struct {
	Sort        SortMode `json:"sort"`
	Fingerprint string   `json:"fingerprint"`
	Bucket      int      `json:"bucket"` // only meaningful for SortEnding
	Value       string   `json:"v"`
	ID          string   `json:"id"`
}

// EncodeCursor never fails -- json.Marshal on this struct cannot error.
func EncodeCursor(c Cursor) string {
	data, _ := json.Marshal(c) //nolint:errcheck // Cursor has no unmarshalable field types
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeCursor rejects anything that isn't exactly what EncodeCursor produces --
// a cursor is never meant to be hand-constructed by a client.
func DecodeCursor(s string) (Cursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, domain.ErrInvalidCursor
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return Cursor{}, domain.ErrInvalidCursor
	}
	return c, nil
}

// Fingerprint hashes the normalized active sort+filter params -- deliberately
// not page size or direction, so switching those mid-walk from an existing
// cursor still works (guidelines/06-backend-architecture.md). Not a security
// boundary (nothing sensitive rides on cursor content), so a short prefix of
// the hash is plenty -- this only has to catch "the cursor was issued against
// a different query," not resist deliberate forgery.
func Fingerprint(sort SortMode, filter Filter) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s", sort, filter.Status, filter.Make, filter.Search)))
	return hex.EncodeToString(h[:])[:12]
}
