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
// token, not something the user provides. No agreement fields — ToS
// and Privacy acceptance happens in the SPA via the post-login
// AgreementsPage interstitial.
type InviteAccept struct {
	Token string `form:"token"      validate:"required,len=128"`
	// One name, free-form, matching the profile edit request. The pair it
	// replaces each capped at 50 and rejected the space between them.
	Name     string `form:"name" validate:"required,min=1,max=100"`
	Password string `form:"password"   validate:"required,min=15,max=150"`
}

type InviteDelete struct {
	TokenID int64 `json:"token_id" validate:"required"`
}
