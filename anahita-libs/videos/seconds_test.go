package videos

import "testing"

// A length in seconds is a positive number an int holds; anything else,
// too large included, is the default.
func TestSecondsAreReadAsAnIntHoldsThem(t *testing.T) {
	for _, each := range []struct {
		raw  string
		want int
	}{
		{"", DefaultAudioMaxSeconds},
		{"90", 90},
		{" 120 ", 120},
		{"0", DefaultAudioMaxSeconds},
		{"-5", DefaultAudioMaxSeconds},
		{"ten", DefaultAudioMaxSeconds},
		{"2147483647", 2147483647},
		{"2147483648", DefaultAudioMaxSeconds},
		{"9223372036854775807", DefaultAudioMaxSeconds},
	} {
		t.Setenv(EnvAudioMaxSeconds, each.raw)
		if got := AudioMaxSeconds(); got != each.want {
			t.Errorf("AUDIO_MAX_SECONDS=%q: %d, want %d", each.raw, got, each.want)
		}
	}
}
