package responses

import (
	"anahita-libs/data/entities"
)

func NewAuthor(node *entities.AnahitaNode) *Author {
	if node == nil {
		return nil
	}

	dto := &Author{
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

// Author is who wrote something.
type Author struct {
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

func (a *Author) MapAuthor(person *entities.Person) *Author {
	if person == nil {
		return nil
	}

	dto := &Author{
		ID:     person.ID,
		Type:   person.Type,
		Alias:  person.Alias,
		Name:   person.Name,
		Avatar: make(map[string]Image),
	}

	if person.Avatar != nil {
		dto.Avatar = NewAvatar(person.Avatar)
	}

	return dto
}
