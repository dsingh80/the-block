package pgstore

import (
	"strings"
	"testing"
)

// TestMigrate_MalformedDSNFailsAtInit covers Migrate's own init-error wrapping
// (NewWithSourceInstance) -- distinct from test/integration's
// TestMigrate_CreatesExpectedSchema, which exercises the real apply/idempotent
// path against a live container. A malformed DSN fails driver init before any
// network attempt, so this needs no Postgres at all.
func TestMigrate_MalformedDSNFailsAtInit(t *testing.T) {
	err := Migrate("not a valid dsn at all")
	if err == nil {
		t.Fatal("expected an error for a malformed DSN, got nil")
	}
	if !strings.Contains(err.Error(), "pgstore:") {
		t.Errorf("error = %q, want it wrapped with a pgstore: prefix", err.Error())
	}
}
