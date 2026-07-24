package domain

import (
	"testing"
	"time"
)

// Mirrors client/src/utils/lifecycle.test.ts's boundary cases exactly, so the two
// implementations can never silently drift on the active/upcoming/ended boundary math.
func TestDeriveLifecycle(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	tests := []struct {
		name string
		now  time.Time
		want Lifecycle
	}{
		{"upcoming before the start time", start.Add(-time.Hour), LifecycleUpcoming},
		{"active exactly at the start time", start, LifecycleActive},
		{"active in the middle of the window", start.Add(12 * time.Hour), LifecycleActive},
		{"active just before the end boundary", end.Add(-time.Millisecond), LifecycleActive},
		{"ended exactly at the end boundary", end, LifecycleEnded},
		{"ended well after the end boundary", end.Add(24 * time.Hour), LifecycleEnded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveLifecycle(start, end, tt.now); got != tt.want {
				t.Errorf("DeriveLifecycle(now=%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}
