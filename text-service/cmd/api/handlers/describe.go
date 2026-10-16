package handlers

import "anahita-libs/responses"

// What the routes send, for the API reference (cmd/api/routes.go).

// Thread is everything said under a post: the replies, and what the viewer
// may do there.
type Thread struct {
	// The replies, each saying what it answers. Hidden ones are sent only
	// to whoever may show them again, marked `hidden`.
	Data []*responses.MediumNode `json:"data"`
	// For whoever looks after the thread: how many of the replies sent are
	// hidden from everybody else.
	HiddenCount int `json:"hidden_count"`
	// Whether the viewer may reply here at all, for a thread with nothing
	// in it yet.
	CanReply bool `json:"can_reply"`
	// Who the post's author lets reply, for saying why not: `anyone`,
	// `nobody`, or any of `followers`, `following` and `mentioned`.
	ReplyAccess string `json:"reply_access"`
	// Whether the thread is longer than is sent at once: 500 replies.
	Truncated bool `json:"truncated"`
}

// NotARoot refuses a thread asked for by a reply, and says the post at the
// top of the one it is in.
type NotARoot struct {
	// Always `not_a_root`.
	Error string `json:"error"`
	// The post whose thread it is in.
	RootID int64 `json:"root_id"`
}

// HiddenAnswer says whether a reply is hidden now.
type HiddenAnswer struct {
	// The reply's id.
	ID int64 `json:"id"`
	// Whether it is hidden.
	Hidden bool `json:"hidden"`
}

// SavedAnswer says whether a post is among the viewer's saved now.
type SavedAnswer struct {
	// The post's id.
	ID int64 `json:"id"`
	// Whether the viewer has it saved.
	IsSavedByViewer bool `json:"is_saved_by_viewer"`
}
