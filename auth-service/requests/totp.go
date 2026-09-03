package requests

type TotpRequest struct {
	Delete *TotpDeleteRequest
}

func NewTotpRequest() *TotpRequest {
	return &TotpRequest{
		Delete: &TotpDeleteRequest{},
	}
}

// TotpDeleteRequest disables 2FA on one account.
//
// Bound from the PATH, matching the route (DELETE /totp/:person_id). It used
// to read an `identifier` form field, which no DELETE from the app ever sent —
// so every disable failed validation with a 400 before reaching the handler.
type TotpDeleteRequest struct {
	PersonID int64 `param:"person_id" validate:"required,numeric"`
}

type Totp struct {
	TotpPasscode string `form:"passcode"   validate:"omitempty"`
}
