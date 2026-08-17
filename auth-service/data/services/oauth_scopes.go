package services

import (
	"fmt"
	"strings"

	"auth/data/entities"

	libs_constants "anahita-libs/constants"
)

// ValidateScopes checks that all provided scopes are registered in the
// Anahita scope catalog.
func (s *OAuthService) ValidateScopes(scopes string) error {
	for _, scope := range strings.Fields(scopes) {
		if !libs_constants.OAuthScopeExists(scope) {
			return fmt.Errorf("unknown scope: %s", scope)
		}
	}
	return nil
}

// ValidateScopesForClient checks that all provided scopes are allowed for
// the given client. Third-party (non-first-party) clients may only
// request public scopes.
func (s *OAuthService) ValidateScopesForClient(scopes string, client *entities.OAuthClient) error {
	for _, scope := range strings.Fields(scopes) {
		def, ok := libs_constants.OAuthScopeDefinitions[scope]
		if !ok {
			return fmt.Errorf("unknown scope: %s", scope)
		}

		if !client.AllowsScope(scope) {
			return fmt.Errorf("scope %s not allowed for client %s", scope, client.ClientID)
		}

		if !client.FirstParty && !def.Public {
			return fmt.Errorf("scope %s not available to third-party clients", scope)
		}
	}
	return nil
}

// Note: there is deliberately no ValidateScopesForPerson. Privilege
// filtering happens in AvailableScopesForConsent below, which SILENTLY
// DROPS what the viewer may not grant rather than erroring. That is what
// lets a first-party app request its full scope list from every viewer
// and receive a narrower token instead of invalid_scope. A second
// function with the opposite semantics sitting next to it is a loaded
// gun — if a future non-interactive grant needs strict validation, add
// it back deliberately.

// ScopeDescriptions returns human-readable descriptions for display on
// the consent screen. Unknown scopes are echoed through unchanged.
func (s *OAuthService) ScopeDescriptions(scopes []string) []string {
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if def, ok := libs_constants.OAuthScopeDefinitions[scope]; ok {
			result = append(result, def.Description)
		} else {
			result = append(result, scope)
		}
	}
	return result
}

// AvailableScopesForConsent returns the subset of a client's allowed
// scopes that the authenticated person has the privileges to grant.
// Used to populate the consent screen.
func (s *OAuthService) AvailableScopesForConsent(client *entities.OAuthClient, user *entities.User) []string {
	available := make([]string, 0, len(client.Scopes))
	for _, scope := range client.Scopes {
		def, ok := libs_constants.OAuthScopeDefinitions[scope]
		if !ok {
			continue
		}
		if def.SuperAdminOnly && !user.IsSuperAdmin() {
			continue
		}
		if def.AdminOnly && !user.IsAdminOrSuperAdmin() {
			continue
		}
		if !client.FirstParty && !def.Public {
			continue
		}
		available = append(available, scope)
	}
	return available
}

// NarrowScopes returns the requested scopes intersected with the
// originally-granted scopes. Used during refresh token exchange when the
// client requests a narrower scope than was originally granted.
//
// Returns an error if any requested scope was not in the original grant.
func NarrowScopes(originalScopes, requestedScopes string) (string, error) {
	if requestedScopes == "" {
		return originalScopes, nil
	}

	original := scopeSet(originalScopes)
	narrowed := make([]string, 0)
	for _, scope := range strings.Fields(requestedScopes) {
		if !original[scope] {
			return "", fmt.Errorf("scope %s was not in original grant", scope)
		}
		narrowed = append(narrowed, scope)
	}
	return strings.Join(narrowed, " "), nil
}
