package federation

import (
	"anahita-libs/posts"
	"strings"

	"anahita-libs/data/entities"
)

// QuotedAddress is the address of the post a post quotes, as a quote names
// it on the network it goes to: a post from elsewhere where it lives, its
// address on the fediverse or its page on Bluesky; a post made here, its
// page here, only when it is public (`public`, as the caller read it for
// somebody signed out). Nothing for a quoted post that is not. A quote goes
// only to the networks the quoted post is on (roadmap row 162), and the
// address is the line a server that shows no quotes leads its readers by.
func QuotedAddress(quoted *entities.AnahitaNode, public bool, web func(path string) string) string {
	if quoted == nil || quoted.ID == 0 || quoted.DeletedAt != nil {
		return ""
	}
	if uri := strings.TrimSpace(quoted.URI); uri != "" {
		if strings.HasPrefix(uri, "https://") {
			return uri
		}
		if page := blueskyPage(uri); page != "" {
			return page
		}
		return ""
	}
	if !public || web == nil {
		return ""
	}
	return web(posts.Page(quoted.Type, quoted.ID))
}

// blueskyPage is a Bluesky post's page, from its at:// address:
// at://did:plc:x/app.bsky.feed.post/rkey is
// https://bsky.app/profile/did:plc:x/post/rkey.
func blueskyPage(uri string) string {
	parts := strings.Split(strings.TrimPrefix(uri, "at://"), "/")
	if !strings.HasPrefix(uri, "at://") || len(parts) != 3 || parts[1] != "app.bsky.feed.post" || parts[0] == "" || parts[2] == "" {
		return ""
	}
	return "https://bsky.app/profile/" + parts[0] + "/post/" + parts[2]
}
