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
	//
	// A family covers its sub-resources rather than giving each one its
	// own scope: actors covers avatars, covers, access, features and
	// group admins; media covers photos, articles, notes, topics and
	// their hashtag/location tags. Reads on public content need no scope
	// at all — a read scope gates the authenticated-visibility uplift,
	// not the public surface — which is why taxonomy has no read half.
	OAuthScopeActorsRead         = "actors:read"
	OAuthScopeActorsWrite        = "actors:write"
	OAuthScopeMediaRead          = "media:read"
	OAuthScopeMediaWrite         = "media:write"
	OAuthScopeCommentsWrite      = "comments:write"
	OAuthScopeLikesWrite         = "likes:write"
	OAuthScopeSocialgraphRead    = "socialgraph:read"
	OAuthScopeSocialgraphWrite   = "socialgraph:write"
	OAuthScopeNotificationsRead  = "notifications:read"
	OAuthScopeNotificationsWrite = "notifications:write"
	OAuthScopeTaxonomyWrite      = "taxonomy:write"
	OAuthScopeAccountRead        = "account:read"
	OAuthScopeAccountWrite       = "account:write"
	OAuthScopeOAuthRead          = "oauth:read"
	OAuthScopeOAuthWrite         = "oauth:write"
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
//   - AdminOnly is deliberately unused. Gating a write scope on admin
//     would break self-service — a person editing their own profile
//     needs actors:write — and it duplicates authority at the wrong
//     layer. Role lives in the person_type checks inside handler
//     permissions, per the doctrine above.
//
//   - "Public" is false for every resource scope — Anahita microservices
//     has no third-party integrations yet. When/if we add third-party
//     developer apps, we'd flip the appropriate scopes to Public so
//     external clients can request them.
//
//   - oauth:* is the sole SuperAdminOnly family. Possession of a token
//     carrying oauth:write is equivalent to owning the identity
//     provider: its holder can add a redirect URI to any client and
//     harvest authorization codes for every person on the site. Refusing
//     to mint it for anyone below super_admin is defence in depth on top
//     of the handler role check.
//
// Descriptions are user-facing consent copy. Each one completes the
// sentence "This application will be able to: ..." — see
// auth-service/web/templates/consent.html.
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

	// Actors — people and groups, plus their avatars, covers, privacy
	// settings, features and administrators. Admin authority over
	// someone else's actor is a role check in the handler, not a scope.
	OAuthScopeActorsRead: {
		Scope:       OAuthScopeActorsRead,
		Description: "View people and groups on this site",
	},
	OAuthScopeActorsWrite: {
		Scope:       OAuthScopeActorsWrite,
		Description: "Create and edit people and groups, their avatars, covers, privacy settings, and administrators",
	},

	// Media — photos, articles, notes and topics, plus their covers,
	// privacy settings and hashtag/location tags.
	OAuthScopeMediaRead: {
		Scope:       OAuthScopeMediaRead,
		Description: "View posts, photos, articles, notes, and their comments",
	},
	OAuthScopeMediaWrite: {
		Scope:       OAuthScopeMediaWrite,
		Description: "Create, edit, and delete posts, photos, articles, and notes, and tag them with hashtags and locations",
	},

	// Engaging with content someone else owns is separated from owning
	// it, so a client can be granted one without the other. Reads ride
	// on media:read — there is no comments:read or likes:read.
	OAuthScopeCommentsWrite: {
		Scope:       OAuthScopeCommentsWrite,
		Description: "Write, edit, and delete comments",
	},
	OAuthScopeLikesWrite: {
		Scope:       OAuthScopeLikesWrite,
		Description: "Like and repost content",
	},

	// Social graph — follows, follow requests, blocks and mutuals.
	OAuthScopeSocialgraphRead: {
		Scope:       OAuthScopeSocialgraphRead,
		Description: "View follows, followers, and blocks",
	},
	OAuthScopeSocialgraphWrite: {
		Scope:       OAuthScopeSocialgraphWrite,
		Description: "Follow, unfollow, block, and unblock",
	},

	// Notifications — every person can read and manage their own.
	OAuthScopeNotificationsRead: {
		Scope:       OAuthScopeNotificationsRead,
		Description: "View your notifications",
	},
	OAuthScopeNotificationsWrite: {
		Scope:       OAuthScopeNotificationsWrite,
		Description: "Manage your notifications and subscriptions",
	},

	// Taxonomy — the hashtag and location vocabularies themselves.
	// Tagging content with a term is a media write, not a taxonomy
	// write. Reads are public, so there is no read half.
	OAuthScopeTaxonomyWrite: {
		Scope:       OAuthScopeTaxonomyWrite,
		Description: "Create and edit hashtags and locations",
	},

	// Account — the person's own security surface: password, TOTP,
	// passkeys, and session/auth-log history. Self-service, so no role
	// gate; handlers enforce that a person only touches their own.
	OAuthScopeAccountRead: {
		Scope:       OAuthScopeAccountRead,
		Description: "View your account and security settings",
	},
	OAuthScopeAccountWrite: {
		Scope:       OAuthScopeAccountWrite,
		Description: "Manage your password, two-factor authentication, passkeys, and sessions",
	},

	// OAuth infrastructure — client registrations and signing keys.
	// Super-admin only at mint time; see the note above.
	OAuthScopeOAuthRead: {
		Scope:          OAuthScopeOAuthRead,
		Description:    "View OAuth client registrations and signing keys",
		SuperAdminOnly: true,
	},
	OAuthScopeOAuthWrite: {
		Scope:          OAuthScopeOAuthWrite,
		Description:    "Create, edit, and delete OAuth client registrations and signing keys",
		SuperAdminOnly: true,
	},
}

// OAuthScopeExists reports whether the given scope identifier is known.
func OAuthScopeExists(scope string) bool {
	_, ok := OAuthScopeDefinitions[scope]
	return ok
}
