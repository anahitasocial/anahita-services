package requests

import "anahita-libs/constants"

type LikeRequest struct {
	Browse *LikeBrowseRequest
	Add    *LikeAddRequest
	Delete *LikeDeleteRequest
}

func NewLikeRequest() *LikeRequest {
	return &LikeRequest{
		Browse: &LikeBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "asc",
			Ordering:  "recent",
		},
		Add:    &LikeAddRequest{},
		Delete: &LikeDeleteRequest{},
	}
}

type LikeBrowseRequest struct {
	// The post's or the reply's id.
	NodeID int64 `param:"node_id" validate:"required,number"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// The order they come in.
	Ordering string `query:"sort" validate:"max=10,omitempty,oneof=recent oldest"`
	// Which way the order runs.
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
}

type LikeAddRequest struct {
	// The post's or the reply's id.
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}

type LikeDeleteRequest struct {
	// The post's or the reply's id.
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}
