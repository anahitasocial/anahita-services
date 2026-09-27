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

	// What the viewer may add, answered by the checks in
	// cmd/api/permissions/capabilities.go so the web app shows only controls
	// that will work. Pointers: nil means not worked out for this response,
	// which the web app treats differently from a no.
	Comment     *bool
	Like        *bool
	AddFollower *bool
	// Actors only: the composers the viewer may use on this profile. nil when
	// not worked out; empty when none.
	Composers []string
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
	WebsiteURL string
	// PersonType is the authority a person holds on the installation: guest,
	// registered, administrator or super-administrator
	// (constants.PersonType*). Meaningful only on person nodes.
	//
	// Named for the node rather than for auth. The graph owns the person; a
	// "user" is an auth concept, and that word is kept for protocol surface an
	// external specification names that way.
	PersonType string
	// PersonPronouns is free text — "she/her", "iel", or empty. Never an enum:
	// an enum hard-codes English grammar and is meaningless in Persian,
	// Turkish or Finnish, which have no gendered third-person pronoun. Display
	// only, never read for logic.
	PersonPronouns string
	// LastVisitDate is nil for a person who has never signed in.
	//
	// A pointer for the same reason DeletedAt is one: the column is nullable,
	// and a zero time is a value rather than an absence. Scanning NULL into a
	// time.Time fails outright — "unsupported Scan, storing driver.Value type
	// <nil>" — which is how twenty-one people became unreadable the moment
	// their legacy '0000-00-00' was normalised to NULL.
	LastVisitDate      *time.Time
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

	// FeaturedAt is set when the installation features this actor, nil
	// otherwise — the accounts onboarding offers to follow. It is also their
	// order: most recently featured first.
	//
	// A pointer for the same reason as the two above.
	FeaturedAt *time.Time
}
