package responses

import "anahita-libs/data/entities"

// ActorBrief is an actor named beside something else: who invited, who was
// invited, what they were invited to. Enough to show a picture and a name
// and to lead to the profile, and nothing of the profile itself.
type ActorBrief struct {
	// Its id.
	ID int64 `json:"id"`
	// What kind of thing it is: `node.actor.<service>.person.v1`, `...group.v1`,
	// `...event.v1`, or a post's kind.
	Type string `json:"type"`
	// A profile's name, or a post's title where it has one.
	Name string `json:"name,omitempty"`
	// A profile's handle (`ana`, or `ana@mastodon.social` for somebody on
	// another server), or a hashtag's name.
	Alias string `json:"alias,omitempty"`
	// A mark kept from earlier versions of Anahita. Nothing sets it yet.
	Verified bool `json:"verified,omitempty"`
	// Its picture, in each size there is: `small`, `medium`, `large`,
	// `square`.
	Avatar map[string]Image `json:"avatar_urls,omitempty"`
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
