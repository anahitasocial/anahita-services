package requests

import "anahita-libs/constants"

type InboundRequest struct {
	Browse *InboundBrowseRequest
	Add    *InboundAddRequest
	Delete *InboundDeleteRequest
}

func NewInboundRequest() *InboundRequest {
	return &InboundRequest{
		Browse: &InboundBrowseRequest{
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			OrderingDir: constants.DirDesc,
			Ordering:    constants.OrderingRecent,
		},
		Add:    &InboundAddRequest{},
		Delete: &InboundDeleteRequest{},
	}
}

type InboundBrowseRequest struct {
	// What the posts are tied to: a hashtag's or a place's id.
	SourceID int64 `param:"id" validate:"number,omitempty"`
	// The profile whose posts alone are wanted, where a route takes one.
	OwnerID int64 `param:"owner_id" validate:"numeric,omitempty"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// The order they come in.
	Ordering string `query:"sort" validate:"max=10,omitempty,oneof=created_at updated_at recent popular"`
	// Which way the order runs.
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	// Set by the service, not read from the request.
	IDs         []int64
	AccessCheck bool
}

type InboundAddRequest struct {
	// The id of the hashtag or the place.
	TargetID int64 `param:"id" validate:"required,number"`
	// The id of the post to tie to it.
	SourceID int64 `form:"source_id" validate:"required,number"`
}

type InboundDeleteRequest struct {
	// The id of the hashtag or the place.
	TargetID int64 `param:"id" validate:"required,number"`
	// The id of the post to untie from it.
	SourceID int64 `param:"source_id" validate:"required,number"`
}
