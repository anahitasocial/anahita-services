package requests

type SignupRequest struct {
	Add *SignupAddRequest
}

func NewSignupRequest() *SignupRequest {
	return &SignupRequest{
		Add: &SignupAddRequest{},
	}
}

// SignupAddRequest is the server-rendered signup form.
//
// Three fields, which is what Mastodon asks for and one fewer than Bluesky.
// Everything else a profile could carry is offered by onboarding afterwards,
// where it is skippable and the editor is better than a plain form.
//
// NO DISPLAY NAME. nodes.name is left empty and rendering falls back to the
// handle — which is what both federation protocols expect, and what lets
// onboarding tell "never set one" from "chose their handle".
//
// NO AGREEMENT FIELDS. Acceptance is collected after sign-in and checked on
// every sign-in, so that a revision re-prompts by itself. Account creation
// happens once; the terms change more than once.
type SignupAddRequest struct {
	// The CSRF token, matched against the one put in the session when the
	// form was rendered.
	CSRFToken string `form:"csrf_token" validate:"required"`

	Email    string `form:"email" validate:"required,email,min=3,max=64"`
	Username string `form:"username" validate:"required,username,min=3,max=32"`
	Password string `form:"password" validate:"required,min=15,max=150"`

	// Collected in `approval` mode only, and required there.
	//
	// A human is being asked to decide, and deciding on an email address alone
	// is not deciding. Mastodon asks "why do you want to join?" and throws the
	// answer away after; these three are the profile itself, so nobody
	// rejected has written anything they would not have written anyway.
	//
	// `omitempty` because the same struct binds both forms — the handler
	// requires them when the mode does, which is the only place that knows.
	Name       string `form:"name" validate:"omitempty,max=100"`
	Body       string `form:"body" validate:"omitempty,max=8000"`
	WebsiteURL string `form:"website_url" validate:"omitempty,max=255"`
}

// UsernameAvailability backs GET /auth/is/username, the check the signup form
// makes as the field is left.
//
// The same rules the signup request applies, so the two cannot disagree — an
// endpoint that reported a username available and a form that then refused it
// would be worse than no check at all.
type UsernameAvailability struct {
	Username string `query:"username" validate:"required,username,min=3,max=32"`
}
