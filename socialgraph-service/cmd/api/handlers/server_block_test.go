package handlers

import "testing"

// THE RULE THIS GUARDS (roadmap row 152): whatever names a server, a domain,
// a handle or an address on it, blocks that server.
func TestServerIn(t *testing.T) {
	cases := map[string]string{
		"tycho.example":                      "tycho.example",
		"  Tycho.Example ":                   "tycho.example",
		"naomi@tycho.example":                "tycho.example",
		"@naomi@tycho.example":               "tycho.example",
		"https://Tycho.Example/@naomi":       "tycho.example",
		"https://tycho.example:8443/users/n": "tycho.example",
	}
	for said, want := range cases {
		if got := serverIn(said); got != want {
			t.Errorf("%q: got %q, want %q", said, got, want)
		}
	}
}
