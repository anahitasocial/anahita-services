package responses

import (
	"anahita-libs/responses"
	"auth/data/entities"
)

type UserNode struct {
	ID          int64                      `json:"id"`
	PersonType  string                     `json:"user_type"`
	Alias       string                     `json:"alias,omitempty"`
	Name        string                     `json:"name,omitempty"`
	Email       string                     `json:"email,omitempty"`
	Avatar      map[string]responses.Image `json:"avatar_urls,omitempty"`
	TotpEnabled bool                       `json:"totp_enabled,omitempty"`
}

type UserNodeItem struct {
	ID          int64                      `json:"id"`
	PersonType  string                     `json:"user_type"`
	Alias       string                     `json:"alias,omitempty"`
	Name        string                     `json:"name,omitempty"`
	Email       string                     `json:"email,omitempty"`
	Avatar      map[string]responses.Image `json:"avatar_urls,omitempty"`
	TotpEnabled bool                       `json:"totp_enabled,omitempty"`
}

type UserResponse struct{}

func (r *UserResponse) MapNode(node *entities.User) *UserNode {
	if node == nil {
		return nil
	}

	return &UserNode{
		ID:         node.ID,
		PersonType: node.PersonType,
		Name:       node.Name,
		Alias:      node.Alias,
		Email:      node.Email,
		Avatar: func() map[string]responses.Image {
			if node.Avatar != nil {
				return responses.NewAvatar(node.Avatar)
			}
			return nil
		}(),
		TotpEnabled: node.TotpEnabled,
	}
}

func (r *UserResponse) MapNodeItem(node *entities.User) *UserNodeItem {
	if node == nil {
		return nil
	}

	return &UserNodeItem{
		ID:         node.ID,
		PersonType: node.PersonType,
		Name:       node.Name,
		Alias:      node.Alias,
		Email:      node.Email,
		Avatar: func() map[string]responses.Image {
			if node.Avatar != nil {
				return responses.NewAvatar(node.Avatar)
			}
			return nil
		}(),
		TotpEnabled: node.TotpEnabled,
	}
}

func (r *UserResponse) MapNodeItems(nodes []*entities.User) []*UserNodeItem {
	if nodes == nil {
		return nil
	}

	items := make([]*UserNodeItem, 0, len(nodes))
	for _, node := range nodes {
		item := r.MapNodeItem(node)
		if item != nil {
			items = append(items, item)
		}
	}
	return items
}
