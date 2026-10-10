package responses

import "anahita-libs/data/entities"

// ActorBrief is an actor named beside something else: who invited, who was
// invited, what they were invited to. Enough to show a picture and a name
// and to lead to the profile, and nothing of the profile itself.
type ActorBrief struct {
	ID       int64            `json:"id"`
	Type     string           `json:"type"`
	Name     string           `json:"name,omitempty"`
	Alias    string           `json:"alias,omitempty"`
	Verified bool             `json:"verified,omitempty"`
	Avatar   map[string]Image `json:"avatar_urls,omitempty"`
}

// NewActorBrief maps a node to its picture and name.
func NewActorBrief(node *entities.AnahitaNode) *ActorBrief {
	if node == nil {
		return nil
	}

	brief := &ActorBrief{
		ID:       node.ID,
		Type:     node.Type,
		Name:     node.Name,
		Alias:    node.Alias,
		Verified: node.Verified,
	}

	if node.Avatar != nil {
		brief.Avatar = NewAvatar(node.Avatar)
	}

	return brief
}
