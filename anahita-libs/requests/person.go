package requests

import (
	"anahita-libs/constants"
	"anahita-libs/policy"
)

type PersonRequest struct {
	Browse       *ActorBrowseRequest
	Read         *PersonReadRequest
	Edit         *PersonEditRequest
	EditAccess   *PersonEditAccessRequest
	EditFeatures *PersonEditFeaturesRequest
	Add          *PersonAddRequest
	Delete       *PersonDeleteRequest
}

func NewPersonRequest() *PersonRequest {
	return &PersonRequest{
		Browse: &ActorBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "updated_at",
			Search:    "",
		},
		Read: &PersonReadRequest{},
		Edit: &PersonEditRequest{},
		EditAccess: &PersonEditAccessRequest{
			ID:                 0,
			Access:             policy.MinContentAccess(),
			AllowFollowRequest: false,
		},
		EditFeatures: &PersonEditFeaturesRequest{
			ID:       0,
			Features: []PersonFeatureRequest{},
		},
		Add:    &PersonAddRequest{},
		Delete: &PersonDeleteRequest{},
	}
}

type PersonReadRequest struct {
	// Their username.
	//
	// The handle, which is nodes.alias. `alphanumunicode` is gone: it is
	// ^[\p{L}\p{N}]+$, so it rejects the hyphen that slugged handles use.
	Alias string `param:"username" validate:"required,max=100"`
}

type PersonEditRequest struct {
	// The person's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Their name.
	//
	// One name, free-form.
	//
	// given_name and family_name are gone. They assumed a Western name
	// structure, and `alphanumunicode` — ^[\p{L}\p{N}]+$ — rejected every
	// space, hyphen and apostrophe, so "Jean-Luc", "O'Brien" and "van der
	// Berg" could not be saved at all. nodes.name already held the right value
	// while those two were truncated at the first non-ASCII byte.
	Name string `form:"name" validate:"required,max=100"`
	// What they say of themselves.
	Body string `form:"body" validate:"omitempty,max=8000"`
	// The language this person usually posts in, as a BCP 47 tag. It is what
	// the composer starts on; each post can say otherwise.
	Language string `form:"language" validate:"omitempty,max=12"`
	// Their pronouns, as free text: `she/her`, `iel`.
	//
	// Free text, e.g. "she/her" or "iel". The gender enum it replaces was
	// broken in both directions: the validator allowed "other", which the
	// handler discarded, and the handler stored "neutral", which the validator
	// rejected.
	PersonPronouns string `form:"person_pronouns" validate:"omitempty,max=50"`
	// One link on the profile. A bare `example.com` is taken as https.
	//
	// Normalised before validation so a bare "example.com" is accepted;
	// http_url then enforces the scheme, which is what keeps a javascript:
	// value out of an anchor href on a public profile.
	WebsiteURL string `form:"website_url" validate:"omitempty,http_url,max=255"`
}

type PersonEditAccessRequest struct {
	// The person's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Who may see their profile: `public`, `registered`, `followers`,
	// `leaders`, `mutuals` or `myself`.
	Access string `form:"access" validate:"required,oneof=public registered followers leaders mutuals myself"`
	// Whether people it is not open to may ask to follow them.
	AllowFollowRequest bool `form:"allow_follow_request" validate:"omitempty"`
}

type PersonEditAddPermission struct {
	// What may be added: a kind of post.
	Entity string `json:"entity" validate:"required,max=100,alphanumunicode"`
	// Who may add it.
	Access string `json:"access" validate:"required,person_add_permission_access"`
}

type PersonFeatureRequest struct {
	// The service it is.
	Service string `json:"service" validate:"required"`
	// The kinds of post it makes.
	NodeTypes []string `json:"node_types" validate:"required,dive,max=100,alphanumunicode"`
	// The composers it offers.
	Composers []string `json:"composers" validate:"required,dive,max=100,alphanumunicode"`
	// Whether it may be turned off.
	Optional bool `json:"optional"`
	// Whether it is on for this profile.
	Enabled bool `json:"enabled"`
	// Who may add what.
	AddPermissions []PersonEditAddPermission `json:"add_permissions" validate:"dive"`
	// Where it comes among the others.
	Ordering int `json:"ordering" validate:"required,numeric"`
}
type PersonEditFeaturesRequest struct {
	// The person's id.
	ID int64 `param:"id" validate:"required"`
	// Each feature as it is to be. An empty list goes back to the installation's defaults.
	Features []PersonFeatureRequest `json:"features" validate:"required,dive"`
}

type PersonAddRequest struct{}

// PersonDeleteRequest identifies the person to delete by id.
//
// Username used to be here and `required`, bound from a :username route
// parameter the client never sent — it sends the id — so validation failed on
// every request before the handler ran.
type PersonDeleteRequest struct {
	// The person's id.
	ID int64 `param:"id" validate:"required,numeric"`
}
