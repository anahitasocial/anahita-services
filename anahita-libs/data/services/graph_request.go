package services

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

type GraphRequest struct {
	BrowseNeighbours      *GrapBrowseNeighboursRequest
	BrowseConnectionNodes *GraphBrowseConnectionNodesRequest
	BrowseOrderedByDegree *GraphBrowseOrderedByDegreeRequest
	ReadNeighbourhood     *GraphReadNeighbourhoodRequest
	Traverse              *GraphTraverseRequest
	ReadShortestPath      *GraphReadShortestPathRequest
	Trending              *GraphTrendingRequest
}

func NewGraphRequest() *GraphRequest {
	return &GraphRequest{
		BrowseNeighbours: &GrapBrowseNeighboursRequest{
			Direction:   constants.GraphQueryDirectionOutbound,
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			Ordering:    constants.OrderingRecent,
			OrderingDir: constants.DirDesc,
		},
		BrowseConnectionNodes: &GraphBrowseConnectionNodesRequest{
			Direction:   constants.GraphQueryDirectionOutbound,
			Distinct:    true,
			Limit:       constants.RequestLimitDefault,
			Offset:      0,
			Ordering:    constants.OrderingRecent,
			OrderingDir: constants.DirDesc,
		},
		BrowseOrderedByDegree: &GraphBrowseOrderedByDegreeRequest{
			Direction:   constants.GraphQueryDirectionOutbound,
			Limit:       constants.RequestLimitDefault,
			Offset:      0,
			OrderingDir: constants.DirDesc,
		},
		ReadNeighbourhood: &GraphReadNeighbourhoodRequest{
			Direction: constants.GraphQueryDirectionOutbound,
		},
		Traverse: &GraphTraverseRequest{
			Direction: constants.GraphQueryDirectionOutbound,
		},
		ReadShortestPath: &GraphReadShortestPathRequest{
			Direction: constants.GraphQueryDirectionOutbound,
		},
		Trending: &GraphTrendingRequest{
			Direction: constants.GraphQueryDirectionOutbound,
			Limit:     constants.RequestLimitDefault,
		},
	}
}

type GrapBrowseNeighboursRequest struct {
	NodeId      int64    `validate:"required,numeric"`
	EdgeTypes   []string `validate:"string,omitempty"`
	Direction   string   `validate:"required,oneof=IN OUT BOTH"`
	Offset      uint64   `validate:"omitempty,numeric"`
	Limit       uint64   `validate:"omitempty,numeric,max=100"`
	Ordering    string   `validate:"max=10,omitempty,oneof=created_at updated_at recent popular connected"`
	OrderingDir string   `validate:"max=4,omitempty,oneof=asc desc"`
	Viewer      *entities.Viewer
	AccessCheck bool
}

type GraphBrowseConnectionNodesRequest struct {
	NodeId    int64  `validate:"required,numeric"`
	EdgeType  string `validate:"string,omitempty"`
	Direction string `validate:"required,oneof=IN OUT BOTH"`
	Hops      uint32 `validate:"required,numeric"`
	Distinct  bool
	// Only the ones this actor follows. Asked of a profile's followers, it
	// gives who that actor and the profile have in common.
	FollowedBy int64
	// Only the ones whose name or alias holds this.
	Search      string
	Limit       uint64 `validate:"omitempty,numeric,max=100"`
	Offset      uint64 `validate:"omitempty,numeric"`
	Ordering    string `validate:"max=10,omitempty,oneof=created_at updated_at recent popular connected"`
	OrderingDir string `validate:"max=4,omitempty,oneof=asc desc"`
	Viewer      *entities.Viewer
	AccessCheck bool
}

type GraphBrowseOrderedByDegreeRequest struct {
	NodeType    string   `validate:"string,omitempty"`
	EdgeTypes   []string `validate:"string,omitempty"`
	Direction   string   `validate:"required,oneof=IN OUT BOTH"`
	Limit       uint64   `validate:"omitempty,numeric,max=100"`
	Offset      uint64   `validate:"omitempty,numeric"`
	OrderingDir string   `validate:"max=4,omitempty,oneof=ASC DESC"`
	Viewer      *entities.Viewer
	AccessCheck bool
}

type GraphReadNeighbourhoodRequest struct {
	NodeId      int64    `validate:"required,numeric"`
	EdgeTypes   []string `validate:"string,omitempty"`
	Depth       uint32   `validate:"required,numeric,max=5"`
	Direction   string   `validate:"required,oneof=IN OUT BOTH"`
	Viewer      *entities.Viewer
	AccessCheck bool
}

type GraphTraverseRequest struct {
	StartNodeId int64    `validate:"required,numeric"`
	EdgeTypes   []string `validate:"string,omitempty"`
	Direction   string   `validate:"required,oneof=IN OUT BOTH"`
	MaxDepth    uint32   `validate:"required,numeric,max=5"`
	Strategy    string   `validate:"required,oneof=BFS DFS"`
	Limit       uint64   `validate:"omitempty,numeric,max=100"`
}

type GraphReadShortestPathRequest struct {
	StartNodeId int64    `validate:"required,numeric"`
	EndNodeId   int64    `validate:"required,numeric"`
	EdgeTypes   []string `validate:"string,omitempty"`
	Direction   string   `validate:"required,oneof=IN OUT BOTH"`
	MaxDepth    uint32   `validate:"required,numeric,max=5"`
}

type GraphTrendingRequest struct {
	NodeType    string   `validate:"string,omitempty"`
	EdgeTypes   []string `validate:"string,omitempty"`
	Direction   string   `validate:"required,oneof=IN OUT BOTH"`
	TimeWindow  string   `validate:"required,oneof=1h 6h 24h DAILY WEEKLY MONTHLY YEARLY"`
	Limit       uint64   `validate:"omitempty,numeric,max=100"`
	Viewer      *entities.Viewer
	AccessCheck bool
}
