package handlers

// What some routes take, for the API reference (routes.go), where the
// handler reads each value itself with c.Param or c.QueryParam by the same
// name; nothing binds into these structs.

// PersonRequest names a person.
type PersonRequest struct {
	// The person's id.
	PersonID int64 `param:"person_id" validate:"required,gt=0"`
}

// LinkRequest is the token in a link that was mailed.
type LinkRequest struct {
	// From the link: 128 characters.
	Token string `query:"token"`
}

// LoginPageRequest is the sign-in page, sent back to with why.
type LoginPageRequest struct {
	// Why the page is shown again: `session_expired` or `too_many_attempts`.
	Error string `query:"error"`
}

// LoginCodePageRequest is the page that asks who is signing in with a
// code, sent back to with why.
type LoginCodePageRequest struct {
	// Why the page is shown again: `identifier_required`,
	// `session_expired` or `code_expired`.
	Error string `query:"error"`
}

// ExpiredPageRequest is a page that asks for a new code, sent back to
// because the last one ran out.
type ExpiredPageRequest struct {
	// `expired`, when the code ran out.
	Error string `query:"error"`
}

// LogoutRequest is where to go once signed out.
type LogoutRequest struct {
	// One of the installation's own apps. Anything else, or nothing, is
	// the web app.
	ReturnTo string `query:"return_to"`
}
