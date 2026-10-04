package services

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

type GraphNodeRequest struct {
	Browse             *GraphNodeBrowseRequest
	Read               *GraphNodeReadRequest
	ReadByAlias        *GraphNodeReadByAlias
	BatchRead          *GraphNodeBatchReadRequest
	BatchReadByAliases *GraphNodeBatchReadByAliasesRequest
	BatchDelete        *GraphNodeBatchDeleteRequest
}

func NewGraphNodeRequest() *GraphNodeRequest {
	return &GraphNodeRequest{
		Browse: &GraphNodeBrowseRequest{
			NearbyLatitude:  0,
			NearbyLongitude: 0,
			NearbyRadius:    0,
			Offset:          0,
			Limit:           constants.RequestLimitDefault,
			OrderingDir:     constants.DirDesc,
			Ordering:        constants.OrderingRecent,
			Viewer:          entities.NewViewer(),
			AccessCheck:     false,
		},
		Read:        &GraphNodeReadRequest{},
		ReadByAlias: &GraphNodeReadByAlias{},
		BatchRead: &GraphNodeBatchReadRequest{
			Viewer:      entities.NewViewer(),
			AccessCheck: false,
		},
		BatchReadByAliases: &GraphNodeBatchReadByAliasesRequest{
			Viewer:      entities.NewViewer(),
			AccessCheck: false,
		},
		BatchDelete: &GraphNodeBatchDeleteRequest{},
	}
}

type GraphNodeBrowseRequest struct {
	Types           []string         `validate:"max=50"`
	OwnerIDs        []int64          `validate:"numeric,omitempty"`
	ParentIDs       []int64          `validate:"numeric,omitempty"`
	NearbyLatitude  float64          `validate:"omitempty"`
	NearbyLongitude float64          `validate:"omitempty"`
	NearbyRadius    uint64           `validate:"omitempty,numeric"`
	Search          string           `validate:"max=100,omitempty"`
	Offset          uint64           `validate:"omitempty,numeric"`
	Limit           uint64           `validate:"omitempty,numeric,max=100"`
	Ordering        string           `validate:"max=10,omitempty,oneof=created_at updated_at recent popular"`
	OrderingDir     string           `validate:"max=4,omitempty,oneof=asc desc"`
	Viewer          *entities.Viewer `validate:"required"`
	AccessCheck     bool
}

type GraphNodeReadRequest struct {
	ID int64 `validate:"required,numeric"`
}

type GraphNodeReadByAlias struct {
	Alias    string `validate:"required,max=100"`
	NodeType string `validate:"max=50,omitempty"`
}

type GraphNodeBatchReadRequest struct {
	IDs         []int64          `validate:"required,dive,numeric"`
	Ordering    string           `validate:"max=10,omitempty,oneof=created_at updated_at recent popular"`
	OrderingDir string           `validate:"max=4,omitempty,oneof=asc desc"`
	Viewer      *entities.Viewer `validate:"required"`
	AccessCheck bool
}

type GraphNodeBatchReadByAliasesRequest struct {
	Aliases     []string         `validate:"required,dive,max=100"`
	NodeType    string           `validate:"max=50,omitempty"`
	Ordering    string           `validate:"max=10,omitempty,oneof=created_at updated_at recent popular"`
	OrderingDir string           `validate:"max=4,omitempty,oneof=asc desc"`
	Viewer      *entities.Viewer `validate:"required"`
	AccessCheck bool
}

type GraphNodeBatchDeleteRequest struct {
	IDs []int64 `validate:"required,dive,numeric"`
}
