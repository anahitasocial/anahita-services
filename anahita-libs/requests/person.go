package requests

import "anahita-libs/constants"

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
			Ordering:  "modified_on",
			Search:    "",
		},
		Read: &PersonReadRequest{},
		Edit: &PersonEditRequest{},
		EditAccess: &PersonEditAccessRequest{
			ID:                 0,
			Access:             "public",
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
	Username string `param:"username" validate:"required,max=100,alphanumunicode"`
}

type PersonEditRequest struct {
	ID         int64  `param:"id" validate:"required,numeric"`
	GivenName  string `form:"given_name" validate:"required,max=100,alphanumunicode"`
	FamilyName string `form:"family_name" validate:"required,max=100,alphanumunicode"`
	Body       string `form:"body" validate:"required,max=8000"`
	Gender     string `form:"gender" validate:"max=10,omitempty,oneof=male female other"`
	// Normalised before validation so a bare "example.com" is accepted;
	// http_url then enforces the scheme, which is what keeps a javascript:
	// value out of an anchor href on a public profile.
	WebsiteURL string `form:"website_url" validate:"omitempty,http_url,max=255"`
}

type PersonEditAccessRequest struct {
	ID                 int64  `param:"id" validate:"required,numeric"`
	Access             string `form:"access" validate:"required,oneof=public registered followers leaders mutuals myself"`
	AllowFollowRequest bool   `form:"allow_follow_request" validate:"omitempty"`
}

type PersonEditAddPermission struct {
	Entity string `json:"entity" validate:"required,max=100,alphanumunicode"`
	Access string `json:"access" validate:"required,person_add_permission_access"`
}

type PersonFeatureRequest struct {
	Service        string                    `json:"service" validate:"required"`
	NodeTypes      []string                  `json:"node_types" validate:"required,dive,max=100,alphanumunicode"`
	Composers      []string                  `json:"composers" validate:"required,dive,max=100,alphanumunicode"`
	Optional       bool                      `json:"optional"`
	Enabled        bool                      `json:"enabled"`
	AddPermissions []PersonEditAddPermission `json:"add_permissions" validate:"dive"`
	Ordering       int                       `json:"ordering" validate:"required,numeric"`
}
type PersonEditFeaturesRequest struct {
	ID       int64                  `json:"id" validate:"required"`
	Features []PersonFeatureRequest `json:"features" validate:"required,dive"`
}

type PersonAddRequest struct{}

// PersonDeleteRequest identifies the person to delete by id.
//
// Username used to be here and `required`, bound from a :username route
// parameter the client never sent — it sends the id — so validation failed on
// every request before the handler ran.
type PersonDeleteRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}
