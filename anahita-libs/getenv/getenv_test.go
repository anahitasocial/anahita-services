package getenv

import "testing"

func TestString(t *testing.T) {
	t.Setenv("GETENV_TEST", "set")
	if got := String("GETENV_TEST", "fallback"); got != "set" {
		t.Errorf("String() = %q, want set", got)
	}
	t.Setenv("GETENV_TEST", "")
	if got := String("GETENV_TEST", "fallback"); got != "fallback" {
		t.Errorf("String() of an empty variable = %q, want fallback", got)
	}
}

func TestInt(t *testing.T) {
	cases := []struct {
		value string
		want  int
	}{
		{"42", 42},
		{"0", 0},
		{"", 7},
		{"forty-two", 7},
	}
	for _, tc := range cases {
		t.Setenv("GETENV_TEST", tc.value)
		if got := Int("GETENV_TEST", 7); got != tc.want {
			t.Errorf("Int(%q) = %d, want %d", tc.value, got, tc.want)
		}
	}
}

// THE RULE THIS GUARDS (roadmap row 147): a renamed setting is read by its
// new name, and by its old one for a release when only that is set.
func TestRenamed(t *testing.T) {
	t.Setenv("CHANNELS_FROM", "")
	t.Setenv("WEBSITES_FROM", "admin")
	if got := Renamed("CHANNELS_FROM", "WEBSITES_FROM"); got != "admin" {
		t.Errorf("the old name alone: got %q", got)
	}
	t.Setenv("CHANNELS_FROM", "registered")
	if got := Renamed("CHANNELS_FROM", "WEBSITES_FROM"); got != "registered" {
		t.Errorf("the new name first: got %q", got)
	}
	t.Setenv("CHANNEL_FEED_READ_EVERY", "")
	t.Setenv("WEBSITE_READ_EVERY", "45")
	if got := RenamedInt("CHANNEL_FEED_READ_EVERY", "WEBSITE_READ_EVERY", 30); got != 45 {
		t.Errorf("a number by its old name: got %d", got)
	}
}

func TestRenamedString(t *testing.T) {
	t.Setenv("VALKEY_ADDR", "")
	t.Setenv("REDIS_ADDR", "")
	if got := RenamedString("VALKEY_ADDR", "REDIS_ADDR", "localhost:6379"); got != "localhost:6379" {
		t.Errorf("neither: got %q", got)
	}
	t.Setenv("REDIS_ADDR", "redis:6379")
	if got := RenamedString("VALKEY_ADDR", "REDIS_ADDR", "localhost:6379"); got != "redis:6379" {
		t.Errorf("the old name alone: got %q", got)
	}
	t.Setenv("VALKEY_ADDR", "valkey:6379")
	if got := RenamedString("VALKEY_ADDR", "REDIS_ADDR", "localhost:6379"); got != "valkey:6379" {
		t.Errorf("the new name first: got %q", got)
	}
}
