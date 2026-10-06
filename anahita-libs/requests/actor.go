package requests

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

type ActorRequest struct {
	Browse       *ActorBrowseRequest
	Read         *ActorReadRequest
	BatchRead    *ActorBatchReadRequest
	Edit         *ActorEditRequest
	EditAccess   *ActorEditAccessRequest `validate:"omitempty"`
	EditFeatures *ActorEditFeaturesRequest
	Add          *ActorAddRequest
	Delete       *ActorDeleteRequest
}

func NewActorRequest() *ActorRequest {
	return &ActorRequest{
		Browse: &ActorBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "updated_at",
			Search:    "",
		},
		Read: &ActorReadRequest{},
		BatchRead: &ActorBatchReadRequest{
			IDs:       []int64{},
			Ordering:  "updated_at",
			Direction: "desc",
		},
		Edit: &ActorEditRequest{},
		EditAccess: &ActorEditAccessRequest{
			Access:             "public",
			AllowFollowRequest: false,
		},
		EditFeatures: &ActorEditFeaturesRequest{
			ID:       0,
			Features: []ActorFeatureRequest{},
		},
		Add: &ActorAddRequest{
			Enabled: true,
		},
		Delete: &ActorDeleteRequest{},
	}
}

type ActorBrowseRequest struct {
	Offset    uint64 `query:"start"`
	Limit     uint64 `query:"limit"`
	Search    string `query:"q" validate:"max=100,omitempty,search_query"`
	Ordering  string `query:"ordering" validate:"max=15,omitempty,oneof=created_at updated_at enabled"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool   `query:"disabled" validate:"omitempty"`
	// Only the actors the installation features — what onboarding offers a
	// new member to follow. Ordered most recently featured first unless the
	// caller asks for an ordering; see the browse handlers.
	Featured bool `query:"featured" validate:"omitempty"`
	IDs      []int64
}

// GraphOrdering is the ordering to ask the graph for.
//
// Ordering defaults to updated_at before the query string is bound, so by the
// time a handler sees it there is no telling a default from a choice — and for
// the featured list the default is wrong: the order is the featured date, most
// recent first, which is why that column is a timestamp. orderingGiven is
// whether the caller actually sent one; when they did not, the featured list
// asks for no ordering and the graph applies its own.
func (r *ActorBrowseRequest) GraphOrdering(orderingGiven bool) string {
	if r.Featured && !orderingGiven {
		return ""
	}

	return r.Ordering
}

type ActorReadRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}

type ActorBatchReadRequest struct {
	IDs       []int64 `json:"ids" validate:"required,dive,required,numeric"`
	Ordering  string  `query:"ordering" validate:"max=15,omitempty,oneof=created_at updated_at enabled"`
	Direction string  `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Viewer    *entities.Viewer
}

type ActorEditRequest struct {
	ID   int64  `param:"id" validate:"required,numeric"`
	Name string `form:"name" validate:"required,max=100"`
	// Optional. A profile with no bio is ordinary, and requiring one made
	// editing anything else on such a profile impossible.
	Body       string `form:"body" validate:"omitempty,max=8000"`
	Enabled    bool   `form:"enabled" validate:"omitempty"`
	WebsiteURL string `form:"website_url" validate:"omitempty,http_url,max=255"`
	// Normalised before validation so a bare "example.com" is accepted;
	// http_url then enforces the scheme, which is what keeps a
	// javascript: value out of an anchor href on a public profile.
}

type ActorEditAccessRequest struct {
	ID                 int64  `param:"id" validate:"required,numeric"`
	Access             string `form:"access" validate:"required,oneof=public registered followers admins"`
	AllowFollowRequest bool   `form:"allowFollowRequest" validate:"omitempty"`
}

type ActorEditAddPermission struct {
	Entity string `json:"entity" validate:"required,max=100,alphanumunicode"`
	Access string `json:"access" validate:"required,actor_add_permission_access"`
}

type ActorFeatureRequest struct {
	Service        string                   `json:"service" validate:"required"`
	NodeTypes      []string                 `json:"node_types" validate:"required,dive,max=100,alphanumunicode"`
	Composers      []string                 `json:"composers" validate:"required,dive,max=100,alphanumunicode"`
	Optional       bool                     `json:"optional"`
	Enabled        bool                     `json:"enabled"`
	AddPermissions []ActorEditAddPermission `json:"add_permissions" validate:"dive"`
	Ordering       int                      `json:"ordering" validate:"required,numeric"`
}
type ActorEditFeaturesRequest struct {
	ID       int64                 `json:"id" validate:"required"`
	Features []ActorFeatureRequest `json:"features" validate:"required,dive"`
}

type ActorAddRequest struct {
	Name       string `form:"name" validate:"required,max=100"`
	Body       string `form:"body" validate:"omitempty,max=10000"`
	Enabled    bool   `form:"enabled" validate:"omitempty"`
	WebsiteURL string `form:"website_url" validate:"omitempty,http_url,max=255"`
	// Normalised before validation so a bare "example.com" is accepted;
	// http_url then enforces the scheme, which is what keeps a
	// javascript: value out of an anchor href on a public profile.
}

type ActorDeleteRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}

// ActorEditRepostsOnProfileRequest says whether what a person reposts is
// also shown among the posts on their profile. People only: nothing else
// reposts.
type ActorEditRepostsOnProfileRequest struct {
	ID   int64 `param:"id" validate:"required,numeric"`
	Show bool  `json:"show_reposts_on_profile" form:"show_reposts_on_profile"`
}
