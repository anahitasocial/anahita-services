package entities

import "strings"

func (a *AnahitaNode) IsActor() bool {
	return strings.Contains(a.Type, "node.actor.")
}

func (a *AnahitaNode) IsPerson() bool {
	return strings.Contains(a.Type, ".person.")
}

// IsRemote says whether the node came from another server.
func (a *AnahitaNode) IsRemote() bool {
	return a.URI != ""
}

func (a *AnahitaNode) IsMedium() bool {
	return strings.Contains(a.Type, "node.medium.")
}
