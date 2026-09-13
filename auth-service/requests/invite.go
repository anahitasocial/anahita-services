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
// Agreement IS collected here now. The comment that used to stand in this
// place described an SPA interstitial called AgreementsPage that gates on
// AuthenticatedRoute; no such component was ever written, and there was no
// column to record an acceptance in either, so acceptance was captured
// nowhere. It happens at account creation instead — an account should not
// exist without it.
type InviteAccept struct {
	Token string `form:"token"      validate:"required,len=128"`
	// Optional, and offered rather than required.
	//
	// Somebody vouched for this person, so there is nothing for a required
	// name to support — no one is judging the answer. Onboarding asks again a
	// minute later with a better editor, and a required field here would
	// mostly produce names typed to get past it. Empty falls back to the
	// handle, as it does for a self-serve signup.
	Name     string `form:"name" validate:"omitempty,max=100"`
	Password string `form:"password"   validate:"required,min=15,max=150"`

	// See SignupAddRequest for why these are `eq=true` and not `required`.
	TosAccepted bool   `form:"tos_accepted" validate:"eq=true"`
	TosVersion  string `form:"tos_version" validate:"required,max=20"`
	PpAccepted  bool   `form:"pp_accepted" validate:"eq=true"`
	PpVersion   string `form:"pp_version" validate:"required,max=20"`
}

type InviteDelete struct {
	TokenID int64 `json:"token_id" validate:"required"`
}
