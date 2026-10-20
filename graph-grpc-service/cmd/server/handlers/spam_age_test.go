package handlers

import (
	"testing"
	"time"
)

// WHAT THIS GUARDS (roadmap row 102): only an account newer than the
// installation allows is removed as spam; one here longer is disabled or
// deleted instead, with the thirty days to bring it back (the user,
// 2026-10-19).
func TestOnlyANewAccountIsRemovedAsSpam(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		made time.Time
		old  bool
	}{
		{"made today", now.Add(-time.Hour), false},
		{"made 29 days ago", now.AddDate(0, 0, -29), false},
		{"made 31 days ago", now.AddDate(0, 0, -31), true},
		{"no date", time.Time{}, false},
	} {
		if got := tooOldForSpam(tc.made, now, 30); got != tc.old {
			t.Errorf("%s: too old %v, want %v", tc.name, got, tc.old)
		}
	}
}
