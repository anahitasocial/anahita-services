package requests

type TotpPassword struct {
	Token    string `form:"token"    validate:"omitempty,len=128"`
	Password string `form:"password" validate:"min=15,max=150,required"`
}
