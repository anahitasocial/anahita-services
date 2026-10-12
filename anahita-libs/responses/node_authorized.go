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
		Download:       node.Authorized.Download,
		Composers:      node.Authorized.Composers,
		Audiences:      node.Authorized.Audiences,
	}
}

type NodeAuthorized struct {
	Edit           bool `json:"edit"`
	Delete         bool `json:"delete"`
	Administration bool `json:"administration"`

	// Left out when not worked out for this response, so the web app can
	// tell "no" from "not answered here" and keep its fallback for the
	// latter. See entities.AnahitaNodeAuthorized.
	//
	// Composers is left out when empty too: every person and group response
	// works it out, so there a missing list means none.
	Comment     *bool `json:"comment,omitempty"`
	Like        *bool `json:"like,omitempty"`
	AddFollower *bool `json:"add_follower,omitempty"`
	// Whether the viewer may invite people to follow this actor.
	Invite *bool `json:"invite,omitempty"`
	// On a reply: whether the viewer may hide it from its thread.
	Hide *bool `json:"hide,omitempty"`
	// On a post: whether the viewer may quote it.
	Quote *bool `json:"quote,omitempty"`
	// On a post: whether the viewer may pin it to the profile it is on.
	Pin *bool `json:"pin,omitempty"`
	// On a post that holds a file: whether the viewer may download it.
	Download  *bool    `json:"download,omitempty"`
	Composers []string `json:"composers,omitempty"`
	// On a person or group: who a new post there may be shown to. On a
	// post: who it may be changed to be shown to, for somebody who may edit
	// it. Widest first; left out otherwise.
	Audiences []string `json:"audiences,omitempty"`
}
