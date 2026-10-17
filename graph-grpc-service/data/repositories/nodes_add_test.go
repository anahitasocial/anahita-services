package repositories

import (
	"testing"
	"time"
)

// A new node keeps the date it says it was written, as a Pulse's article
// keeps its item's, unless that date was never set or is still to come.
func TestANewNodeKeepsTheDateItSays(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	written := now.Add(-3 * time.Hour)

	for _, c := range []struct {
		name string
		said time.Time
		want time.Time
	}{
		{"its own date", written, written},
		{"never set", time.Time{}, now},
		{"set to 1970", time.Unix(0, 0), now},
		{"in the future", now.Add(time.Minute), now},
	} {
		if got := addedAt(c.said, now); !got.Equal(c.want) {
			t.Errorf("%s: made at %v, want %v", c.name, got, c.want)
		}
	}
}
