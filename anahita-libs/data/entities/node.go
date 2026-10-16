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
	// Whether the viewer may invite people to follow this actor.
	Invite *bool
	// On a reply: whether the viewer may hide it from its thread, or show
	// it again. nil on anything that is not a reply.
	Hide *bool
	// On a post: whether the viewer may quote it. nil when not worked out.
	Quote *bool
	// On a post: whether the viewer may pin it to the profile it is on.
	Pin *bool
	// On a post or a reply in a group or an event: whether the viewer may
	// block its writer from there, as the group. nil when not worked out.
	BlockFromOwner *bool
	// On a post that holds a file: whether the viewer may download it. nil
	// on a post that holds none, and when not worked out.
	Download *bool
	// Actors only: the composers the viewer may use on this profile. nil when
	// not worked out; empty when none.
	Composers []string
	// On an actor: the audiences a new post by the viewer may be given on
	// this profile. On a post: the audiences the viewer may change it to,
	// set only when they may edit it. Widest first; nil when not worked out.
	Audiences []string
}

// AnahitaNodeImageSize is how big a picture is.
type AnahitaNodeImageSize struct {
	// In pixels.
	Width int
	// In pixels.
	Height int
}

// AnahitaNodeImage is one size of a picture. It has no JSON tags, so its
// fields are sent as Go names them.
type AnahitaNodeImage struct {
	// Where it is read from: good for a short while.
	Url string
	// How big it is.
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
	ID int64
	// What the node is called when another server is told of it: random,
	// given by the database when the node is added, and never changed.
	// Read only; nothing sets it.
	UUID string
	// What the node is called where it lives, when that is another server:
	// the address of an ActivityPub object, or an at:// URI. Empty on
	// everything made here.
	URI string
	// The server it came from, in lower case. Empty on everything made here.
	OriginDomain string
	Type         string
	Service      string
	Name         string
	Alias        string
	Body         string
	StartsAt     time.Time
	EndsAt       time.Time
	OwnerID      int64
	OwnerType    string
	ParentID     int64
	ParentType   string
	// The post at the top of the thread this reply belongs to. 0 on
	// anything that is not a reply. Who may see a reply, answer it, like it
	// or remove it is decided by this post, never by the reply's own owner,
	// who is its author.
	RootID int64
	// On a reply: hidden from its thread by whoever looks after the thread.
	// Not a column. It is a fact about the thread, worked out when a thread
	// is read.
	Hidden     bool
	QuoteCount int64
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
	// LastVisitedAt is nil for a person who has never signed in.
	//
	// A pointer for the same reason DeletedAt is one: the column is nullable,
	// and a zero time is a value rather than an absence. Scanning NULL into a
	// time.Time fails outright — "unsupported Scan, storing driver.Value type
	// <nil>" — which is how twenty-one people became unreadable the moment
	// their legacy '0000-00-00' was normalised to NULL.
	LastVisitedAt    *time.Time
	Pinned           bool
	Meta             json.RawMessage
	ViewCount        int64
	GeoLatitude      float64
	GeoLongitude     float64
	GeoAddress       string
	GeoCity          string
	GeoStateProvince string
	GeoCountry       string
	GeoPostalcode    string
	CreatedAt        time.Time
	CreatedBy        int64
	UpdatedAt        time.Time
	ModifiedBy       int64
	// The file this node is: a photo post's photo, and a document's file if
	// file sharing is built. Never an actor's picture; that is
	// AvatarFilename.
	Filename string
	// The picture that stands for an actor. Only actors have one.
	AvatarFilename     string
	CoverFilename      string
	FollowerCount      int64
	LeaderCount        int64
	AllowFollowRequest bool
	// Whether what an actor reposts is also among its profile's posts. Off
	// unless it says otherwise: reposts are in the list of reposts.
	ShowRepostsOnProfile bool
	// On a post that holds a file: whether whoever can see it may download
	// it. On a person or a group: what their new posts start with. Off
	// unless somebody turned it on; nobody is taken to have agreed.
	AllowDownload bool
	// A photo or video post in its author's media library only: read by
	// its address with the access any post has, listed nowhere but the
	// library, and announced to nobody. Set when it is made.
	LibraryOnly bool
	// An article or a topic still being written: seen by its author alone,
	// listed nowhere but their drafts, announced to nobody. Only ever
	// cleared, by publishing it.
	Draft          bool
	Mimetype       string
	Language       string
	Access         string
	PermissionsRaw string
	Permissions    map[string]string
	Features       []*AnahitaNodeFeature
	FeaturesRaw    json.RawMessage

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
	// For a reply read on its own: the post at the top of its thread, so
	// the page can say what it is a reply to and lead there. Not stored,
	// and set only by a handler that has checked the viewer may see it.
	Root  *AnahitaNode
	Owner *AnahitaNode

	SubscriberCount      int64
	IsSubscribedByViewer bool

	LikesCount      int64
	IsLikedByViewer bool
	// Whether the viewer has saved it, to find again. Known to them alone.
	IsSavedByViewer bool

	// An actor the viewer may not see, shown because it lets people ask to
	// follow it. It carries its name and picture and nothing else.
	Restricted bool
	// Whether the viewer has asked to follow this actor and is waiting.
	IsFollowRequestedByViewer bool
	// Whether the viewer has been invited to follow this actor and is yet
	// to answer.
	IsInvitedViewer bool

	// On an event: what only an event has. Nil on anything else.
	Event *EventDetails
	// Whether the viewer may be shown where to join it online.
	EventShowsOnlineURL bool

	RepostCount        int64
	IsRepostedByViewer bool
	CommentCount       int64
	CommentStatus      bool
	// Who may reply, as the post's author sets it. Empty when they have
	// not said. See constants.NormalizeReplyAccess.
	ReplyAccess string
	// Who may quote: on a post, that post; on a person, their posts when
	// they say nothing. See constants.EffectiveQuotePolicy.
	QuotePolicy string
	// On a note that quotes a post: the post, and whether that post's
	// author has taken it out of the quote.
	QuoteID       int64
	QuoteDetached bool
	// The quoted post as this viewer may see it, and what to say when they
	// may not. Not stored: filled by the handler that sends the note.
	Quote      *AnahitaNode
	QuoteState string

	IsLeadingViewer   bool
	Administrators    []*AnahitaNode
	IsAdminedByViewer bool
	IsFollowingViewer bool
	IsBlockedByViewer bool

	// Images
	Avatar   map[string]AnahitaNodeImage
	Cover    map[string]AnahitaNodeImage
	Portrait map[string]AnahitaNodeImage
	// The images of a photo post, in order. Filled in by the services that
	// show them; nil elsewhere, and for everything that is not a photo.
	Files []*PhotoFile

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

// IsDraft says whether it is a draft, an article or a topic still being
// written, which its author alone may read.
func (a *AnahitaNode) IsDraft() bool {
	return a != nil && a.Draft
}
