package federation

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func TestAQuotedPostIsLedTo(t *testing.T) {
	web := func(path string) string { return "https://banu.io" + path }

	photo := &entities.AnahitaNode{ID: 185877, Type: constants.NodeMediumPhotoEntityType}
	if got := QuotedAddress(photo, true, web); got != "https://banu.io/photos/185877" {
		t.Errorf("a public photo here: %q", got)
	}
	if got := QuotedAddress(photo, false, web); got != "" {
		t.Errorf("a post here nobody signed out may see: %q", got)
	}
	note := &entities.AnahitaNode{ID: 12, Type: constants.NodeMediumNoteEntityType}
	if got := QuotedAddress(note, true, web); got != "https://banu.io/notes/12" {
		t.Errorf("a note: %q", got)
	}

	mastodon := &entities.AnahitaNode{ID: 13, URI: "https://mastodon.world/users/ana/statuses/1"}
	if got := QuotedAddress(mastodon, false, web); got != mastodon.URI {
		t.Errorf("a post from the fediverse: %q", got)
	}
	bluesky := &entities.AnahitaNode{ID: 14, URI: "at://did:plc:abc/app.bsky.feed.post/3n2f"}
	if got := QuotedAddress(bluesky, false, web); got != "https://bsky.app/profile/did:plc:abc/post/3n2f" {
		t.Errorf("a post from Bluesky: %q", got)
	}

	gone := time.Now()
	if got := QuotedAddress(&entities.AnahitaNode{ID: 15, Type: constants.NodeMediumNoteEntityType, DeletedAt: &gone}, true, web); got != "" {
		t.Errorf("a deleted post: %q", got)
	}
	if got := QuotedAddress(nil, true, web); got != "" {
		t.Errorf("nothing: %q", got)
	}
}
