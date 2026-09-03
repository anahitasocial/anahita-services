package requests

// ReauthPassword is submitted as JSON by the step-up dialog when
// re-authenticating with a password rather than a passkey.
//
// JSON tags, not form tags: this is an API endpoint called by the SPA,
// like PasswordEdit and unlike the server-rendered password forms.
//
// Passcode is optional HERE and required by the HANDLER when the
// account has TOTP enrolled. The split is deliberate — whether a
// second factor is needed is a property of the account, not of the
// payload, and a `required` tag would let a caller who simply omits
// the field get a 400 that says "bad request" instead of a 403 that
// says "your code was wrong". The client cannot be trusted to declare
// whether it should have sent one.
//
// No minimum length on Password. The rule that matters is whether it
// matches the stored hash, and a length floor here would reject a
// legitimate older password that predates the current policy — which
// would lock exactly the people this endpoint exists to let through.
type ReauthPassword struct {
	Password string `json:"password" validate:"required,max=150"`
	Passcode string `json:"passcode" validate:"omitempty,max=64"`
}
