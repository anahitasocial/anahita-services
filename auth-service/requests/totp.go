package requests

type TotpRequest struct {
	Delete *TotpDeleteRequest
}

func NewTotpRequest() *TotpRequest {
	return &TotpRequest{
		Delete: &TotpDeleteRequest{},
	}
}

type TotpDeleteRequest struct {
	EmailOrUsername string `form:"identifier" validate:"required,email|username,max=64"`
}

type Totp struct {
	TotpPasscode string `form:"passcode"   validate:"omitempty"`
}
