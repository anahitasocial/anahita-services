package ap

import (
	"fmt"
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

// Note is the address of a post made here, and NoteCreate of its having
// been posted. Made of the post's UUID, like a person's.
func (a Addresses) Note(uuid string) string       { return a.Site.API("/ap/notes/" + uuid) }
func (a Addresses) NoteCreate(uuid string) string { return a.Note(uuid) + "/activity" }

// NotePage is where a note is read, in the web app.
func (a Addresses) NotePage(id int64) string { return a.Site.Web(fmt.Sprintf("/notes/%d", id)) }

// PhotoPage is where a photo post is read, in the web app.
func (a Addresses) PhotoPage(id int64) string { return a.Site.Web(fmt.Sprintf("/photos/%d", id)) }

// Picture is one of a photo post's pictures, as other servers fetch it:
// by the post's UUID and where the picture comes in it, from 0.
func (a Addresses) Picture(uuid string, position int) string {
	return a.Site.API(fmt.Sprintf("/ap/notes/%s/pictures/%d", uuid, position))
}

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

// AliasOf is whose profile in the web app an address is, or "" for any
// other address.
func (a Addresses) AliasOf(address string) string {
	rest, ok := strings.CutPrefix(address, a.Site.Web("/people/"))
	if !ok || rest == "" || strings.ContainsAny(rest, "/?#@ ") {
		return ""
	}
	return rest
}

// Hashtag is a hashtag's page in the web app, by the address it is given
// to other servers: /tags/<name>, which leads to /hashtags/<name> there.
//
// Not the web app's own address for it, because of who reads this one.
// Mastodon, and most of what speaks to it, has a hashtag's page at
// /tags/<name>, and some of the apps people read with take a link for a
// hashtag's by that shape: a link to /hashtags/<name> was opened in a
// browser by Mastodon's own phone app, where one to /tags/<name> opens the
// hashtag in the app.
func (a Addresses) Hashtag(tag string) string { return a.Site.Web("/tags/" + tag) }

// Avatar and Cover are a person's pictures, at addresses that stay.
func (a Addresses) Avatar(uuid string) string { return a.Site.API("/avatars/" + uuid + "/large") }
func (a Addresses) Cover(uuid string) string  { return a.Site.API("/covers/" + uuid + "/large") }

// NoteUUIDOf is the UUID in the address of one of our own notes, or "" for
// any other address.
func (a Addresses) NoteUUIDOf(address string) string {
	rest, ok := strings.CutPrefix(address, a.Site.API("/ap/notes/"))
	if !ok || rest == "" || strings.ContainsAny(rest, "/?#") {
		return ""
	}
	return rest
}

// UUIDOf is the UUID in one of our own actors' addresses, or "" for any
// other address.
func (a Addresses) UUIDOf(address string) string {
	rest, ok := strings.CutPrefix(address, a.Site.API("/ap/users/"))
	if !ok || rest == "" || strings.ContainsAny(rest, "/?#") {
		return ""
	}
	return rest
}
