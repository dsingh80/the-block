package listings

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/dsingh80/the-block/server/internal/domain"
)

func TestCursor_RoundTrips(t *testing.T) {
	original := Cursor{Sort: SortEnding, Fingerprint: "abc123", Bucket: 1, Value: "2026-01-01T00:00:00Z", ID: "listing-1"}

	got, err := DecodeCursor(EncodeCursor(original))
	if err != nil {
		t.Fatalf("DecodeCursor(EncodeCursor(x)): %v", err)
	}
	if got != original {
		t.Errorf("round-tripped cursor = %+v, want %+v", got, original)
	}
}

func TestDecodeCursor_RejectsGarbage(t *testing.T) {
	t.Run("invalid base64", func(t *testing.T) {
		_, err := DecodeCursor("not valid base64!!!")
		if !errors.Is(err, domain.ErrInvalidCursor) {
			t.Errorf("err = %v, want domain.ErrInvalidCursor", err)
		}
	})

	t.Run("valid base64, invalid JSON", func(t *testing.T) {
		bogus := base64.RawURLEncoding.EncodeToString([]byte("this is not json"))
		_, err := DecodeCursor(bogus)
		if !errors.Is(err, domain.ErrInvalidCursor) {
			t.Errorf("err = %v, want domain.ErrInvalidCursor", err)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := DecodeCursor("")
		if !errors.Is(err, domain.ErrInvalidCursor) {
			t.Errorf("err = %v, want domain.ErrInvalidCursor", err)
		}
	})
}

func TestFingerprint_DeterministicAndDistinguishing(t *testing.T) {
	base := Fingerprint(SortEnding, Filter{Status: "active", Make: "Mazda", Search: ""})

	t.Run("same inputs give the same fingerprint", func(t *testing.T) {
		again := Fingerprint(SortEnding, Filter{Status: "active", Make: "Mazda", Search: ""})
		if again != base {
			t.Errorf("Fingerprint gave %q then %q for identical inputs", base, again)
		}
	})

	t.Run("a different sort gives a different fingerprint", func(t *testing.T) {
		if got := Fingerprint(SortPriceLow, Filter{Status: "active", Make: "Mazda"}); got == base {
			t.Error("changing sort did not change the fingerprint")
		}
	})

	t.Run("a different filter gives a different fingerprint", func(t *testing.T) {
		if got := Fingerprint(SortEnding, Filter{Status: "ended", Make: "Mazda"}); got == base {
			t.Error("changing status did not change the fingerprint")
		}
		if got := Fingerprint(SortEnding, Filter{Status: "active", Make: "Toyota"}); got == base {
			t.Error("changing make did not change the fingerprint")
		}
	})
}
