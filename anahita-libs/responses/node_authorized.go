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
	Comment     *bool    `json:"comment,omitempty"`
	Like        *bool    `json:"like,omitempty"`
	AddFollower *bool    `json:"add_follower,omitempty"`
	Composers   []string `json:"composers,omitempty"`
	// Who a new post here may be shown to, widest first. Left out for
	// somebody who is not signed in.
	Audiences []string `json:"audiences,omitempty"`
}
