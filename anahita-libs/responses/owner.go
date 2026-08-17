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
}
