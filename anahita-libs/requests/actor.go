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
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// Words the name, the handle or the description holds.
	Search string `query:"q" validate:"max=100,omitempty,search_query"`
	// What they are ordered by.
	Ordering string `query:"ordering" validate:"max=15,omitempty,oneof=created_at updated_at enabled"`
	// Which way the order runs.
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	// Disabled ones as well, for whoever may see them.
	Disabled bool `query:"disabled" validate:"omitempty"`
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
	// The profile's id.
	ID int64 `param:"id" validate:"required,numeric"`
}

type ActorBatchReadRequest struct {
	IDs       []int64 `json:"ids" validate:"required,dive,required,numeric"`
	Ordering  string  `query:"ordering" validate:"max=15,omitempty,oneof=created_at updated_at enabled"`
	Direction string  `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Viewer    *entities.Viewer
}

type ActorEditRequest struct {
	// The profile's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its name.
	Name string `form:"name" validate:"required,max=64"`
	// What it says of itself.
	//
	// Optional. A profile with no bio is ordinary, and requiring one made
	// editing anything else on such a profile impossible.
	Body string `form:"body" validate:"omitempty,max=500"`
	// Whether it is enabled.
	Enabled bool `form:"enabled" validate:"omitempty"`
	// One link on the profile. A bare `example.com` is taken as https.
	WebsiteURL string `form:"website_url" validate:"omitempty,http_url,max=255"`
	// Normalised before validation so a bare "example.com" is accepted;
	// http_url then enforces the scheme, which is what keeps a
	// javascript: value out of an anchor href on a public profile.
}

type ActorEditAccessRequest struct {
	// The profile's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Who may see it: `public`, `registered`, `followers` or `admins`.
	Access string `form:"access" validate:"required,oneof=public registered followers admins"`
	// Whether people it is not open to may ask to follow it.
	AllowFollowRequest bool `form:"allow_follow_request" validate:"omitempty"`
}

type ActorEditAddPermission struct {
	// What may be added: a kind of post.
	Entity string `json:"entity" validate:"required,max=100,alphanumunicode"`
	// Who may add it.
	Access string `json:"access" validate:"required,actor_add_permission_access"`
}

type ActorFeatureRequest struct {
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
	AddPermissions []ActorEditAddPermission `json:"add_permissions" validate:"dive"`
	// Where it comes among the others.
	Ordering int `json:"ordering" validate:"required,numeric"`
}
type ActorEditFeaturesRequest struct {
	// The profile's id.
	ID int64 `param:"id" validate:"required"`
	// Each feature as it is to be. An empty list goes back to the installation's defaults.
	Features []ActorFeatureRequest `json:"features" validate:"required,dive"`
}

type ActorAddRequest struct {
	// Its name.
	Name string `form:"name" validate:"required,max=64"`
	// What it says of itself.
	Body string `form:"body" validate:"omitempty,max=500"`
	// Whether it is enabled.
	Enabled bool `form:"enabled" validate:"omitempty"`
	// One link on the profile. A bare `example.com` is taken as https.
	WebsiteURL string `form:"website_url" validate:"omitempty,http_url,max=255"`
	// Normalised before validation so a bare "example.com" is accepted;
	// http_url then enforces the scheme, which is what keeps a
	// javascript: value out of an anchor href on a public profile.
}

type ActorDeleteRequest struct {
	// The profile's id.
	ID int64 `param:"id" validate:"required,numeric"`
}

// ActorEditRepostsOnProfileRequest says whether what a person reposts is
// also shown among the posts on their profile. People only: nothing else
// reposts.
type ActorEditRepostsOnProfileRequest struct {
	// The person's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Whether what they repost is also among their profile's posts.
	Show bool `json:"show_reposts_on_profile" form:"show_reposts_on_profile"`
}

// PersonEditQuotePolicyRequest says who may quote a person's posts, for the
// posts that do not say themselves.
type PersonEditQuotePolicyRequest struct {
	// The person's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Who may quote their posts, when a post does not say: `anyone`,
	// `followers` or `nobody`.
	QuotePolicy string `json:"quote_policy" form:"quote_policy" validate:"required,max=16"`
}

// ActorEditDownloadDefaultRequest says what a person's or a group's new
// photos and videos start with: whether they may be downloaded.
type ActorEditDownloadDefaultRequest struct {
	// The profile's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Whether its new photos and videos start out downloadable.
	Allow bool `json:"allow" form:"allow"`
}
