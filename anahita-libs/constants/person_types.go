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

// IsGivenPersonType reports whether a person may be given this type. A guest
// is nobody signed in, not a type an account has.
func IsGivenPersonType(personType string) bool {
	switch personType {
	case PersonTypeRegistered, PersonTypeAdmin, PersonTypeSuperAdmin:
		return true
	}
	return false
}

// MayChangePersonType reports whether somebody of viewerType may change a
// person from one type to another (the user, 2026-10-18).
//
// What sets a super administrator apart is the installation's configuration
// and its system tasks, so an administrator may make somebody registered an
// administrator, and nothing else: making a super administrator, and taking
// a type away from anybody, are a super administrator's. Nobody changes
// their own type, which the caller checks, having the ids: it is what keeps
// the last super administrator.
func MayChangePersonType(viewerType, from, to string) bool {
	if from == to || !IsGivenPersonType(from) || !IsGivenPersonType(to) {
		return false
	}
	switch viewerType {
	case PersonTypeSuperAdmin:
		return true
	case PersonTypeAdmin:
		return from == PersonTypeRegistered && to == PersonTypeAdmin
	}
	return false
}
