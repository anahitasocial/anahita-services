package requests

// InviteAdd is submitted as JSON by the Anahita web app when an admin
// creates an invite. The recipient's email is the only field needed;
// the inviter is derived from the authenticated viewer.
type InviteAdd struct {
	Email string `json:"email" validate:"required,email,min=10,max=100"`
}

// InviteAccept is submitted from the server-rendered invite-accept
// form after the user clicks the email link. The token is carried in
// a hidden field on the form and re-validated server-side on submit.
//
// No email field — the email is the recipient stored on the invite
// token, not something the user provides.
//
// NO AGREEMENT FIELDS, matching signup.
//
// Acceptance is collected after signing in and checked on every sign-in, so a
// revision re-prompts by itself. Account creation happens once; the terms
// change more than once. The form shows them and links them, so nobody can say
// they were never told — it just does not ask for a tick here.
type InviteAccept struct {
	Token string `form:"token"      validate:"required,len=128"`

	// The username, chosen rather than derived.
	//
	// It used to be taken from the email local part and never shown. Two
	// people invited as alice@ at different domains both wanted "alice", the
	// second was refused with "could not create your account, please try
	// again", and trying again derived the same name — a permanent failure
	// with nothing the person could do about it.
	//
	// It is also the account's address on the network, which is not a thing to
	// assign somebody silently. The form suggests the local part, so the
	// common case is still one glance and a click.
	Username string `form:"username" validate:"required,username,min=3,max=32"`

	// Optional, and offered rather than required.
	//
	// Somebody vouched for this person, so there is nothing for a required
	// name to support — no one is judging the answer. Onboarding asks again a
	// minute later with a better editor, and a required field here would
	// mostly produce names typed to get past it. Empty falls back to the
	// username, as it does for a self-serve signup.
	Name     string `form:"name" validate:"omitempty,max=100"`
	Password string `form:"password"   validate:"required,min=15,max=150"`
}

type InviteDelete struct {
	TokenID int64 `json:"token_id" validate:"required"`
}
