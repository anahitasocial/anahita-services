package services

import (
	"slices"
	"strings"
	"testing"

	"auth/data/entities"

	libs_constants "anahita-libs/constants"
)

// The scope functions under test read only their arguments and the
// package-level catalog, so a zero-value service is sufficient.
func newTestOAuthService() *OAuthService { return &OAuthService{} }

func TestValidateScopes(t *testing.T) {
	s := newTestOAuthService()

	t.Run("known scopes pass", func(t *testing.T) {
		if err := s.ValidateScopes("openid actors:read notifications:write"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty passes", func(t *testing.T) {
		if err := s.ValidateScopes(""); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// The operator editing a client through /clients needs to know
	// WHICH scope was wrong, not just that something was.
	t.Run("unknown scope is named in the error", func(t *testing.T) {
		err := s.ValidateScopes("actors:read bogus:scope")
		if err == nil {
			t.Fatal("expected an error for an unknown scope")
		}
		if !strings.Contains(err.Error(), "bogus:scope") {
			t.Errorf("error %q does not name the offending scope", err)
		}
	})

	// Guards against a regression to the previous vocabulary.
	t.Run("retired names are rejected", func(t *testing.T) {
		for _, old := range []string{"write:media", "admin:actors", "people:read", "read:notifications"} {
			if err := s.ValidateScopes(old); err == nil {
				t.Errorf("retired scope %q was accepted", old)
			}
		}
	})
}

func TestValidateScopesForClient(t *testing.T) {
	s := newTestOAuthService()

	firstParty := &entities.OAuthClient{
		ClientID:   "test-web",
		FirstParty: true,
		Scopes: entities.StringSlice{
			libs_constants.OAuthScopeOpenID,
			libs_constants.OAuthScopeActorsRead,
			libs_constants.OAuthScopeMediaWrite,
		},
	}

	t.Run("registered scopes pass", func(t *testing.T) {
		if err := s.ValidateScopesForClient("openid actors:read", firstParty); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unregistered scope is rejected", func(t *testing.T) {
		if err := s.ValidateScopesForClient("openid oauth:write", firstParty); err == nil {
			t.Error("expected rejection of a scope the client is not registered for")
		}
	})

	// Non-public scopes are first-party only. Every resource scope is
	// non-public today because there are no third-party integrations.
	t.Run("third party cannot request non-public scopes", func(t *testing.T) {
		thirdParty := *firstParty
		thirdParty.FirstParty = false
		if err := s.ValidateScopesForClient("actors:read", &thirdParty); err == nil {
			t.Error("expected a third-party client to be refused a non-public scope")
		}
		if err := s.ValidateScopesForClient("openid", &thirdParty); err != nil {
			t.Errorf("third party should still get OIDC scopes: %v", err)
		}
	})
}

// TestAvailableScopesForConsent covers the silent-downscoping contract.
//
// This is the behaviour the React app depends on: it requests all 19 of
// its registered scopes from every viewer, and expects a NARROWER TOKEN
// rather than an invalid_scope error when the viewer isn't entitled to
// some of them. Break this and login fails for every non-super-admin.
func TestAvailableScopesForConsent(t *testing.T) {
	s := newTestOAuthService()

	client := &entities.OAuthClient{
		ClientID:   "test-web",
		FirstParty: true,
		Scopes: entities.StringSlice{
			libs_constants.OAuthScopeOpenID,
			libs_constants.OAuthScopeActorsRead,
			libs_constants.OAuthScopeNotificationsRead,
			libs_constants.OAuthScopeOAuthRead,
			libs_constants.OAuthScopeOAuthWrite,
		},
	}

	t.Run("registered user is downscoped, not errored", func(t *testing.T) {
		user := &entities.User{UserType: libs_constants.UserTypeRegistered}
		got := s.AvailableScopesForConsent(client, user)

		for _, denied := range []string{libs_constants.OAuthScopeOAuthRead, libs_constants.OAuthScopeOAuthWrite} {
			if slices.Contains(got, denied) {
				t.Errorf("super-admin-only scope %q leaked to a registered user", denied)
			}
		}
		for _, kept := range []string{libs_constants.OAuthScopeOpenID, libs_constants.OAuthScopeActorsRead} {
			if !slices.Contains(got, kept) {
				t.Errorf("scope %q was dropped for a registered user", kept)
			}
		}
	})

	t.Run("super admin keeps everything", func(t *testing.T) {
		user := &entities.User{UserType: libs_constants.UserTypeSuperAdmin}
		if got := s.AvailableScopesForConsent(client, user); len(got) != len(client.Scopes) {
			t.Errorf("super admin got %d scopes, want all %d", len(got), len(client.Scopes))
		}
	})

	// A stale row can carry a scope the catalog no longer defines; that
	// must be skipped, not panic.
	t.Run("unknown scope on the client row is skipped", func(t *testing.T) {
		stale := *client
		stale.Scopes = entities.StringSlice{libs_constants.OAuthScopeOpenID, "retired:scope"}
		got := s.AvailableScopesForConsent(&stale, &entities.User{UserType: libs_constants.UserTypeRegistered})
		if slices.Contains(got, "retired:scope") {
			t.Error("an unknown scope survived consent filtering")
		}
	})
}

func TestNarrowScopes(t *testing.T) {
	original := "openid actors:read media:write"

	t.Run("empty request keeps the original grant", func(t *testing.T) {
		got, err := NarrowScopes(original, "")
		if err != nil || got != original {
			t.Errorf("got (%q, %v), want (%q, nil)", got, err, original)
		}
	})

	t.Run("subset narrows", func(t *testing.T) {
		got, err := NarrowScopes(original, "openid media:write")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "openid media:write" {
			t.Errorf("got %q", got)
		}
	})

	// Refresh must never widen a grant.
	t.Run("scope outside the original grant is rejected", func(t *testing.T) {
		if _, err := NarrowScopes(original, "openid oauth:write"); err == nil {
			t.Error("expected rejection of a scope not in the original grant")
		}
	})
}
