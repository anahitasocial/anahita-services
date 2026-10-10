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
	// Whether it lets people ask to follow it. False for one shown only
	// because the viewer was invited.
	AllowFollowRequest bool `json:"allow_follow_request"`
	// Whether the viewer has asked already and is waiting.
	IsFollowRequestedByViewer bool `json:"is_follow_requested_by_viewer"`
	// Whether the viewer has been invited to follow it and is yet to
	// answer.
	IsInvited bool `json:"is_invited,omitempty"`
	// On an event the viewer was invited to: when it is, so they can
	// decide. Never where to join it online.
	Event *Event `json:"event,omitempty"`
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
		AllowFollowRequest:        node.AllowFollowRequest,
		IsFollowRequestedByViewer: requested,
	}

	if node.Avatar != nil {
		limited.Avatar = NewAvatar(node.Avatar)
	}

	return limited
}
