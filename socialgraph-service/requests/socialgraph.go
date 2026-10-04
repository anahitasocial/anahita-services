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
	Offset    uint64 `query:"start"`
	Limit     uint64 `query:"limit"`
	Ordering  string `query:"ordering" validate:"max=15,omitempty,oneof=created_at updated_at"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Search    string `query:"q" validate:"max=100,omitempty,name_search_query"`
	ActorID   int64  `param:"actor_id" validate:"required,numeric"`
}

type SocialgraphEditRequest struct {
	TargetID int64 `param:"target_id" validate:"required,numeric"`
	SourceID int64 `param:"source_id" validate:"required,numeric"`
}

type SocialgraphAddRequest struct {
	TargetID int64 `param:"target_id" validate:"required,numeric"`
	SourceID int64 `param:"source_id" validate:"required,numeric"`
}

type SocialgraphDeleteRequest struct {
	TargetID int64 `param:"target_id" validate:"required,numeric"`
	SourceID int64 `param:"source_id" validate:"required,numeric"`
}
