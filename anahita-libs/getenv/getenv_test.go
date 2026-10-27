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
