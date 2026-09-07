package requests

// Browse is the shared paging request.
//
// Ordering has no oneof rule, and cannot have one here: the valid sort columns
// differ per resource, so the list belongs to each service's own request
// struct (see auth-service's UserBrowseRequest, which does carry one). What it
// does carry is a length bound, so a caller that forgets to validate cannot be
// handed an arbitrarily long string.
//
// A repository that sorts by this value MUST pass it through its own
// safeOrderColumn first. ORDER BY is concatenated rather than bound, so a
// repository accepting this field unchecked is an injection. The one caller
// today (auth-service's tokens repo) hardcodes its ORDER BY and never reads
// the field.
type Browse struct {
	Offset   uint64 `query:"offset"   json:"offset"`
	Limit    uint64 `query:"limit"    json:"limit"   validate:"omitempty,max=100"`
	Keywords string `query:"q"        json:"keywords" validate:"omitempty,max=100"`
	Ordering string `query:"ordering" json:"ordering" validate:"omitempty,max=32"`
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
