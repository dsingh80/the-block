package pgstore

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

// filterWhere is pure string/args building -- no Postgres needed. The actual
// filter *semantics* (does a status/make/search predicate really match the
// right rows) are covered by test/integration's TestListingReader_FilterSemanticsAndDistinctMakes,
// which needs a real database; this only guards the placeholder-numbering
// contract every ListPage sort mode depends on.
func TestFilterWhere(t *testing.T) {
	t.Run("always returns exactly 3 clauses (status, make, search)", func(t *testing.T) {
		clauses, args := filterWhere(listings.Filter{}, 1)
		if len(clauses) != 3 {
			t.Errorf("len(clauses) = %d, want 3", len(clauses))
		}
		if len(args) != 3 {
			t.Errorf("len(args) = %d, want 3", len(args))
		}
	})

	t.Run("args echo the filter's fields in status/make/search order", func(t *testing.T) {
		_, args := filterWhere(listings.Filter{Status: "active", Make: "Mazda", Search: "cx-5"}, 1)
		want := []any{"active", "Mazda", "cx-5"}
		for i := range want {
			if args[i] != want[i] {
				t.Errorf("args[%d] = %v, want %v", i, args[i], want[i])
			}
		}
	})

	t.Run("startArgIndex offsets every placeholder so a caller can append its own predicates after", func(t *testing.T) {
		clauses, _ := filterWhere(listings.Filter{}, 4)
		for i, c := range clauses {
			// Each clause's own placeholders should start at $4, $5, $6 respectively --
			// not always $1 -- so a cursor predicate appended after these continues
			// numbering correctly instead of colliding with an earlier placeholder.
			wantPlaceholder := fmt.Sprintf("$%d", 4+i)
			if !strings.Contains(c, wantPlaceholder) {
				t.Errorf("clause[%d] = %q, want it to reference placeholder %s", i, c, wantPlaceholder)
			}
		}
	})
}
