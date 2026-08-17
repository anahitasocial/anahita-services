package requests

type Identifier struct {
	Identifier string `form:"identifier" validate:"min=10,max=100,email,required"`
}
