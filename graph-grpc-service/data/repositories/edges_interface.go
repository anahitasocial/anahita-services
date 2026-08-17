package repositories

import (
	"anahita-libs/data/entities"
	"graph/requests"
)

type EdgesRepoInterface interface {
	// Browse retrieves a list of edges based on the provided filters
	Browse(req *requests.EdgeBrowseRequest) ([]*entities.AnahitaEdge, error)

	// Read retrieves a single edge by its ID
	Read(id int64) (*entities.AnahitaEdge, error)

	// ReadByFilters retrieves a single edge based on provided filters
	ReadByFilters(req *requests.EdgeReadByFiltersRequest) (*entities.AnahitaEdge, error)

	// Edit updates an existing edge
	Edit(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error)

	// Add creates a new edge
	Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error)

	// Delete removes an edge by its ID
	Delete(id int64) error

	// Count returns the number of edges matching the provided filters
	Count(req *requests.EdgeCountRequest) (uint64, error)

	// Exists checks if an edge exists based on the provided criteria
	Exists(edge *entities.AnahitaEdge) (bool, error)

	// Batch operations

	// BatchRead retrieves multiple edges by their IDs
	BatchRead(ids []int64) ([]*entities.AnahitaEdge, error)

	// BatchReadByFilters retrieves multiple edges based on provided filters
	BatchReadByFilters(req *requests.EdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error)

	// BatchEdit updates multiple edges
	BatchEdit(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error)

	// BatchAdd creates multiple edges
	BatchAdd(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error)

	// BatchDelete removes multiple edges by their IDs
	BatchDelete(ids []int64) (uint32, error)

	// BatchDeleteByFilters removes multiple edges based on provided filters
	BatchDeleteByFilters(req *requests.EdgeBatchDeleteByFiltersRequest) (uint32, error)

	// BatchDeleteByNodeIDs removes multiple edges based on provided node IDs
	BatchDeleteByNodeIDs(nodeIDs []int64) (uint32, error)
}
