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

// MuteAddRequest mutes somebody, or changes a mute (roadmap row 133).
type MuteAddRequest struct {
	// The id of the person muted.
	TargetID int64 `param:"target_id" validate:"required,numeric"`
	// The id of whoever mutes: the viewer.
	SourceID int64 `param:"source_id" validate:"required,numeric"`
	// Whether their notifications are hidden too; true when not sent.
	HideNotifications *bool `json:"hide_notifications"`
	// How long the mute lasts: `day`, `week` or `month`; empty for until
	// unmuted.
	For string `json:"for" validate:"omitempty,oneof=day week month"`
}

// ServerBlockBrowseRequest lists the servers a person blocked for
// themselves (roadmap row 152).
type ServerBlockBrowseRequest struct {
	// The person's id: the viewer.
	ActorID int64 `param:"actor_id" validate:"required,numeric"`
	// How many to pass over.
	Offset int64 `query:"start" validate:"omitempty,min=0"`
	// The most to send, up to 100.
	Limit int64 `query:"limit" validate:"omitempty,min=0,max=100"`
}

// ServerBlockAddRequest blocks a server for the viewer.
type ServerBlockAddRequest struct {
	// The person's id: the viewer.
	ActorID int64 `param:"actor_id" validate:"required,numeric"`
	// The server: a domain (`tycho.example`), or a handle or an address on
	// it, from which its domain is taken.
	Domain string `json:"domain" validate:"required,max=255"`
}

// ServerBlockDeleteRequest unblocks a server for the viewer.
type ServerBlockDeleteRequest struct {
	// The person's id: the viewer.
	ActorID int64 `param:"actor_id" validate:"required,numeric"`
	// The server's domain, as it is listed.
	Domain string `param:"domain" validate:"required,max=255"`
}
