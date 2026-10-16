package requests

// TotpPassword begins setting up two-factor authentication: the password,
// to prove who is asking.
type TotpPassword struct {
	// The viewer's password.
	Password string `form:"password" validate:"min=15,max=150,required"`
}
