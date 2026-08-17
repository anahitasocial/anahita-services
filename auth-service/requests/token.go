package requests

type Token struct {
	Body string `form:"token" query:"token" validate:"required,alphanum"`
}
