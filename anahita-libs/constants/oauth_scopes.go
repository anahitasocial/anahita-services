package constants

// OAuth scope identifiers. Scopes are published via the Anahita microservices
// OAuth discovery endpoint and declared on each oauth_client row.
//
// Naming convention: lowercase, colon-separated.
//
//	resource:action  — read/write on a resource family
//	category         — OIDC standard scopes (openid, profile, email)
//
// Scopes describe API surface (what the client may call). User
// authority (admin / super_admin) is enforced separately
// via person_type and tenancy checks in handler permissions.
const (
	// OIDC standard scopes.
	OAuthScopeOpenID        = "openid"
	OAuthScopeProfile       = "profile"
	OAuthScopeEmail         = "email"
	OAuthScopeOfflineAccess = "offline_access"

	// Resource scopes — read/write per resource family.
	OAuthScopePeopleRead            = "people:read"
	OAuthScopePeopleWrite           = "people:write"
	OAuthScopeNotificationsRead     = "notifications:read"
	OAuthScopeNotificationsWrite    = "notifications:write"
	OAuthScopeAuthLogsRead          = "authlogs:read"
	OAuthScopeAuthLogsWrite         = "authlogs:write"
	OAuthScopeOAuthClientsRead      = "oauth_clients:read"
	OAuthScopeOAuthClientsWrite     = "oauth_clients:write"
	OAuthScopeOAuthSigningKeysRead  = "oauth_signing_keys:read"
	OAuthScopeOAuthSigningKeysWrite = "oauth_signing_keys:write"
	OAuthScopeWebAuthnRead          = "webauthn:read"
	OAuthScopeWebAuthnWrite         = "webauthn:write"
)

// OAuthScopeDefinition describes a single scope: who can grant it, who
// can see it on the consent screen, and the human-readable label shown
// to users.
//
// The flags here are about token issuance, not handler authorization.
// Handlers do their own role + tenancy checks per resource. The flags
// answer: "should auth-service refuse to mint a token containing this
// scope for this client/person at all?"
type OAuthScopeDefinition struct {
	// Scope is the scope identifier string.
	Scope string

	// Description is shown on the consent screen.
	Description string

	// Public means third-party clients may request this scope.
	// Non-public scopes are only grantable to first-party clients.
	Public bool

	// AdminOnly means the authenticated person must be admin or
	// super_admin for auth-service to issue a token with this scope.
	// Handlers may impose additional checks (e.g., tenancy).
	AdminOnly bool

	// SuperAdminOnly means the authenticated person must be
	// super_admin for auth-service to issue a token with this scope.
	SuperAdminOnly bool
}

// OAuthScopeDefinitions is the canonical scope registry for Anahita
// microservices. Source of truth for discovery endpoints, consent UI, and
// scope validation.
//
// Notes on flag choices:
//
//   - Most resource scopes have no role flag because the per-resource
//     handlers enforce role and tenancy. The OAuth layer only gates
//     scopes whose mere presence implies elevated authority (billing,
//     oauth_clients).
//
//   - "Public" is currently false for all resource scopes — Anahita
//     microservices has no third-party integrations yet. When/if we add
//     third-party developer apps, we'd flip the appropriate scopes
//     to Public so external clients can request them.
//
//   - authlogs scopes are not admin-gated: every user can view and
//     revoke their own session log. Handlers enforce that they only
//     touch their own log entries unless they're admin+.
var OAuthScopeDefinitions = map[string]OAuthScopeDefinition{
	// OIDC standard — always public, no role gating.
	OAuthScopeOpenID: {
		Scope:       OAuthScopeOpenID,
		Description: "Verify your identity",
		Public:      true,
	},
	OAuthScopeProfile: {
		Scope:       OAuthScopeProfile,
		Description: "View your profile information (name, avatar)",
		Public:      true,
	},
	OAuthScopeEmail: {
		Scope:       OAuthScopeEmail,
		Description: "View your email address",
		Public:      true,
	},
	OAuthScopeOfflineAccess: {
		Scope:       OAuthScopeOfflineAccess,
		Description: "Maintain access while you are away",
		Public:      true,
	},

	// People — admin-gated; non-admins can't manage members.
	OAuthScopePeopleRead: {
		Scope:       OAuthScopePeopleRead,
		Description: "View people on this site",
		AdminOnly:   false,
	},
	OAuthScopePeopleWrite: {
		Scope:       OAuthScopePeopleWrite,
		Description: "Invite, edit, and disable people on this site",
		AdminOnly:   true,
	},

	// Notifications — every user can read/manage their own.
	OAuthScopeNotificationsRead: {
		Scope:       OAuthScopeNotificationsRead,
		Description: "View your notifications",
	},
	OAuthScopeNotificationsWrite: {
		Scope:       OAuthScopeNotificationsWrite,
		Description: "Mark notifications as read or dismiss them",
	},

	// Auth logs — every user can view and revoke their own sessions.
	// Handlers enforce that they only touch their own entries unless
	// they're admin+.
	OAuthScopeAuthLogsRead: {
		Scope:       OAuthScopeAuthLogsRead,
		Description: "View authentication and session activity",
	},
	OAuthScopeAuthLogsWrite: {
		Scope:       OAuthScopeAuthLogsWrite,
		Description: "Revoke active sessions",
	},

	// Passkeys — every person manages their own, so no role gate.
	// Same reasoning as auth logs: this is a person acting on their
	// own credentials, not on anyone else's. There is deliberately no
	// admin path at all — permissions/webauthn.go checks ownership
	// against the credential row and has no admin branch, because an
	// admin has no legitimate reason to enumerate which devices a
	// person carries.
	OAuthScopeWebAuthnRead: {
		Scope:       OAuthScopeWebAuthnRead,
		Description: "View the passkeys on your account",
	},
	OAuthScopeWebAuthnWrite: {
		Scope:       OAuthScopeWebAuthnWrite,
		Description: "Add and remove passkeys on your account",
	},

	// OAuth clients — global super-admin only. Handlers further
	// restrict to global admins; this gate just refuses to mint
	// the scope for anyone who isn't at least a super_admin.
	OAuthScopeOAuthClientsRead: {
		Scope:          OAuthScopeOAuthClientsRead,
		Description:    "View OAuth client registrations",
		SuperAdminOnly: true,
	},
	OAuthScopeOAuthClientsWrite: {
		Scope:          OAuthScopeOAuthClientsWrite,
		Description:    "Create, edit, and delete OAuth client registrations",
		SuperAdminOnly: true,
	},

	// OAuth signing keys — global super-admin only. These are used to sign
	// tokens across all orgs, so we gate them at the highest level. Handlers
	// further restrict to global admins; this gate just refuses to mint
	// the scope for anyone who isn't at least a super_admin.
	OAuthScopeOAuthSigningKeysRead: {
		Scope:          OAuthScopeOAuthSigningKeysRead,
		Description:    "View OAuth signing keys",
		SuperAdminOnly: true,
	},
	OAuthScopeOAuthSigningKeysWrite: {
		Scope:          OAuthScopeOAuthSigningKeysWrite,
		Description:    "Create and delete OAuth signing keys",
		SuperAdminOnly: true,
	},
}

// OAuthScopeExists reports whether the given scope identifier is known.
func OAuthScopeExists(scope string) bool {
	_, ok := OAuthScopeDefinitions[scope]
	return ok
}
