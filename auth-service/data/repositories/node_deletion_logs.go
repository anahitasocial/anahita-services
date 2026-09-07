package repositories

import (
	"context"
	"database/sql"
	"time"
)

// Deletion audit actions. Mirrors the constants in graph-grpc-service, which
// writes the other two — scheduled when the grace period starts, purged when
// the sweeper erases the account.
//
// The table is shared rather than owned by one service, matching how `nodes`
// and `people_people` already work here: one database, several writers. It is
// keyed on a node rather than an actor, so media and comments can use it too.
const (
	DeletionActionRestored = "restored"
)

// NodeDeletionLogsInterface records deletion-lifecycle actions.
//
// auth-service writes only the restore, because restoring is the one step that
// happens at login and so belongs to this service. Scheduling and purging are
// written by graph-grpc-service where they occur.
type NodeDeletionLogsInterface interface {
	Add(nodeID int64, alias, nodeType string, deletedBy int64, action string) error
}

func NewNodeDeletionLogs(db *sql.DB) NodeDeletionLogsInterface {
	return &nodeDeletionLogs{db: db}
}

type nodeDeletionLogs struct {
	db *sql.DB
}

// Add writes one row.
//
// The alias is stored alongside the id rather than looked up later: after the
// sweeper runs the node is a tombstone, and a verified erasure request can
// scrub even that, leaving this as the only record of which alias went.
func (repo *nodeDeletionLogs) Add(nodeID int64, alias, nodeType string, deletedBy int64, action string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	const query = `
INSERT INTO node_deletion_logs
  (node_id, node_alias, node_type, deleted_by, action, created_on)
VALUES (?, ?, ?, ?, ?, ?);
`

	_, err := repo.db.ExecContext(ctx, query,
		nodeID, alias, nodeType, deletedBy, action, time.Now().UTC())

	return err
}
