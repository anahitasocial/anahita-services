package ap

import (
	"encoding/json"
)

// Activity is something done, as it arrives from another server: somebody
// followed, somebody took a follow back. Only what is acted on is read;
// the rest of what a server sends is left where it is.
type Activity struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Actor  json.RawMessage `json:"actor"`
	Object json.RawMessage `json:"object"`
}

// ActorID is who did it. Sent as an address, or as a description with one.
func (a *Activity) ActorID() string { return idOf(a.Actor) }

// ObjectID is the address of what it was done to.
func (a *Activity) ObjectID() string { return idOf(a.Object) }

// Inner is what it was done to, when that is itself something done: the
// Follow inside an Undo or an Accept. Nil when it is only an address.
func (a *Activity) Inner() *Activity {
	var inner Activity
	if len(a.Object) == 0 || a.Object[0] != '{' || json.Unmarshal(a.Object, &inner) != nil {
		return nil
	}
	return &inner
}

// idOf reads an address sent as itself or as the id of an object.
func idOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var address string
	if json.Unmarshal(raw, &address) == nil {
		return address
	}
	var object struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &object) == nil {
		return object.ID
	}
	return ""
}

// Sent is something done, as it is sent to another server.
type Sent struct {
	Context any    `json:"@context"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	Actor   string `json:"actor"`
	Object  any    `json:"object"`
}

// FollowRef is a follow as it is quoted back to whoever asked for it.
type FollowRef struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Actor  string `json:"actor"`
	Object string `json:"object"`
}

// NewAccept says yes to a follow: `actor` here accepts the Follow that
// `follower`, elsewhere, sent. The follow is quoted whole, because servers
// match the answer to the question by what is in it.
func NewAccept(id, actor, follower, followID string) *Sent {
	return &Sent{
		Context: contextActivityStreams,
		ID:      id,
		Type:    "Accept",
		Actor:   actor,
		Object:  FollowRef{ID: followID, Type: "Follow", Actor: follower, Object: actor},
	}
}

// NewFollow asks to follow: `actor` here, of `object` elsewhere.
func NewFollow(id, actor, object string) *Sent {
	return &Sent{Context: contextActivityStreams, ID: id, Type: "Follow", Actor: actor, Object: object}
}

// NewUndoFollow takes a follow back, quoting the Follow that asked for it.
func NewUndoFollow(id, actor, object, followID string) *Sent {
	return &Sent{
		Context: contextActivityStreams,
		ID:      id,
		Type:    "Undo",
		Actor:   actor,
		Object:  FollowRef{ID: followID, Type: "Follow", Actor: actor, Object: object},
	}
}

// NewBlock says somebody here blocks somebody elsewhere. Mastodon and what
// speaks to it stop showing them the blocker's posts and stop them
// answering, and end any follow between the two.
func NewBlock(id, actor, object string) *Sent {
	return &Sent{Context: contextActivityStreams, ID: id, Type: "Block", Actor: actor, Object: object}
}

// NewUndoBlock takes a block back, quoting the Block.
func NewUndoBlock(id, actor, object, blockID string) *Sent {
	return &Sent{
		Context: contextActivityStreams,
		ID:      id,
		Type:    "Undo",
		Actor:   actor,
		Object:  FollowRef{ID: blockID, Type: "Block", Actor: actor, Object: object},
	}
}

// NewLike says somebody here likes something elsewhere.
func NewLike(id, actor, object string) *Sent {
	return &Sent{Context: contextActivityStreams, ID: id, Type: "Like", Actor: actor, Object: object}
}

// NewUndoLike takes a like back, quoting the Like that made it.
func NewUndoLike(id, actor, object, likeID string) *Sent {
	return &Sent{
		Context: contextActivityStreams,
		ID:      id,
		Type:    "Undo",
		Actor:   actor,
		Object:  FollowRef{ID: likeID, Type: "Like", Actor: actor, Object: object},
	}
}

// Announced is a repost, as it is sent: what Mastodon calls a boost.
type Announced struct {
	Context   any      `json:"@context,omitempty"`
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Actor     string   `json:"actor"`
	Published string   `json:"published,omitempty"`
	To        []string `json:"to,omitempty"`
	Cc        []string `json:"cc,omitempty"`
	Object    string   `json:"object,omitempty"`
}

// NewAnnounce says somebody here reposted something: to everybody, with
// their followers, and whoever wrote it when they are elsewhere, named as
// who it is for.
func NewAnnounce(id, actor, object, published string, cc ...string) *Announced {
	return &Announced{
		Context: contextActivityStreams, ID: id, Type: "Announce", Actor: actor,
		Published: published, To: []string{Public}, Cc: cc, Object: object,
	}
}

// NewUndoAnnounce takes a repost back, quoting the Announce by its name.
func NewUndoAnnounce(id, actor, announceID string) *Sent {
	return &Sent{
		Context: contextActivityStreams,
		ID:      id,
		Type:    "Undo",
		Actor:   actor,
		Object:  Announced{ID: announceID, Type: "Announce", Actor: actor},
	}
}

// NewReject says no to one, the same way.
func NewReject(id, actor, follower, followID string) *Sent {
	sent := NewAccept(id, actor, follower, followID)
	sent.Type = "Reject"
	return sent
}
