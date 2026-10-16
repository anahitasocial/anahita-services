package responses

import "anahita-libs/data/entities"

func NewNodeAuthorized(node *entities.AnahitaNode) *NodeAuthorized {
	if node == nil || node.Authorized == nil {
		return nil
	}

	return &NodeAuthorized{
		Edit:           node.Authorized.Edit,
		Delete:         node.Authorized.Delete,
		Administration: node.Authorized.Administration,
		Comment:        node.Authorized.Comment,
		Like:           node.Authorized.Like,
		AddFollower:    node.Authorized.AddFollower,
		Invite:         node.Authorized.Invite,
		Hide:           node.Authorized.Hide,
		Quote:          node.Authorized.Quote,
		Pin:            node.Authorized.Pin,
		BlockFromOwner: node.Authorized.BlockFromOwner,
		Download:       node.Authorized.Download,
		Composers:      node.Authorized.Composers,
		Audiences:      node.Authorized.Audiences,
	}
}

// NodeAuthorized is what the viewer may do with what it is sent with, as the
// server's own rules answer. edit, delete and administration are always
// there. The rest are left out when not worked out for this answer, so that
// a client can tell "no" from "not answered here" (see
// entities.AnahitaNodeAuthorized); composers is left out when empty too,
// since every person and group answer works it out.
type NodeAuthorized struct {
	// Whether the viewer may change it.
	Edit bool `json:"edit"`
	// Whether the viewer may delete it.
	Delete bool `json:"delete"`
	// Whether the viewer may look after it: on a profile, its settings.
	Administration bool `json:"administration"`

	// Whether the viewer may reply to it.
	Comment *bool `json:"comment,omitempty"`
	// Whether the viewer may like it.
	Like *bool `json:"like,omitempty"`
	// On a profile: whether the viewer may add a follower to it.
	AddFollower *bool `json:"add_follower,omitempty"`
	// Whether the viewer may invite people to follow this actor.
	Invite *bool `json:"invite,omitempty"`
	// On a reply: whether the viewer may hide it from its thread.
	Hide *bool `json:"hide,omitempty"`
	// On a post: whether the viewer may quote it.
	Quote *bool `json:"quote,omitempty"`
	// On a post: whether the viewer may pin it to the profile it is on.
	Pin *bool `json:"pin,omitempty"`
	// On a post or a reply in a group or an event: whether the viewer may
	// block its writer from there, as the group.
	BlockFromOwner *bool `json:"block_from_owner,omitempty"`
	// On a post that holds a file: whether the viewer may download it.
	Download *bool `json:"download,omitempty"`
	// On a profile: the kinds of post the viewer may make on it.
	Composers []string `json:"composers,omitempty"`
	// On a person or group: who a new post there may be shown to. On a
	// post: who it may be changed to be shown to, for somebody who may edit
	// it. Widest first; left out otherwise.
	Audiences []string `json:"audiences,omitempty"`
}
