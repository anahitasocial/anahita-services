package services

import (
	"anahita-libs/constants"
)

type ActorGRPCRequest struct {
	Browse    *ActorGRPCBrowseRequest
	BatchRead *ActorGRPCBatchReadRequest
}

func NewActorGRPCRequest() *ActorGRPCRequest {
	return &ActorGRPCRequest{
		Browse: &ActorGRPCBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "updated_at",
			Search:    "",
		},
		BatchRead: &ActorGRPCBatchReadRequest{
			IDs:       []int64{},
			Ordering:  "updated_at",
			Direction: "desc",
		},
	}
}

type ActorGRPCBrowseRequest struct {
	Types     []string `validate:"required,min=1,dive,required"`
	Search    string   `validate:"max=100,omitempty,search_query"`
	Ordering  string   `validate:"max=15,omitempty,oneof=created_at updated_at enabled"`
	Direction string   `validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool     `validate:"omitempty"`
	// Only featured actors. An empty Ordering then means most recently
	// featured first.
	Featured bool `validate:"omitempty"`
	// Also list the actors the viewer may not see but may ask to follow.
	// They come back marked Restricted, with a name and a picture only.
	IncludeAskable bool
	Offset         uint64
	Limit          uint64
}

type ActorGRPCBatchReadRequest struct {
	IDs       []int64 `validate:"required,dive,required,numeric"`
	Ordering  string  `validate:"max=15,omitempty,oneof=created_at updated_at enabled"`
	Direction string  `validate:"max=4,omitempty,oneof=asc desc"`
}
