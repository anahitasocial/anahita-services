package responses

import "anahita-libs/data/entities"

// LimitedActor is what somebody is shown of a profile they may not see, when
// that profile lets people ask to follow it: enough to know whose it is and
// to ask.
//
// A whitelist, written out. It is not an ActorNodeItem with fields blanked:
// a field added to that later would be sent here without anybody deciding
// that it should be.
type LimitedActor struct {
	ID       int64            `json:"id"`
	Type     string           `json:"type"`
	Name     string           `json:"name,omitempty"`
	Alias    string           `json:"alias,omitempty"`
	Verified bool             `json:"verified,omitempty"`
	Avatar   map[string]Image `json:"avatar_urls,omitempty"`
	// Always true. What tells a client this is not the profile itself.
	Restricted bool `json:"restricted"`
	// Always true: it is why this much is shown.
	AllowFollowRequest bool `json:"allow_follow_request"`
	// Whether the viewer has asked already and is waiting.
	IsFollowRequestedByViewer bool `json:"is_follow_requested_by_viewer"`
}

// NewLimitedActor maps a node to what a stranger is shown of it.
func NewLimitedActor(node *entities.AnahitaNode, requested bool) *LimitedActor {
	if node == nil {
		return nil
	}

	limited := &LimitedActor{
		ID:                        node.ID,
		Type:                      node.Type,
		Name:                      node.Name,
		Alias:                     node.Alias,
		Verified:                  node.Verified,
		Restricted:                true,
		AllowFollowRequest:        true,
		IsFollowRequestedByViewer: requested,
	}

	if node.Avatar != nil {
		limited.Avatar = NewAvatar(node.Avatar)
	}

	return limited
}
