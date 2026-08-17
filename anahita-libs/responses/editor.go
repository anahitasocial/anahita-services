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

type Editor struct {
	ID     int64            `json:"id"`
	Type   string           `json:"type,omitempty"`
	Alias  string           `json:"alias,omitempty"`
	Name   string           `json:"name,omitempty"`
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
}
