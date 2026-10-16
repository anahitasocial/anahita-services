package responses

import (
	"anahita-libs/data/entities"
)

func NewOwner(node *entities.AnahitaNode) *Owner {
	if node == nil {
		return nil
	}

	dto := &Owner{
		ID:     node.ID,
		Type:   node.Type,
		Alias:  node.Alias,
		Name:   node.Name,
		Body:   node.Body,
		Avatar: make(map[string]Image),
		// Set where a caller has asked how the owner stands with the
		// viewer, as the feed does; false and left out otherwise.
		IsLeadingViewer:   node.IsLeadingViewer,
		IsFollowingViewer: node.IsFollowingViewer,
	}

	if node.Avatar != nil {
		dto.Avatar = NewAvatar(node.Avatar)
	}

	return dto
}

// Owner is the profile something is on.
type Owner struct {
	// Its id.
	ID int64 `json:"id"`
	// What kind of thing it is: `node.actor.<service>.person.v1`, `...group.v1`,
	// `...event.v1`, or a post's kind.
	Type string `json:"type"`
	// A profile's handle (`ana`, or `ana@mastodon.social` for somebody on
	// another server), or a hashtag's name.
	Alias string `json:"alias,omitempty"`
	// A profile's name, or a post's title where it has one.
	Name string `json:"name,omitempty"`
	// Its description.
	Body string `json:"body,omitempty"`
	// Its picture, in each size there is: `small`, `medium`, `large`,
	// `square`.
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
	// Whether the viewer follows the owner: with is_following_viewer, what
	// the Follow item in a post's menu says (Unfollow, Follow, Follow back).
	IsLeadingViewer bool `json:"is_leading_viewer,omitempty"`
	// Whether the owner follows the viewer.
	IsFollowingViewer bool `json:"is_following_viewer,omitempty"`
}
