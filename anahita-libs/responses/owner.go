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

type Owner struct {
	ID     int64            `json:"id"`
	Type   string           `json:"type"`
	Alias  string           `json:"alias,omitempty"`
	Name   string           `json:"name,omitempty"`
	Body   string           `json:"body,omitempty"`
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
	// Whether the viewer follows the owner, and the owner the viewer. For
	// the Follow item in a post's menu: Unfollow, Follow, or Follow back.
	IsLeadingViewer   bool `json:"is_leading_viewer,omitempty"`
	IsFollowingViewer bool `json:"is_following_viewer,omitempty"`
}
