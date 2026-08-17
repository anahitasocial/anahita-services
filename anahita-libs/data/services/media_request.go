package services

import (
	"anahita-libs/constants"
)

type MediaGRPCRequest struct {
	Browse *MediaGRPCBrowseRequest
}

func NewMediaGRPCRequest() *MediaGRPCRequest {
	return &MediaGRPCRequest{
		Browse: &MediaGRPCBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "created_on",
			OwnerID:   0,
		},
	}
}

type MediaGRPCBrowseRequest struct {
	Types     []string `validate:"required,min=1,dive,required"`
	Offset    uint64   `validate:"omitempty"`
	Limit     uint64   `validate:"max=100,omitempty"`
	Ordering  string   `validate:"max=15,omitempty,oneof=modified_on created_on enabled"`
	Direction string   `validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool     `validate:"omitempty"`
	OwnerID   int64    `validate:"omitempty,numeric"`
}
