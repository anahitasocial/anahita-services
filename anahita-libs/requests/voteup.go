package requests

import "anahita-libs/constants"

type VoteupRequest struct {
	Browse *VoteupBrowseRequest
}

func NewVoteupRequest() *VoteupRequest {
	return &VoteupRequest{
		Browse: &VoteupBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "asc",
			Ordering:  "newest",
		},
	}
}

type VoteupBrowseRequest struct {
	VotableID int64  `param:"votable_id" validate:"required,number"`
	Offset    uint64 `query:"start"`
	Limit     uint64 `query:"limit"`
	Ordering  string `query:"sort" validate:"max=10,omitempty,oneof=newest oldest"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
}
