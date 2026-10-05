package requests

import (
	"anahita-libs/constants"
	"anahita-libs/policy"
	"anahita-libs/requests"
)

type NoteRequest struct {
	Browse     *requests.MediumBrowseRequest
	Read       *requests.MediumReadRequest
	Edit       *NoteEditRequest
	EditAccess *NoteEditAccessRequest
	Add        *NoteAddRequest
	Delete     *requests.MediumDeleteRequest
}

func NewNoteRequest() *NoteRequest {
	return &NoteRequest{
		Browse: &requests.MediumBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "created_at",
			OwnerID:   0,
		},
		Read: &requests.MediumReadRequest{
			ID: 0,
		},
		Edit: &NoteEditRequest{
			ID:   0,
			Body: "",
		},
		EditAccess: &NoteEditAccessRequest{
			ID:     0,
			Access: policy.MinContentAccess(),
		},
		Add: &NoteAddRequest{
			Access:   policy.MinContentAccess(),
			Composed: false,
		},
		Delete: &requests.MediumDeleteRequest{
			ID: 0,
		},
	}
}

type NoteEditRequest struct {
	ID   int64  `param:"id" validate:"required,numeric"`
	Body string `form:"body" validate:"required,max=8000"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
}

type NoteEditAccessRequest struct {
	ID     int64  `param:"id" validate:"required,numeric"`
	Access string `form:"access" validate:"required,oneof=public registered followers leaders mutuals admins myself"`
}

type NoteAddRequest struct {
	OwnerID  int64  `param:"owner_id" validate:"required,numeric"`
	Body     string `form:"body" validate:"required,max=8000"`
	Access   string `form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	Composed bool   `form:"composed" validate:"omitempty"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
}
