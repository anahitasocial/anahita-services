package repositories

import (
	"time"

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
	// ReadOfType is Read restricted to a type family — see the implementation
	// for why an id-only read is not enough.
	ReadOfType(id int64, typePrefix string) (*entities.AnahitaNode, error)

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

	// SoftDeleteActor marks an actor deleted and disables it, leaving everything
	// it owns in place so a restore brings the profile back whole.
	SoftDeleteActor(id int64) (bool, error)

	// RestoreActor reverses SoftDeleteActor within the grace period.
	RestoreActor(id int64) (bool, error)

	// BrowseDeletedBefore returns the ids of actors soft-deleted before the
	// cutoff — the sweeper's queue. Ids only: the sweeper reads each node it is
	// about to purge anyway, and the queue is small.
	BrowseDeletedBefore(cutoff time.Time, limit uint64) ([]int64, error)

	// PurgeActorContent removes everything an actor owns and every edge
	// touching it, but LEAVES THE ACTOR NODE so it can be tombstoned.
	PurgeActorContent(ownerID int64) (bool, error)

	// TombstoneActor strips an actor node to an identity marker: the id, the
	// handle, and the fact that it is gone.
	TombstoneActor(id int64) (bool, error)

	// BrowseActorFiles returns the actor and everything it owns that carries an
	// uploaded file, so the blobs can be deleted before the rows are.
	BrowseActorFiles(ownerID int64) ([]*entities.AnahitaNode, error)

	// Count returns the total number of nodes matching the provided browse request and viewer context.
	Count(req *requests.NodeCountRequest) (uint64, error)

	// BrowseMutualConnections retrieves mutual connections between two nodes based on the provided request and viewer context.
	BrowseMutualConnections(req *requests.NodeBrowseMutualConnectionsRequest) ([]*entities.AnahitaNode, error)

	// CountConnections returns the total number of connections for a node based on the provided request and viewer context.
	CountConnections(req *requests.NodeCountConnectionsRequest) (uint64, error)

	// ReadConnectionStats retrieves connection statistics for a node based on the provided request and viewer context.
	ReadConnectionStats(req *requests.NodeReadConnectionStatsRequest) (*responses.NodeReadConnectionStatsResponse, error)
}
