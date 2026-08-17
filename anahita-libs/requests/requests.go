package requests

type Browse struct {
	Offset   uint64 `query:"offset"   json:"offset"`
	Limit    uint64 `query:"limit"    json:"limit"`
	Keywords string `query:"q"        json:"keywords"`
	Ordering string `query:"ordering" json:"ordering"`
}

func NewBrowse() *Browse {
	return &Browse{
		Offset:   0,
		Limit:    10,
		Keywords: "",
		Ordering: "",
	}
}

type EmailRequest struct {
	Email string `query:"email" form:"email" validate:"min=10,max=100,email,required"`
}
