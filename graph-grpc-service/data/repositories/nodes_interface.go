package repositories

import (
	"anahita-libs/data/entities"
	"graph/requests"
	"graph/responses"
)

// NodesRepoInterface defines the methods for interacting with nodes in the database.
type NodesRepoInterface interface {
	// Browse retrieves a list of nodes based on the provided browse request and viewer context.
	Browse(req *requests.NodeBrowseRequest) ([]*entities.AnahitaNode, error)

	// Read retrieves a single node based on the provided read request and viewer context.
	Read(id int64) (*entities.AnahitaNode, error)

	// ReadByAlias retrieves a single node based on the provided alias and viewer context.
	ReadByAlias(req *requests.NodeReadByAliasRequest) (*entities.AnahitaNode, error)

	// Edit updates an existing node based on the provided edit request and viewer context.
	Edit(node *entities.AnahitaNode) (*entities.AnahitaNode, error)

	// Add creates a new node based on the provided add request and viewer context.
	Add(node *entities.AnahitaNode) (*entities.AnahitaNode, error)

	// Delete removes a node based on the provided delete request and viewer context.
	Delete(id int64) (bool, error)

	// BatchRead retrieves multiple nodes based on the provided read request and viewer context.
	BatchRead(req *requests.NodeBatchReadRequest) ([]*entities.AnahitaNode, error)

	// BatchReadByAliases retrieves multiple nodes based on the provided aliases and viewer context.
	BatchReadByAliases(req *requests.NodeBatchReadByAliasesRequest) ([]*entities.AnahitaNode, error)

	// BatchEdit updates multiple existing nodes based on the provided entities.
	BatchEdit(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error)

	// BatchAdd creates multiple new nodes based on the provided nodes.
	BatchAdd(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error)

	// BatchDelete removes multiple nodes based on the provided delete request and viewer context.
	BatchDelete(ids []int64) (bool, error)

	// DeleteSubtree deletes rootID, all descendants via parent_id, and all edges attached to any of them.
	DeleteSubtree(rootID int64) (bool, error)

	// PurgeByOwner deletes the owner node, all nodes owned by it, all descendants via parent_id,
	// and all edges connected to any of those nodes (including the owner).
	PurgeByOwner(ownerID int64) (bool, error)

	// Count returns the total number of nodes matching the provided browse request and viewer context.
	Count(req *requests.NodeCountRequest) (uint64, error)

	// BrowseMutualConnections retrieves mutual connections between two nodes based on the provided request and viewer context.
	BrowseMutualConnections(req *requests.NodeBrowseMutualConnectionsRequest) ([]*entities.AnahitaNode, error)

	// CountConnections returns the total number of connections for a node based on the provided request and viewer context.
	CountConnections(req *requests.NodeCountConnectionsRequest) (uint64, error)

	// ReadConnectionStats retrieves connection statistics for a node based on the provided request and viewer context.
	ReadConnectionStats(req *requests.NodeReadConnectionStatsRequest) (*responses.NodeReadConnectionStatsResponse, error)
}
