package dtos

import (
	"anahita-libs/data/entities"
	"time"
)

type BrowseNeighboursResponse struct {
	Nodes []*entities.AnahitaNode
	Total uint64
}

type BrowseConnectionNodesResponse struct {
	Nodes []*entities.AnahitaNode
	Total uint64
}

type ReadNeighbourhoodResp struct {
	Center     *entities.AnahitaNode
	Neighbours []*entities.AnahitaNode
	Edges      []*entities.AnahitaEdge
	CreatedAt  time.Time
}

type TraverseResponse struct {
	Nodes        []*entities.AnahitaNode
	Edges        []*entities.AnahitaEdge
	DepthReached uint32
	CreatedAt    time.Time
}

type ReadShortestPathResponse struct {
	Nodes     []*entities.AnahitaNode
	Edges     []*entities.AnahitaEdge
	Hops      uint32
	Found     bool
	CreatedAt time.Time
	Strategy  string
}
