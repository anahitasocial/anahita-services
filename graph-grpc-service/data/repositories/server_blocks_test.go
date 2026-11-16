package repositories

import "testing"

// THE RULE THIS GUARDS (roadmap row 152): a blocked server is the server
// and every one under it, whatever port an address carries, and nothing
// that merely ends the same way.
func TestOnBlockedServer(t *testing.T) {
	blocked := []string{"tycho.example", "ceres.example"}
	cases := []struct {
		origin string
		want   bool
	}{
		{"tycho.example", true},
		{"Tycho.Example", true},
		{"tycho.example:8443", true},
		{"media.tycho.example", true},
		{"notycho.example", false},
		{"tycho.example.net", false},
		{"", false},
		{"eros.example", false},
	}
	for _, c := range cases {
		if got := OnBlockedServer(c.origin, blocked); got != c.want {
			t.Errorf("%q: got %v, want %v", c.origin, got, c.want)
		}
	}
}
