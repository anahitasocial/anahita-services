package federation

import (
	"fmt"
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/videos"
)

// QuotedLine starts the line a post that quotes ends with on another
// network, before the quoted post's address, as a quote that comes in ends
// with "↪ bsky.app".
const QuotedLine = "↪ "

// QuotedAddress is where a post that quotes leads, on another network, to
// the post it quotes: Mastodon and Bluesky make a card of it. A post from
// elsewhere is led to where it lives: its address on the fediverse, or its
// page on Bluesky. A post made here is led to its page here, only when it
// is public (`public`, as the caller read it for somebody signed out): a
// quoted post that is not leaves no line, and nothing of it goes out.
// web is the site's address of a path: web("/photos/12").
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
	return web(pathOf(quoted))
}

// pathOf is a post's page here, by its kind.
func pathOf(post *entities.AnahitaNode) string {
	switch post.Type {
	case constants.NodeMediumPhotoEntityType:
		return fmt.Sprintf("/photos/%d", post.ID)
	case videos.NodeType:
		return fmt.Sprintf("/videos/%d", post.ID)
	case videos.AudioNodeType:
		return fmt.Sprintf("/audio/%d", post.ID)
	case constants.NodeMediumArticleEntityType:
		return fmt.Sprintf("/articles/%d", post.ID)
	case "node.medium.text-service.topic.v1":
		return fmt.Sprintf("/topics/%d", post.ID)
	}
	return fmt.Sprintf("/notes/%d", post.ID)
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
