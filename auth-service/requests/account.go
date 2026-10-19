package requests

// AccountBrowseRequest is the query string of GET /accounts.
//
// The three "only" filters are spelled as the value that is unusual:
// email_verified=false asks for people who never verified, and so on. Leaving
// one out means no filter; there is no way to ask for the verified ones only,
// because nobody cleaning up a site is looking for those.
type AccountBrowseRequest struct {
	// `person`, the default, `group`, `event` or `website`.
	Kind string `query:"kind" validate:"omitempty,oneof=person group event website"`
	// Words in the name or the username.
	Search string `query:"q" validate:"max=100"`
	// `empty`, `dormant` or `active`.
	Tier string `query:"tier" validate:"omitempty,oneof=empty dormant active"`
	// `active`, `disabled`, `archived` or `deleted`.
	State string `query:"state" validate:"omitempty,oneof=active disabled archived deleted"`

	// Dates, as YYYY-MM-DD. An account matches if it was last active, or
	// created, before the start of that day.
	LastActiveBefore string `query:"last_active_before" validate:"omitempty,datetime=2006-01-02"`
	CreatedBefore    string `query:"created_before" validate:"omitempty,datetime=2006-01-02"`

	// `false` for people who never confirmed their address.
	EmailVerified string `query:"email_verified" validate:"omitempty,oneof=false"`
	// `false` for people who never went through the walk-through after signing up.
	Onboarded string `query:"onboarded" validate:"omitempty,oneof=false"`
	// `false` for groups with no administrator.
	HasAdmin string `query:"has_admin" validate:"omitempty,oneof=false"`

	// ContentMax is a pointer so that 0, "has written nothing", can be told
	// from not asking.
	ContentMax *int64 `query:"content_max" validate:"omitempty,min=0"`

	// `last_active_at`, the default, `created_at` or `name`.
	Sort string `query:"sort" validate:"omitempty,oneof=last_active_at created_at name"`
	// `asc`, the default, or `desc`.
	Direction string `query:"dir" validate:"omitempty,oneof=asc desc"`
	// How many on a page: 50 unless said.
	Limit uint64 `query:"limit" validate:"max=500"`
	// How many to skip.
	Offset uint64 `query:"start"`
}
