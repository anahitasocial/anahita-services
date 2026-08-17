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
	NodeID    int64  `param:"node_id" validate:"required,number"`
	Offset    uint64 `query:"start"`
	Limit     uint64 `query:"limit"`
	Ordering  string `query:"sort" validate:"max=10,omitempty,oneof=recent oldest"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
}

type LikeAddRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}

type LikeDeleteRequest struct {
	NodeID int64 `param:"node_id" form:"node_id" validate:"required,number"`
}
