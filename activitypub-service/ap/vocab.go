// Package ap is ActivityPub as this service speaks it: the few shapes that
// are sent and read, written out as they are on the wire.
//
// Not a library for ActivityStreams in general. That vocabulary is large and
// open-ended, and a server needs a small, fixed part of it; what is here is
// that part, and anything else that arrives is ignored.
package ap

const (
	// The type a server asks for and is answered in.
	ContentType = "application/activity+json; charset=utf-8"
	// The type of a WebFinger answer.
	JRDContentType = "application/jrd+json; charset=utf-8"

	// The collection that means everybody.
	Public = "https://www.w3.org/ns/activitystreams#Public"

	contextActivityStreams = "https://www.w3.org/ns/activitystreams"
	contextSecurity        = "https://w3id.org/security/v1"
)

// actorContext is what the words in a person's description mean. The last
// part names the two that are Mastodon's and that every server has since
// come to read.
var actorContext = []any{
	contextActivityStreams,
	contextSecurity,
	map[string]string{
		"toot":                      "http://joinmastodon.org/ns#",
		"manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
		"discoverable":              "toot:discoverable",
	},
}

// Actor is somebody, as they are described to another server.
type Actor struct {
	Context           any        `json:"@context,omitempty"`
	ID                string     `json:"id"`
	Type              string     `json:"type"`
	PreferredUsername string     `json:"preferredUsername"`
	Name              string     `json:"name,omitempty"`
	Summary           string     `json:"summary,omitempty"`
	URL               string     `json:"url,omitempty"`
	Inbox             string     `json:"inbox"`
	Outbox            string     `json:"outbox"`
	Followers         string     `json:"followers"`
	Following         string     `json:"following"`
	Endpoints         *Endpoints `json:"endpoints,omitempty"`
	PublicKey         PublicKey  `json:"publicKey"`
	Icon              *Image     `json:"icon,omitempty"`
	Image             *Image     `json:"image,omitempty"`
	Published         string     `json:"published,omitempty"`
	// Whether a follow waits to be approved.
	ManuallyApprovesFollowers bool `json:"manuallyApprovesFollowers"`
	Discoverable              bool `json:"discoverable"`
}

type Endpoints struct {
	SharedInbox string `json:"sharedInbox,omitempty"`
}

type PublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPem string `json:"publicKeyPem"`
}

type Image struct {
	Type      string `json:"type"`
	MediaType string `json:"mediaType,omitempty"`
	URL       string `json:"url"`
}

// Collection is a list said only by how long it is: who follows somebody
// is theirs to know, and how many is all that is told.
type Collection struct {
	Context    any    `json:"@context"`
	ID         string `json:"id"`
	Type       string `json:"type"`
	TotalItems int64  `json:"totalItems"`
}

// NewCollection is a list of so many, with none of them named.
func NewCollection(id string, total int64) *Collection {
	if total < 0 {
		total = 0
	}
	return &Collection{Context: contextActivityStreams, ID: id, Type: "OrderedCollection", TotalItems: total}
}

// JRD is a WebFinger answer: who a handle is, and where they are.
type JRD struct {
	Subject string    `json:"subject"`
	Aliases []string  `json:"aliases,omitempty"`
	Links   []JRDLink `json:"links"`
}

type JRDLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
	Href string `json:"href,omitempty"`
}
