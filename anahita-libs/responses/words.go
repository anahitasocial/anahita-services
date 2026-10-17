package responses

import (
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// words is what a node says, in the API's words for its kind. A post has a
// title and content, as WordPress and ActivityPub call them; a profile, a
// custom feed and a hashtag have a name and a body. The tables keep `name`
// and `body` for both: only what the API calls them differs.
type words struct {
	Name    string
	Body    string
	Title   string
	Content string
}

// IsPost says whether a node of this type is a post: a note, an article, a
// topic, a photo post or a video post, a reply among them.
func IsPost(nodeType string) bool {
	return strings.HasPrefix(nodeType, constants.NodeMediumPrefixEntityType)
}

func wordsOf(node *entities.AnahitaNode) words {
	if IsPost(node.Type) {
		return words{Title: node.Name, Content: node.Body}
	}
	return words{Name: node.Name, Body: node.Body}
}
