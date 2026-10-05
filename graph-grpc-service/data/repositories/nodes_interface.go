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

	// FilterVisible returns which of the given nodes the viewer may see.
	FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error)

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

	// ReadDeletionCounts totals what deleting this actor would take with it,
	// for the confirmation dialog.
	ReadDeletionCounts(actorID int64) (*DeletionCounts, error)

	// ArchiveActor marks an actor archived — permanent, and destroys nothing.
	ArchiveActor(id int64) (bool, error)

	// SetEnabled turns an actor on or off. Reversible, and the only thing that
	// writes that column on its own.
	SetEnabled(id int64, enabled bool) (bool, error)

	// SetFeatured features or unfeatures an actor. Refuses to feature one that
	// is disabled, archived or deleted with ErrNotFeaturable.
	SetFeatured(id int64, featured bool) (bool, error)

	// ClearFile takes an avatar or a cover off a node, which Edit cannot
	// do. See nodes_clear_file.go.
	ClearFile(id int64, kind string, modifiedBy int64) (bool, error)

	// TouchLastVisit records that an actor just signed in. See
	// nodes_last_visit.go for why it is not a field on Edit.
	TouchLastVisit(id int64) (bool, error)

	// SoftDeleteActor marks an actor deleted and disables it, leaving everything
	// it owns in place so a restore brings the profile back whole.
	SoftDeleteActor(id int64) (bool, error)

	// RestoreActor reverses SoftDeleteActor within the grace period.
	RestoreActor(id int64) (bool, error)

	// BrowseDeletedBefore returns the ids of actors soft-deleted before the
	// cutoff and not yet purged — the sweeper's queue. Ids only: the sweeper reads each node it is
	// about to purge anyway, and the queue is small.
	BrowseDeletedBefore(cutoff time.Time, limit uint64) ([]int64, error)

	// PreparePurge writes down what a purge of this actor has to do: the nodes
	// to remove, and the nodes whose counters must be counted again afterwards.
	// Refuses the last super administrator with ErrLastSuperAdmin.
	PreparePurge(actorID int64) error

	// NextPurgeBatch returns the next nodes to remove, or nil when none are
	// left.
	NextPurgeBatch(actorID int64, limit int) (*PurgeBatch, error)

	// DeletePurgeBatch removes one batch, with every edge and notification
	// that points at it.
	DeletePurgeBatch(actorID int64, batch *PurgeBatch) error

	// FinishPurge removes what points at the actor itself and counts again
	// what other people's nodes lost. LEAVES THE ACTOR NODE so it can be
	// tombstoned.
	FinishPurge(actorID int64) error

	// GuardLastSuperAdmin returns ErrLastSuperAdmin if removing this actor
	// would leave nobody able to administer the installation.
	GuardLastSuperAdmin(id int64) error

	// IsPurged reports whether an actor is already a tombstone.
	IsPurged(id int64) (bool, error)

	// TombstoneActor strips an actor node to an identity marker: the id, the
	// handle, and the fact that it is gone.
	TombstoneActor(id int64) (bool, error)

	// Count returns the total number of nodes matching the provided browse request and viewer context.
	Count(req *requests.NodeCountRequest) (uint64, error)

	// BrowseMutualConnections retrieves mutual connections between two nodes based on the provided request and viewer context.
	BrowseMutualConnections(req *requests.NodeBrowseMutualConnectionsRequest) ([]*entities.AnahitaNode, error)

	// CountConnections returns the total number of connections for a node based on the provided request and viewer context.
	CountConnections(req *requests.NodeCountConnectionsRequest) (uint64, error)

	// ReadConnectionStats retrieves connection statistics for a node based on the provided request and viewer context.
	ReadConnectionStats(req *requests.NodeReadConnectionStatsRequest) (*responses.NodeReadConnectionStatsResponse, error)
}
