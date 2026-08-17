package responses

import (
	"anahita-libs/data/entities"
)

func NewAdmin(node *entities.AnahitaNode) *Admin {
	if node == nil {
		return nil
	}

	dto := &Admin{
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

type Admin struct {
	ID     int64            `json:"id"`
	Type   string           `json:"type,omitempty"`
	Alias  string           `json:"alias,omitempty"`
	Name   string           `json:"name,omitempty"`
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
}
