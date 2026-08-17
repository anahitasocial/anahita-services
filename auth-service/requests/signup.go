package requests

type SignupRequest struct {
	Add *SignupAddRequest
}

func NewSignupRequest() *SignupRequest {
	return &SignupRequest{
		Add: &SignupAddRequest{},
	}
}

type SignupAddRequest struct {
	Email    string `form:"email" validate:"required,email,min=3,max=64"`
	Username string `form:"username" validate:"required,username,min=3,max=32"`
	Password string `form:"password" validate:"required,min=6,max=32"`
}
