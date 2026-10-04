package services

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

type RepostRequest struct {
	Browse        *RepostBrowseRequest
	BrowseAuthors *RepostBrowseAuthorsRequest
	Read          *RepostReadRequest
	Add           *RepostAddRequest
	Delete        *RepostDeleteRequest
	Count         *RepostCountRequest
}

func NewRepostRequest() *RepostRequest {
	return &RepostRequest{
		Browse: &RepostBrowseRequest{
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			OrderingDir: constants.DirDesc,
			Ordering:    constants.OrderingCreatedAt,
		},
		BrowseAuthors: &RepostBrowseAuthorsRequest{
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			OrderingDir: constants.DirDesc,
			Ordering:    constants.OrderingCreatedAt,
		},
		Read:   &RepostReadRequest{},
		Add:    &RepostAddRequest{},
		Delete: &RepostDeleteRequest{},
		Count:  &RepostCountRequest{},
	}
}

type RepostBrowseRequest struct {
	OwnerID     int64  `query:"owner_id" validate:"omitempty,numeric"`
	Offset      uint64 `query:"start"`
	Limit       uint64 `query:"limit"`
	Ordering    string `query:"ordering" validate:"max=15,omitempty,oneof=updated_at created_at enabled"`
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Viewer      *entities.Viewer
}

type RepostBrowseAuthorsRequest struct {
	ParentID    int64  `param:"parent_id" validate:"required,numeric"`
	Offset      uint64 `query:"start"`
	Limit       uint64 `query:"limit"`
	Ordering    string `query:"ordering" validate:"max=15,omitempty,oneof=updated_at created_at enabled"`
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Viewer      *entities.Viewer
}

type RepostReadRequest struct {
	ParentID int64 `param:"parent_id" validate:"required,numeric"`
	AuthorID int64 `param:"author_id" validate:"required,numeric"`
}

type RepostAddRequest struct {
	OwnerID  int64  `query:"owner_id" validate:"omitempty,numeric"`
	ParentID int64  `param:"parent_id" validate:"required,numeric"`
	Service  string `query:"service" validate:"required,oneof=anahita facebook twitter linkedin pinterest tumblr"`
	Access   string `query:"access" validate:"omitempty,oneof=public private friends only_me"`
	Viewer   *entities.Viewer
}

type RepostDeleteRequest struct {
	ParentID int64 `param:"parent_id" validate:"required,numeric"`
	AuthorID int64 `param:"author_id" validate:"required,numeric"`
}

type RepostCountRequest struct {
	OwnerID int64 `query:"owner_id" validate:"omitempty,numeric"`
	Viewer  *entities.Viewer
}
