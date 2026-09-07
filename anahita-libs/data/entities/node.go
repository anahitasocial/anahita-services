package entities

import (
	"encoding/json"
	"time"
)

type AnahitaNodeAddPermission struct {
	Entity string `json:"entity"`
	Access string `json:"access"`
}

type AnahitaNodeFeature struct {
	Service                  string                     `json:"service"`
	NodeTypes                []string                   `json:"node_types"`
	Composers                []string                   `json:"composers"`
	Optional                 bool                       `json:"optional"`
	Enabled                  bool                       `json:"enabled"`
	AddPermissions           []AnahitaNodeAddPermission `json:"add_permissions"`
	IncludeCommonPermissions []string                   `json:"include_common_permissions,omitempty"`
	Ordering                 int32                      `json:"ordering"`
}

type AnahitaNodeAuthorized struct {
	Edit           bool
	Delete         bool
	Administration bool
}

type AnahitaNodeImageSize struct {
	Width  int
	Height int
}

type AnahitaNodeImage struct {
	Url  string
	Size AnahitaNodeImageSize
}

// AnahitaNode is the entity for anahita node
func NewAnahitaNode() *AnahitaNode {
	a := &AnahitaNode{}

	// Initialize Authorized and its sub-entities to avoid nil pointer dereferences
	a.Authorized = &AnahitaNodeAuthorized{}
	a.Author = &AnahitaNode{}
	a.Editor = &AnahitaNode{}
	a.Owner = &AnahitaNode{}

	// Initialize Parent and its sub-entities to avoid nil pointer dereferences
	a.Parent = &AnahitaNode{}
	a.Parent.Owner = &AnahitaNode{}
	a.Parent.Author = &AnahitaNode{}
	a.Parent.Editor = &AnahitaNode{}

	return a
}

type AnahitaNode struct {
	ID         int64
	Type       string
	Service    string
	Name       string
	Alias      string
	Body       string
	Excerpt    string
	StartDate  time.Time
	EndDate    time.Time
	OwnerID    int64
	OwnerType  string
	ParentID   int64
	ParentType string
	QuoteCount int64
	Ordering   int32
	Enabled    bool
	Verified   bool
	// WebsiteURL is one link on an actor profile. Validated to http(s) before
	// it is stored — it is rendered as an anchor href, so a javascript: or
	// data: value would be stored XSS.
	WebsiteURL         string
	Pinned             bool
	IsDefault          bool
	Meta               json.RawMessage
	HitsCount          int64
	GeoLatitude        float64
	GeoLongitude       float64
	GeoAddress         string
	GeoCity            string
	GeoStateProvince   string
	GeoCountry         string
	GeoPostalcode      string
	CreationTime       time.Time
	CreatedBy          int64
	UpdateTime         time.Time
	ModifiedBy         int64
	Filename           string
	Filesize           int64
	CoverFilename      string
	CoverFilesize      int64
	CoverMimetype      string
	Status             string
	StatusUpdateTime   time.Time
	FollowerCount      int64
	LeaderCount        int64
	MutualCount        int64
	AllowFollowRequest bool
	DislikesCount      int64
	Mimetype           string
	Timezone           int32
	Language           string
	Access             string
	PermissionsRaw     string
	Permissions        map[string]string
	Features           []*AnahitaNodeFeature
	FeaturesRaw        json.RawMessage

	// Parsed Fields
	Mentions []string
	Hashtags []string
	Urls     []string

	// Authorized Fields
	Authorized *AnahitaNodeAuthorized

	// Related Entities
	Author *AnahitaNode
	Editor *AnahitaNode
	Parent *AnahitaNode
	Owner  *AnahitaNode

	SubscriberCount      int64
	IsSubscribedByViewer bool

	LikesCount      int64
	IsLikedByViewer bool

	RepostCount        int64
	IsRepostedByViewer bool
	CommentCount       int64
	CommentStatus      bool
	LastCommentBy      int64
	LastCommentID      int64
	LastCommentOn      time.Time

	IsLeadingViewer   bool
	Administrators    []*AnahitaNode
	IsAdminedByViewer bool
	IsFollowingViewer bool
	IsBlockedByViewer bool

	// Images
	Avatar   map[string]AnahitaNodeImage
	Cover    map[string]AnahitaNodeImage
	Portrait map[string]AnahitaNodeImage

	// @TODO remove EdgeCount field after migrating to mixins
	EdgeCount int64

	// DeletedAt is set when an actor is scheduled for deletion, nil otherwise.
	//
	// Deleting also clears Enabled, so a reader that only checks that flag
	// cannot tell an account on its way out from one an administrator disabled
	// — and the two want opposite treatment. The group settings page needs the
	// difference to offer Restore instead of the delete form.
	DeletedAt *time.Time

	// ArchivedAt is set when an actor is archived, nil otherwise.
	//
	// A pointer for the same reason DeletedAt is one: a zero time and "not
	// archived" are different states, and collapsing them makes every actor
	// look archived at the beginning of the epoch.
	ArchivedAt *time.Time
}
