package config

import "testing"

// CORNER_RADIUS is any whole number of pixels, but a corner past
// MaxCornerRadius is no longer a corner, and one below zero is nothing.
func TestCornerRadiusIsHeldWithinItsBounds(t *testing.T) {
	for given, want := range map[int]int{-3: 0, 0: 0, 3: 3, 4: 4, 16: 16, 24: 24, 80: 24} {
		config := &Config{CornerRadius: given}
		if got := config.GetCornerRadius(); got != want {
			t.Errorf("CORNER_RADIUS=%d is published as %d, want %d", given, got, want)
		}
	}
}
