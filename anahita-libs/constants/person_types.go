package constants

// A person's authority on the installation, stored on nodes.person_type.
//
// Named for the node rather than for auth: the graph owns the person, and
// "user" is reserved for protocol surface that an external specification
// names that way — the OIDC `sub` claim, /oauth/userinfo, and the `user_id`
// columns on the OAuth and WebAuthn tables.
//
// The values are what the database holds and what every issued token already
// carries, so they are fixed. Only the Go identifiers changed.
const (
	PersonTypeSuperAdmin = "super-administrator"
	PersonTypeAdmin      = "administrator"
	PersonTypeRegistered = "registered"
	PersonTypeGuest      = "guest"
)
