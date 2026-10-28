package handlers

import (
	"slices"
	"testing"

	"auth/config"
)

// A passkey is accepted from the pages that use one: the web app, where it
// is made, and the sign-in page, which runs on the auth host and on the
// API's, where the web app sends people to sign in. banu.io's were refused
// from https://api.banu.io until it was listed (2026-10-28).
func TestPasskeysAreAcceptedWhereTheyAreUsed(t *testing.T) {
	env := &Env{Config: &config.Config{Env: "prod", DomainName: "banu.io"}}
	origins := webAuthnOrigins(env)

	for _, want := range []string{"https://banu.io", "https://auth.banu.io", "https://api.banu.io"} {
		if !slices.Contains(origins, want) {
			t.Errorf("%s is not among the origins a passkey is accepted from: %v", want, origins)
		}
	}

	// On a dev stack the auth host and the API are one address: listed once.
	dev := webAuthnOrigins(&Env{Config: &config.Config{Env: "dev"}})
	seen := map[string]int{}
	for _, origin := range dev {
		seen[origin]++
	}
	for origin, n := range seen {
		if n > 1 {
			t.Errorf("%s is listed %d times", origin, n)
		}
	}
}
