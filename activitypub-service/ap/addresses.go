package ap

import (
	"strings"

	"anahita-libs/site"
)

// Addresses is where things are, as other servers are told.
//
// EVERY ONE OF THESE IS KEPT BY WHOEVER IS TOLD IT, for good. A person is
// their address: change what this makes for somebody and they are somebody
// else to every server that knew them, with no followers. So they are made
// of the node's UUID, which never changes, and never of a username, which
// can; and they are made here and nowhere else.
type Addresses struct {
	Site site.Addresses
}

// Actor is a person's own address: what they are called between servers.
func (a Addresses) Actor(uuid string) string { return a.Site.API("/ap/users/" + uuid) }

func (a Addresses) Inbox(uuid string) string     { return a.Actor(uuid) + "/inbox" }
func (a Addresses) Outbox(uuid string) string    { return a.Actor(uuid) + "/outbox" }
func (a Addresses) Followers(uuid string) string { return a.Actor(uuid) + "/followers" }
func (a Addresses) Following(uuid string) string { return a.Actor(uuid) + "/following" }

// Activity is the address of something this installation did: an Accept,
// a Follow. Named by a UUID made for it.
func (a Addresses) Activity(uuid string) string { return a.Site.API("/ap/activities/" + uuid) }

// Key is the name of a person's signing key: their address, and which key.
func (a Addresses) Key(uuid string) string { return a.Actor(uuid) + "#main-key" }

// Instance is the installation's own address, for what it signs as itself.
func (a Addresses) Instance() string    { return a.Site.API("/ap/actor") }
func (a Addresses) InstanceKey() string { return a.Instance() + "#main-key" }

// SharedInbox takes what is for anybody here, sent once and not once for
// each of them.
func (a Addresses) SharedInbox() string { return a.Site.API("/inbox") }

// Profile is where a person is looked at, in the web app.
func (a Addresses) Profile(alias string) string { return a.Site.Web("/people/" + alias) }

// Hashtag is a hashtag's page in the web app.
func (a Addresses) Hashtag(tag string) string { return a.Site.Web("/hashtags/" + tag) }

// Avatar and Cover are a person's pictures, at addresses that stay.
func (a Addresses) Avatar(uuid string) string { return a.Site.API("/avatars/" + uuid + "/large") }
func (a Addresses) Cover(uuid string) string  { return a.Site.API("/covers/" + uuid + "/large") }

// UUIDOf is the UUID in one of our own actors' addresses, or "" for any
// other address.
func (a Addresses) UUIDOf(address string) string {
	rest, ok := strings.CutPrefix(address, a.Site.API("/ap/users/"))
	if !ok || rest == "" || strings.ContainsAny(rest, "/?#") {
		return ""
	}
	return rest
}
