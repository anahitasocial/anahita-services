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

func TestHowNewAnAccountRemovedAsSpamMustBe(t *testing.T) {
	for value, want := range map[string]int{"": 30, "7": 7, "0": 30, "-1": 30, "soon": 30} {
		t.Setenv(EnvSpamRemovalMaxDays, value)
		if got := SpamRemovalMaxDays(); got != want {
			t.Errorf("%s=%q: %d, not %d", EnvSpamRemovalMaxDays, value, got, want)
		}
	}
}

func TestHowLongADeletedProfileCanBeBroughtBack(t *testing.T) {
	for value, want := range map[string]int{"": 30, "14": 14, "90": 90, "0": 30, "-5": 30, "a month": 30} {
		t.Setenv(EnvAccountGraceDays, value)
		if got := AccountGraceDays(); got != want {
			t.Errorf("%s=%q: %d, not %d", EnvAccountGraceDays, value, got, want)
		}
	}
}
