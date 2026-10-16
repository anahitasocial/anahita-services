package handlers

// What the routes take and send, for the API reference (cmd/api/routes.go).

// EnabledRequest turns a profile on or off.
type EnabledRequest struct {
	// The profile's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Whether it is shown and may act. Off, it is out of sight until it is
	// turned on again; nothing is deleted.
	Enabled *bool `json:"enabled" validate:"required"`
}

// FeaturedRequest features a profile, or stops.
type FeaturedRequest struct {
	// The profile's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Whether onboarding offers it to follow.
	Featured *bool `json:"featured" validate:"required"`
}

// PersonDeletionCounts is what deleting a person would take with it, for
// the Danger zone.
type PersonDeletionCounts struct {
	// Their posts.
	Posts int64 `json:"posts"`
	// Their replies.
	Replies int64 `json:"replies"`
	// Who follows them.
	Followers int64 `json:"followers"`
	// Whom they follow.
	Following int64 `json:"following"`
	// The groups they administer.
	GroupsAdministered int64 `json:"groups_administered"`
	// When they joined, when it is known.
	MemberSince string `json:"member_since,omitempty"`
}

// GroupDeletionCounts is what deleting a group or an event would take with
// it, for the Danger zone.
type GroupDeletionCounts struct {
	// The posts in it.
	Posts int64 `json:"posts"`
	// The replies in it.
	Replies int64 `json:"replies"`
	// Who follows it.
	Followers int64 `json:"followers"`
	// Its administrators.
	Admins int64 `json:"admins"`
	// When it was made, when it is known.
	MemberSince string `json:"member_since,omitempty"`
}

// HandleRequest asks what an alias is. The handler reads it with c.Param;
// nothing binds into it.
type HandleRequest struct {
	// A profile's alias: `ana`, `anahita-atrium`, or `ana@mastodon.social`
	// for somebody elsewhere.
	Alias string `param:"alias"`
}
