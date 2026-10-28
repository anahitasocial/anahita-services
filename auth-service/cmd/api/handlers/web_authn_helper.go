package handlers

import (
	"errors"
	"fmt"
	"slices"

	"auth/data/entities"
	"auth/data/services"

	libs_entities "anahita-libs/data/entities"

	"github.com/go-webauthn/webauthn/webauthn"
)

// webAuthnRP builds the relying-party instance for a ceremony.
//
// Package-level rather than a method because both the management
// handler (registration) and the OAuth handler (login) need it, and
// they are different structs that happen to share *Env.
//
// Built per-request rather than cached on Env. The construction is
// cheap — it validates config and stores strings — and rebuilding
// means a config change lands without a restart. If profiling ever
// says otherwise, hoist it into Env at startup.
func webAuthnRP(env *Env) (*webauthn.WebAuthn, error) {
	return services.NewWebAuthn(
		env.Config.GetWebAuthnRPID(),
		webAuthnOrigins(env),
	)
}

// webAuthnOrigins assembles the exact-match origin allowlist.
//
// Ceremonies run in two places: the login page on the auth host, and
// the passkey settings page in whichever SPA the person is using. Both
// have to be listed, because RPOrigins is compared literally — being
// a subdomain of the RP ID is not enough on its own.
//
// GetAllowedAppOrigins is reused rather than introducing a second
// allowlist. It already enumerates the SPA origins for logout
// validation, and two lists that must agree but are maintained
// separately will eventually disagree.
func webAuthnOrigins(env *Env) []string {
	appOrigins := env.Config.GetAllowedAppOrigins()

	origins := make([]string, 0, len(appOrigins)+2)
	origins = append(origins, appOrigins...)

	// The sign-in page runs on the auth host, and on the API's host too: the
	// web app sends people to sign in at its API's address, and a passkey
	// used there was refused as from an origin not listed ("Error validating
	// origin", banu.io, 2026-10-28). On a dev stack both are one address.
	for _, url := range []string{env.Config.GetAuthURL(), env.Config.GetAPIURL()} {
		if url != "" && !slices.Contains(origins, url) {
			origins = append(origins, url)
		}
	}

	return origins
}

// hydrateViewer fills in the viewer's person fields from the database.
//
// The JWT middleware builds a deliberately thin viewer: IDs and scopes
// from the token and nothing else. Every other field reads as its zero
// value — TotpEnabled is false, Type is empty — because none of that is
// carried as a claim.
//
// This is a trap for permission checks. A gate written as
// `if !viewer.TotpEnabled` does not mean "this person has no TOTP", it
// means "this viewer came from a JWT", so it denies everyone
// unconditionally. The symptom is a permission error that flatly
// contradicts the account it describes.
//
// Call this before any permission check that reads person state
// rather than scopes. Scopes and Authenticated live beside the person
// fields on the Viewer, so hydrating leaves them intact.
//
// Returns the loaded User as well, for callers that need methods on it
// directly — IsMfaExpired is on User, not Viewer.
func hydrateViewer(env *Env, viewer *libs_entities.Viewer) (*entities.User, error) {
	if viewer == nil {
		return nil, errors.New("hydrate viewer: nil viewer")
	}

	user, err := env.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		return nil, fmt.Errorf("hydrate viewer: person lookup: %w", err)
	}
	if user == nil {
		return nil, errors.New("hydrate viewer: person not found")
	}

	viewer.ID = user.ID
	viewer.Type = user.Type
	viewer.Email = user.Email

	return user, nil
}
