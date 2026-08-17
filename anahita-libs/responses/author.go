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

type Author struct {
	ID     int64            `json:"id"`
	Type   string           `json:"type,omitempty"`
	Alias  string           `json:"alias,omitempty"`
	Name   string           `json:"name,omitempty"`
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
