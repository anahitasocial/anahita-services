package requests

import "anahita-libs/constants"

type SocialgraphRequest struct {
	Browse *SocialgraphBrowseRequest
	Edit   *SocialgraphEditRequest
	Add    *SocialgraphAddRequest
	Delete *SocialgraphDeleteRequest
}

func NewSocialgraphRequest() *SocialgraphRequest {
	return &SocialgraphRequest{
		Browse: &SocialgraphBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "updated_at",
			Search:    "",
		},
		Edit:   &SocialgraphEditRequest{},
		Add:    &SocialgraphAddRequest{},
		Delete: &SocialgraphDeleteRequest{},
	}
}

type SocialgraphBrowseRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// What they are ordered by.
	Ordering string `query:"ordering" validate:"max=15,omitempty,oneof=created_at updated_at"`
	// Which way the order runs.
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	// Only those whose name has these words.
	Search string `query:"q" validate:"max=100,omitempty,name_search_query"`
	// The profile's id.
	ActorID int64 `param:"actor_id" validate:"required,numeric"`
}

type SocialgraphEditRequest struct {
	// The id of the profile followed, asked or blocked.
	TargetID int64 `param:"target_id" validate:"required,numeric"`
	// The id of whoever follows, asks or blocks: the viewer, or a group they
	// run.
	SourceID int64 `param:"source_id" validate:"required,numeric"`
}

type SocialgraphAddRequest struct {
	// The id of the profile followed, asked or blocked.
	TargetID int64 `param:"target_id" validate:"required,numeric"`
	// The id of whoever follows, asks or blocks: the viewer, or a group they
	// run.
	SourceID int64 `param:"source_id" validate:"required,numeric"`
}

type SocialgraphDeleteRequest struct {
	// The id of the profile followed, asked or blocked.
	TargetID int64 `param:"target_id" validate:"required,numeric"`
	// The id of whoever follows, asks or blocks: the viewer, or a group they
	// run.
	SourceID int64 `param:"source_id" validate:"required,numeric"`
}
