package requests

import (
	"anahita-libs/constants"
	"anahita-libs/policy"
)

type MediumRequest struct {
	Browse            *MediumBrowseRequest
	Read              *MediumReadRequest
	Edit              *MediumEditRequest
	EditAccess        *MediumEditAccessRequest
	EditCommentStatus *MediumEditCommentStatusRequest
	Add               *MediumAddRequest
	Delete            *MediumDeleteRequest
}

func NewMediumRequest() *MediumRequest {
	return &MediumRequest{
		Browse: &MediumBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "created_at",
			OwnerID:   0,
		},
		Read: &MediumReadRequest{
			ID: 0,
		},
		Edit: &MediumEditRequest{
			ID:      0,
			Name:    "",
			Body:    "",
			Excerpt: "",
		},
		EditAccess: &MediumEditAccessRequest{
			ID:     0,
			Access: policy.MinContentAccess(),
		},
		EditCommentStatus: &MediumEditCommentStatusRequest{
			ID:     0,
			Status: false,
		},
		Add: &MediumAddRequest{
			// What a post gets when the client does not say: public, or
			// registered where the installation allows nothing public.
			Access:   policy.MinContentAccess(),
			Composed: false,
		},
		Delete: &MediumDeleteRequest{},
	}
}

type MediumBrowseRequest struct {
	Offset    uint64 `query:"start"`
	Limit     uint64 `query:"limit"`
	Ordering  string `query:"ordering" validate:"max=15,omitempty,oneof=updated_at created_at enabled"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool   `query:"disabled" validate:"omitempty"`
	// OwnerID is the profile whose posts are listed, and is required. There is
	// no list of every post on the site: with the owner optional, and readable
	// from ?oid= as well as the path, leaving it out returned exactly that.
	// Discovery is the job of feeds, search, hashtags and profiles.
	OwnerID int64 `param:"owner_id" validate:"required,min=1"`
}

type MediumReadRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}

type MediumEditRequest struct {
	ID      int64  `param:"id" validate:"required,numeric"`
	Name    string `form:"name" validate:"required,max=100"`
	Body    string `form:"body" validate:"required,max=8000"`
	Excerpt string `form:"excerpt" validate:"omitempty,max=500"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
}

type MediumEditAccessRequest struct {
	ID     int64  `param:"id" validate:"required,numeric"`
	Access string `form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
}

// MediumEditCommentStatusRequest opens or closes a medium to comments. Status
// binds from "1"/"0" as well as "true"/"false", which is what the web client
// sends.
type MediumEditCommentStatusRequest struct {
	ID     int64 `param:"id" validate:"required,numeric"`
	Status bool  `form:"status" validate:"omitempty"`
}

type MediumAddRequest struct {
	OwnerID  int64  `param:"owner_id" validate:"required,numeric"`
	Name     string `form:"name" validate:"required,max=100"`
	Body     string `form:"body" validate:"required,max=8000"`
	Excerpt  string `form:"excerpt" validate:"omitempty,max=500"`
	Access   string `form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	Composed bool   `form:"composed" validate:"omitempty"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
}

type MediumDeleteRequest struct {
	ID int64 `param:"id"`
}
