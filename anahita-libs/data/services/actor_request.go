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
			Ordering:  "modified_on",
			Search:    "",
		},
		BatchRead: &ActorGRPCBatchReadRequest{
			IDs:       []int64{},
			Ordering:  "modified_on",
			Direction: "desc",
		},
	}
}

type ActorGRPCBrowseRequest struct {
	Types     []string `validate:"required,min=1,dive,required"`
	Search    string   `validate:"max=100,omitempty,search_query"`
	Ordering  string   `validate:"max=15,omitempty,oneof=created_on modified_on enabled"`
	Direction string   `validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool     `validate:"omitempty"`
	// Only featured actors. An empty Ordering then means most recently
	// featured first.
	Featured bool `validate:"omitempty"`
	Offset   uint64
	Limit    uint64
}

type ActorGRPCBatchReadRequest struct {
	IDs       []int64 `validate:"required,dive,required,numeric"`
	Ordering  string  `validate:"max=15,omitempty,oneof=created_on modified_on enabled"`
	Direction string  `validate:"max=4,omitempty,oneof=asc desc"`
}
