package requests

// AccountBrowseRequest is the query string of GET /accounts.
//
// The three "only" filters are spelled as the value that is unusual:
// email_verified=false asks for people who never verified, and so on. Leaving
// one out means no filter; there is no way to ask for the verified ones only,
// because nobody cleaning up a site is looking for those.
type AccountBrowseRequest struct {
	Kind   string `query:"kind" validate:"omitempty,oneof=person group"`
	Search string `query:"q" validate:"max=100"`
	Tier   string `query:"tier" validate:"omitempty,oneof=empty dormant active"`
	State  string `query:"state" validate:"omitempty,oneof=active disabled archived deleted"`

	// Dates, as YYYY-MM-DD. An account matches if it was last active, or
	// created, before the start of that day.
	LastActiveBefore string `query:"last_active_before" validate:"omitempty,datetime=2006-01-02"`
	CreatedBefore    string `query:"created_before" validate:"omitempty,datetime=2006-01-02"`

	EmailVerified string `query:"email_verified" validate:"omitempty,oneof=false"`
	Onboarded     string `query:"onboarded" validate:"omitempty,oneof=false"`
	HasAdmin      string `query:"has_admin" validate:"omitempty,oneof=false"`

	// ContentMax is a pointer so that 0, "has written nothing", can be told
	// from not asking.
	ContentMax *int64 `query:"content_max" validate:"omitempty,min=0"`

	Sort      string `query:"sort" validate:"omitempty,oneof=last_active_at created_at name"`
	Direction string `query:"dir" validate:"omitempty,oneof=asc desc"`
	Limit     uint64 `query:"limit" validate:"max=500"`
	Offset    uint64 `query:"offset"`
}
