package handlers

import (
	"testing"

	an_constants "anahita-libs/constants"
	"anahita-libs/policy"
)

// A group made without saying who can see it is its administrators' alone,
// as before the form asked; one made public where nothing new may be public
// is refused.
func TestNewGroupAccess(t *testing.T) {
	cases := []struct {
		name       string
		minContent string
		requested  string
		want       string
		allowed    bool
	}{
		{"nothing asked", "public", "", an_constants.AnahitaAccessAdmins, true},
		{"public", "public", "public", an_constants.AnahitaAccessPublic, true},
		{"registered", "public", "registered", an_constants.AnahitaAccessRegistered, true},
		{"followers", "public", "followers", an_constants.AnahitaAccessFollowers, true},
		{"admins", "public", "admins", an_constants.AnahitaAccessAdmins, true},
		{"public where it may not be", "registered", "public", "", false},
		{"registered where public may not be", "registered", "registered", an_constants.AnahitaAccessRegistered, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restore := policy.SetForTest(tc.minContent)
			defer restore()
			got, allowed := newGroupAccess(tc.requested)
			if got != tc.want || allowed != tc.allowed {
				t.Fatalf("newGroupAccess(%q) = %q, %v; want %q, %v", tc.requested, got, allowed, tc.want, tc.allowed)
			}
		})
	}
}
