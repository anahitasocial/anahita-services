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
	}
}

type NodeAuthorized struct {
	Edit           bool `json:"edit"`
	Delete         bool `json:"delete"`
	Administration bool `json:"administration"`
}
