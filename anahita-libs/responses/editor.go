package responses

import (
	"anahita-libs/data/entities"
)

func NewEditor(node *entities.AnahitaNode) *Editor {
	if node == nil {
		return nil
	}

	dto := &Editor{
		ID:     node.ID,
		Type:   node.Type,
		Alias:  node.Alias,
		Name:   node.Name,
		Avatar: make(map[string]Image),
	}

	if node.Avatar != nil {
		dto.Avatar = NewAvatar(node.Avatar)
	}

	return dto
}

// Editor is who changed something last.
type Editor struct {
	// Its id.
	ID int64 `json:"id"`
	// What kind of thing it is: `node.actor.<service>.person.v1`, `...group.v1`,
	// `...event.v1`, or a post's kind.
	Type string `json:"type,omitempty"`
	// A profile's handle (`ana`, or `ana@mastodon.social` for somebody on
	// another server), or a hashtag's name.
	Alias string `json:"alias,omitempty"`
	// A profile's name, or a post's title where it has one.
	Name string `json:"name,omitempty"`
	// Its picture, in each size there is: `small`, `medium`, `large`,
	// `square`.
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
}
