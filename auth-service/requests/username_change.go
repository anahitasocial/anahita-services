package requests

// UsernameChangeEdit changes the signed-in person's username. Submitted
// as JSON by the SPA from the settings page.
//
// CARRIES NO PROOF OF IDENTITY, deliberately — same contract as
// EmailChangeEdit. Re-authentication happens separately against
// /reauth/*, which records a short-lived marker the handler checks.
//
// min/max mirror the bounds usernames were historically stored under,
// so an existing handle stays valid; `username` is the shared validator in
// anahita-libs, which allows letters, digits, hyphen and underscore
// only. That character set is the impersonation guard: without it a
// handle could carry spaces, dots, or lookalike Unicode and be read as
// somebody else's.
type UsernameChangeEdit struct {
	Username string `json:"username" validate:"required,min=3,max=32,username"`
}
