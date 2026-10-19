package constants

import "testing"

func TestHowManyImagesAPostHolds(t *testing.T) {
	for value, want := range map[string]int{
		"":     PhotoMaxFilesCeiling,
		"4":    4,
		" 10 ": 10,
		"0":    PhotoMaxFilesCeiling,
		"-3":   PhotoMaxFilesCeiling,
		"50":   PhotoMaxFilesCeiling,
		"many": PhotoMaxFilesCeiling,
	} {
		t.Setenv(EnvPhotoMaxFiles, value)
		if got := PhotoMaxFiles(); got != want {
			t.Errorf("%s=%q: %d, not %d", EnvPhotoMaxFiles, value, got, want)
		}
	}
}
