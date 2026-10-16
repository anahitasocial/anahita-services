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
			Ordering:  "created_at",
			OwnerID:   0,
		},
	}
}

type MediaGRPCBrowseRequest struct {
	Types     []string `validate:"required,min=1,dive,required"`
	Offset    uint64   `validate:"omitempty"`
	Limit     uint64   `validate:"max=100,omitempty"`
	Ordering  string   `validate:"max=15,omitempty,oneof=updated_at created_at enabled"`
	Direction string   `validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool     `validate:"omitempty"`
	// Required, for the reason given on requests.MediumBrowseRequest.
	OwnerID int64 `validate:"required,min=1"`
	// Replies are left out of a list of posts unless this is set.
	IncludeReplies bool `validate:"omitempty"`
	// The viewer's own media library: everything of theirs of the kind
	// asked for, with what is in the library only.
	Library bool `validate:"omitempty"`
	// The viewer's own drafts on this profile.
	Drafts bool `validate:"omitempty"`
}
