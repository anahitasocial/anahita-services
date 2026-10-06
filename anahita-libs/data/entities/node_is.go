package entities

import "strings"

func (a *AnahitaNode) IsActor() bool {
	return strings.Contains(a.Type, "node.actor.")
}

func (a *AnahitaNode) IsPerson() bool {
	return strings.Contains(a.Type, ".person.")
}

func (a *AnahitaNode) IsMedium() bool {
	return strings.Contains(a.Type, "node.medium.")
}
