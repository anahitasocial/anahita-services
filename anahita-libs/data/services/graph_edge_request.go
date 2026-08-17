package services

import (
	"anahita-libs/data/entities"
)

type GraphEdgeRequest struct {
	Browse               *GraphEdgeBrowseRequest
	BatchReadByFilters   *GraphEdgeBatchReadByFiltersRequest
	BatchDeleteByFilters *GraphEdgeBatchDeleteByFiltersRequest
	Count                *GraphEdgeCountRequest
}

func NewGraphEdgeRequest() *GraphEdgeRequest {
	return &GraphEdgeRequest{
		Browse: &GraphEdgeBrowseRequest{
			Offset:      0,
			Limit:       20,
			OrderingDir: "desc",
			Ordering:    "created_on",
		},
		BatchReadByFilters: &GraphEdgeBatchReadByFiltersRequest{
			OrderingDir: "desc",
			Ordering:    "created_on",
		},
		BatchDeleteByFilters: &GraphEdgeBatchDeleteByFiltersRequest{},
		Count:                &GraphEdgeCountRequest{},
	}
}

type GraphEdgeBrowseRequest struct {
	NodeAID     int64            `validate:"omitempty,numeric"`
	NodeAType   string           `validate:"omitempty,max=50"`
	Types       []string         `validate:"max=50"`
	NodeBID     int64            `validate:"omitempty,numeric"`
	NodeBType   string           `validate:"omitempty,max=50"`
	Limit       uint64           `validate:"omitempty,numeric,max=100"`
	Offset      uint64           `validate:"omitempty,numeric"`
	Ordering    string           `validate:"max=10,omitempty,oneof=created_on updated_on ordering"`
	OrderingDir string           `validate:"max=4,omitempty,oneof=asc desc"`
	Viewer      *entities.Viewer `validate:"-"`
}

type GraphEdgeCountRequest struct {
	NodeAID   int64    `validate:"omitempty,numeric"`
	NodeAType string   `validate:"omitempty,max=50"`
	Types     []string `validate:"max=50"`
	NodeBID   int64    `validate:"omitempty,numeric"`
	NodeBType string   `validate:"omitempty,max=50"`
}

type GraphEdgeBatchReadByFiltersRequest struct {
	NodeAID     int64    `validate:"omitempty,numeric"`
	NodeAType   string   `validate:"omitempty,max=50"`
	Types       []string `validate:"max=50"`
	NodeBID     int64    `validate:"omitempty,numeric"`
	NodeBType   string   `validate:"omitempty,max=50"`
	Ordering    string   `validate:"max=10,omitempty,oneof=created_on updated_on ordering"`
	OrderingDir string   `validate:"max=4,omitempty,oneof=asc desc"`
}

type GraphEdgeBatchDeleteByFiltersRequest struct {
	NodeAID   int64    `validate:"omitempty,numeric"`
	NodeAType string   `validate:"omitempty,max=50"`
	Types     []string `validate:"max=50"`
	NodeBID   int64    `validate:"omitempty,numeric"`
	NodeBType string   `validate:"omitempty,max=50"`
}
