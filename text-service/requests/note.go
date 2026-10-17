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
			ID:      0,
			Content: "",
		},
		EditAccess: &NoteEditAccessRequest{
			ID:     0,
			Access: policy.MinContentAccess(),
		},
		Add: &NoteAddRequest{
			Access: policy.MinContentAccess(),
		},
		Delete: &requests.MediumDeleteRequest{
			ID: 0,
		},
	}
}

type NoteEditRequest struct {
	// The note's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its words, as Markdown.
	Content string `form:"content" validate:"required,max=8000"`
	// As on NoteAddRequest. Left out, the note keeps the preview it has for
	// as long as that link is still in it.
	LinkPreviewURL string `json:"link_preview_url" form:"link_preview_url" validate:"omitempty,max=2000"`
	// The writer took the card off: the note is saved without one, though
	// its link is still there.
	LinkPreviewRemove bool `json:"link_preview_remove" form:"link_preview_remove"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
}

type NoteEditAccessRequest struct {
	ID     int64  `param:"id" validate:"required,numeric"`
	Access string `form:"access" validate:"required,oneof=public registered followers leaders mutuals admins myself"`
}

type NoteAddRequest struct {
	// The profile it is posted on.
	OwnerID int64 `param:"owner_id" validate:"required,numeric"`
	// Its words, as Markdown.
	Content string `form:"content" validate:"required,max=8000"`
	// Who may see it (permissions.md). The widest the installation allows
	// when left out.
	Access string `form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
	// As on requests.MediumAddRequest.
	ReplyAccess string `json:"reply_access" form:"reply_access" validate:"omitempty,max=64"`
	// As on requests.MediumAddRequest.
	QuotePolicy string `json:"quote_policy" form:"quote_policy" validate:"omitempty,max=16"`
	// The post this note quotes: it is carried under the note's own words.
	// Checked against that post's policy. See permissions.CanQuote.
	QuoteID int64 `json:"quote_id" form:"quote_id" validate:"omitempty,numeric"`
	// The link whose preview the note carries: one its writer was shown a
	// preview of while writing. Only the address is taken. What the card
	// says is what this installation read off the page, and never what is
	// sent here.
	LinkPreviewURL string `json:"link_preview_url" form:"link_preview_url" validate:"omitempty,max=2000"`
}
