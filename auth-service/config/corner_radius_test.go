package config

import "testing"

// CORNER_RADIUS is a name, Material UI's for its shape tokens, published as
// pixels; a name it does not know, or none, is the default.
func TestCornerRadiusIsReadByName(t *testing.T) {
	for given, want := range map[string]int{
		"none": 0, "sm": 4, "md": 8, "lg": 16, "xl": 24,
		" LG ": 16, "": 4, "round": 4, "12": 4,
	} {
		config := &Config{CornerRadius: given}
		if got := config.GetCornerRadius(); got != want {
			t.Errorf("CORNER_RADIUS=%q is published as %d, want %d", given, got, want)
		}
	}
}
