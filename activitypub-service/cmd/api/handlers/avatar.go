package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/federation"
)

// The pictures of people and posts on other servers are copied into this
// installation's storage and never shown from where they came from. How
// is the same for every protocol: anahita-libs/federation.

// Pictures is what fetches a picture from another server.
type Pictures = federation.Pictures

// PhotoFiles is where a post's list of pictures is kept: the graph.
type PhotoFiles = federation.PhotoFiles

func (h *Handlers) keeper() *federation.Keeper {
	keeper := &federation.Keeper{Nodes: h.Nodes, Files: h.Files, PhotoFiles: h.PhotoFiles, Pictures: h.Pictures}
	if h.Blocks != nil {
		keeper.Refused = h.Blocks.IsBlocked
	}
	return keeper
}

func (h *Handlers) copyAvatar(nodeID int64, address string) {
	h.keeper().CopyAvatar(nodeID, address)
}

func (h *Handlers) copyCover(nodeID int64, address string) {
	h.keeper().CopyCover(nodeID, address)
}

func (h *Handlers) deleteAvatar(node *entities.AnahitaNode) {
	h.keeper().DeleteActorPictures(node)
}
