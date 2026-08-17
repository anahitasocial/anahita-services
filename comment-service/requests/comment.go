package requests

import "anahita-libs/constants"

type CommentRequest struct {
	Browse *CommentBrowseRequest
	Edit   *CommentEditRequest
	Add    *CommentAddRequest
	Delete *CommentDeleteRequest
}

func NewCommentRequest() *CommentRequest {
	return &CommentRequest{
		Browse: &CommentBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "asc",
			Ordering:  "recent",
		},
		Edit: &CommentEditRequest{
			ID:   0,
			Body: "",
		},
		Add: &CommentAddRequest{
			ParentID: 0,
			Body:     "",
		},
		Delete: &CommentDeleteRequest{
			ID: 0,
		},
	}
}

type CommentBrowseRequest struct {
	ParentID  int64  `param:"parent_id" validate:"required,number"`
	Offset    uint64 `query:"start"`
	Limit     uint64 `query:"limit"`
	Ordering  string `query:"sort" validate:"max=10,omitempty,oneof=created_on"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
}

type CommentEditRequest struct {
	ID   int64  `param:"id" validate:"required,number"`
	Body string `form:"body" validate:"required,max=8000"`
}

type CommentAddRequest struct {
	ParentID int64  `param:"parent_id" form:"parent_id" validate:"required,number"`
	Body     string `form:"body" validate:"required,max=8000"`
}

type CommentDeleteRequest struct {
	ID       int64 `param:"id" form:"id" validate:"required,number"`
	ParentID int64 `param:"parent_id" form:"parent_id" validate:"required,number"`
}
