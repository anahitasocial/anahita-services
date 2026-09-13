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
}
