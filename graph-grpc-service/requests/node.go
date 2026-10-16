package requests

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

type NodeRequest struct {
	Browse                  *NodeBrowseRequest
	BrowseMutualConnections *NodeBrowseMutualConnectionsRequest
	ReadByAlias             *NodeReadByAliasRequest
	BatchRead               *NodeBatchReadRequest
	BatchReadByAliases      *NodeBatchReadByAliasesRequest
	ReadConnectionStats     *NodeReadConnectionStatsRequest
	Count                   *NodeCountRequest
	CountConnections        *NodeCountConnectionsRequest
}

func NewNodeRequest() *NodeRequest {
	return &NodeRequest{
		Browse: &NodeBrowseRequest{
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			OrderingDir: constants.DirDesc,
			Ordering:    constants.OrderingRecent,
			Viewer:      entities.NewViewer(),
			Enabled:     true,
			AccessCheck: true,
		},
		BrowseMutualConnections: &NodeBrowseMutualConnectionsRequest{},
		ReadByAlias:             &NodeReadByAliasRequest{},
		BatchRead: &NodeBatchReadRequest{
			IDs: []int64{},
		},
		BatchReadByAliases: &NodeBatchReadByAliasesRequest{
			Aliases: []string{},
		},
		ReadConnectionStats: &NodeReadConnectionStatsRequest{},
		Count: &NodeCountRequest{
			Viewer:      entities.NewViewer(),
			Enabled:     true,
			AccessCheck: true,
		},
		CountConnections: &NodeCountConnectionsRequest{},
	}
}

type NodeBrowseRequest struct {
	Types           []string `validate:"max=50"`
	OwnerIDs        []int64  `validate:"numeric,omitempty"`
	ParentIDs       []int64  `validate:"numeric,omitempty"`
	Search          string   `validate:"max=255,omitempty"`
	NearbyLatitude  float64  `validate:"omitempty,numeric"`
	NearbyLongitude float64  `validate:"omitempty,numeric"`
	NearbyRadius    uint64   `validate:"omitempty,numeric"`
	Offset          uint64   `validate:"omitempty,numeric"`
	Limit           uint64   `validate:"omitempty,numeric,max=100"`
	Ordering        string   `validate:"max=10,omitempty,oneof=created_at updated_at recent popular"`
	OrderingDir     string   `validate:"max=4,omitempty,oneof=asc desc"`
	AccessCheck     bool
	Viewer          *entities.Viewer
	Enabled         bool
	// Only featured actors. See setFeaturedQuery.
	Featured bool
	// Actors only: also the ones the viewer may not see but may ask to
	// follow. Whoever asks for this has to send them on as restricted.
	IncludeAskable bool
	// Leave replies out: only rows that are not part of a thread. What a
	// list of somebody's posts wants, so that it is not filled with their
	// answers to other people.
	TopLevelOnly bool
	// The viewer's own media library: everything of theirs, with what is in
	// the library only, which every other list leaves out
	// (repositories/listed.go).
	Library bool
	// The viewer's own drafts: only those, which every other list leaves
	// out.
	Drafts bool
}

type NodeBrowseMutualConnectionsRequest struct {
	AccessCheck bool
	Viewer      *entities.Viewer
}

type NodeReadByAliasRequest struct {
	Alias    string `validate:"max=255"`
	NodeType string `validate:"max=50"`
}

type NodeBatchReadRequest struct {
	IDs         []int64 `validate:"numeric,omitempty"`
	Ordering    string  `validate:"max=10,omitempty,oneof=created_at updated_at recent popular"`
	OrderingDir string  `validate:"max=4,omitempty,oneof=asc desc"`
}

type NodeBatchReadByAliasesRequest struct {
	Aliases     []string `validate:"max=100,dive,max=255"`
	NodeType    string   `validate:"max=50"`
	Ordering    string   `validate:"max=10,omitempty,oneof=created_at updated_at recent popular"`
	OrderingDir string   `validate:"max=4,omitempty,oneof=asc desc"`
}

type NodeReadConnectionStatsRequest struct{}

type NodeCountRequest struct {
	NodeID          int64    `validate:"numeric,omitempty"`
	Types           []string `validate:"max=50"`
	OwnerIDs        []int64  `validate:"numeric,omitempty"`
	ParentIDs       []int64  `validate:"numeric,omitempty"`
	Search          string   `validate:"max=255,omitempty"`
	NearbyLatitude  float64  `validate:"omitempty,numeric"`
	NearbyLongitude float64  `validate:"omitempty,numeric"`
	NearbyRadius    uint64   `validate:"omitempty,numeric"`
	AccessCheck     bool
	Viewer          *entities.Viewer
	Enabled         bool
	// Only featured actors. See setFeaturedQuery.
	Featured bool
	// Actors only: also the ones the viewer may not see but may ask to
	// follow. Whoever asks for this has to send them on as restricted.
	IncludeAskable bool
	// Leave replies out: only rows that are not part of a thread. What a
	// list of somebody's posts wants, so that it is not filled with their
	// answers to other people.
	TopLevelOnly bool
	// The viewer's own media library: everything of theirs, with what is in
	// the library only, which every other list leaves out
	// (repositories/listed.go).
	Library bool
	// The viewer's own drafts: only those, which every other list leaves
	// out.
	Drafts bool
}

type NodeCountConnectionsRequest struct{}
