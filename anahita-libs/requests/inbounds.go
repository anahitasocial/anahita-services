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
	SourceID    int64  `param:"id" validate:"number,omitempty"`
	OwnerID     int64  `param:"owner_id" validate:"numeric,omitempty"`
	Offset      uint64 `query:"start"`
	Limit       uint64 `query:"limit"`
	Ordering    string `query:"sort" validate:"max=10,omitempty,oneof=created_on modified_on recent popular"`
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	IDs         []int64
	AccessCheck bool
}

type InboundAddRequest struct {
	TargetID int64 `param:"id" validate:"required,number"`
	SourceID int64 `form:"source_id" validate:"required,number"`
}

type InboundDeleteRequest struct {
	TargetID int64 `param:"id" validate:"required,number"`
	SourceID int64 `param:"source_id" validate:"required,number"`
}
