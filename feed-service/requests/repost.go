package requests

import "anahita-libs/constants"

type RepostRequest struct {
	Browse *RepostBrowseRequest
	Add    *RepostAddRequest
	Delete *RepostDeleteRequest
}

func NewRepostRequest() *RepostRequest {
	return &RepostRequest{
		Browse: &RepostBrowseRequest{
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			OrderingDir: constants.DirDesc,
			Ordering:    constants.OrderingCreatedAt,
			ParentID:    0,
		},
		Add: &RepostAddRequest{
			ParentID: 0,
		},
		Delete: &RepostDeleteRequest{
			ParentID: 0,
		},
	}
}

type RepostBrowseRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// What they are ordered by.
	Ordering string `query:"ordering" validate:"max=15,omitempty,oneof=updated_at created_at enabled"`
	// Which way the order runs.
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	// The id of the post reposted.
	ParentID int64 `param:"parent_id" validate:"required,numeric"`
}

type RepostAddRequest struct {
	// The id of the post to repost.
	ParentID int64 `param:"parent_id" validate:"required,numeric"`
}

type RepostDeleteRequest struct {
	// The id of the post reposted.
	ParentID int64 `param:"parent_id" validate:"required,numeric"`
}
