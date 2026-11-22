package services

import "strings"

// HashToken exposes the package-level HashToken on the service so that
// tests and downstream services can depend on the service interface
// rather than package-level functions.
func (s *OAuthService) HashToken(token string) string {
	return HashToken(token)
}

// scopeSet builds a set from a space-separated scope string for O(1)
// membership checks.
func scopeSet(scopes string) map[string]bool {
	set := make(map[string]bool)
	for _, scope := range strings.Fields(scopes) {
		set[scope] = true
	}
	return set
}
