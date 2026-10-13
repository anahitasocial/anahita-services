package ap

import (
	"path"
	"strings"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/federation"
)

// NewActor describes a person here to another server.
//
// The caller has decided that they may be described at all. Nothing here is
// more than their public profile already shows a visitor: a name, what they
// say of themselves, their pictures.
func NewActor(a Addresses, person *entities.AnahitaNode, publicKeyPEM string) *Actor {
	uuid := person.UUID

	actor := &Actor{
		Context:           actorContext,
		ID:                a.Actor(uuid),
		Type:              "Person",
		PreferredUsername: person.Alias,
		Name:              person.Name,
		URL:               a.Profile(person.Alias),
		Inbox:             a.Inbox(uuid),
		Outbox:            a.Outbox(uuid),
		Followers:         a.Followers(uuid),
		Following:         a.Following(uuid),
		Endpoints:         &Endpoints{SharedInbox: a.SharedInbox()},
		PublicKey: PublicKey{
			ID:           a.Key(uuid),
			Owner:        a.Actor(uuid),
			PublicKeyPem: publicKeyPEM,
		},
		// Never: only a profile that is open to everybody is on the
		// fediverse, and such a profile is followed by whoever chooses to,
		// here as from elsewhere.
		ManuallyApprovesFollowers: false,
		Discoverable:              true,
	}

	if person.Body != "" {
		actor.Summary = federation.ToHTML(person.Body, federation.Links{Hashtag: a.Hashtag}).HTML
	}
	if !person.CreatedAt.IsZero() && person.CreatedAt.Unix() > 0 {
		actor.Published = person.CreatedAt.UTC().Format(time.RFC3339)
	}
	if person.AvatarFilename != "" {
		actor.Icon = &Image{Type: "Image", MediaType: mediaType(person.AvatarFilename), URL: a.Avatar(uuid)}
	}
	if person.CoverFilename != "" {
		actor.Image = &Image{Type: "Image", MediaType: mediaType(person.CoverFilename), URL: a.Cover(uuid)}
	}

	return actor
}

// NewInstanceActor describes the installation itself: who signs what it
// asks of other servers in nobody's name.
func NewInstanceActor(a Addresses, publicKeyPEM string) *Actor {
	return &Actor{
		Context:           actorContext,
		ID:                a.Instance(),
		Type:              "Application",
		PreferredUsername: a.Site.Domain,
		Name:              a.Site.Domain,
		URL:               a.Site.WebURL,
		Inbox:             a.SharedInbox(),
		Outbox:            a.Instance() + "/outbox",
		Followers:         a.Instance() + "/followers",
		Following:         a.Instance() + "/following",
		Endpoints:         &Endpoints{SharedInbox: a.SharedInbox()},
		PublicKey: PublicKey{
			ID:           a.InstanceKey(),
			Owner:        a.Instance(),
			PublicKeyPem: publicKeyPEM,
		},
		ManuallyApprovesFollowers: true,
	}
}

// mediaType is what kind of picture a stored name says it is.
func mediaType(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return "image/jpeg"
}
