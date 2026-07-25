package redisstore

import "testing"

// redis.NewClient never dials eagerly, so this needs no real Redis -- it just
// guards NewClient's own existence as the one call site every other package
// should go through instead of importing go-redis directly (see the package
// doc comment on NewClient).
func TestNewClient(t *testing.T) {
	c := NewClient("localhost:6379")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	defer c.Close()

	if got := c.Options().Addr; got != "localhost:6379" {
		t.Errorf("Options().Addr = %q, want %q", got, "localhost:6379")
	}
}
