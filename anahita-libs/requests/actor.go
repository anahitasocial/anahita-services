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
			Ordering:  "modified_on",
			Search:    "",
		},
		Read: &ActorReadRequest{},
		BatchRead: &ActorBatchReadRequest{
			IDs:       []int64{},
			Ordering:  "modified_on",
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
	Ordering  string `query:"ordering" validate:"max=15,omitempty,oneof=created_on modified_on enabled"`
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Disabled  bool   `query:"disabled" validate:"omitempty"`
	IDs       []int64
}

type ActorReadRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}

type ActorBatchReadRequest struct {
	IDs       []int64 `json:"ids" validate:"required,dive,required,numeric"`
	Ordering  string  `query:"ordering" validate:"max=15,omitempty,oneof=created_on modified_on enabled"`
	Direction string  `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Viewer    *entities.Viewer
}

type ActorEditRequest struct {
	ID         int64  `param:"id" validate:"required,numeric"`
	Name       string `form:"name" validate:"required,max=100"`
	Body       string `form:"body" validate:"required,max=8000"`
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
	Body       string `form:"body" validate:"required,max=10000"`
	Enabled    bool   `form:"enabled" validate:"omitempty"`
	WebsiteURL string `form:"website_url" validate:"omitempty,http_url,max=255"`
	// Normalised before validation so a bare "example.com" is accepted;
	// http_url then enforces the scheme, which is what keeps a
	// javascript: value out of an anchor href on a public profile.
}

type ActorDeleteRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}
