package repositories

import (
	"anahita-libs/constants"
	"context"
	"database/sql"
	"time"
)

// Deletion actions. One node normally produces up to three rows, and together
// they are its story: scheduled, then either restored or purged.
const (
	DeletionActionScheduled = "scheduled"
	DeletionActionPurged    = "purged"
	DeletionActionRestored  = "restored"
)

// NodeDeletionLogsRepoInterface records who deleted what, and when.
//
// Deletion used to leave nothing behind: the event person-service published had
// no consumer, group-service published none at all, and the only trace was a
// line in a container's stdout. For a self-service deletion that is merely
// untidy; for an administrator removing somebody else's account it means the
// question "who did this" has no answer.
//
// Deliberately keyed on a NODE, not an actor. Nothing here is specific to
// profiles — a deleted photo, article or comment wants the same record for the
// same reason, and `nodeType` carries enough to tell them apart. Actors are
// simply the first thing to write to it.
type NodeDeletionLogsRepoInterface interface {
	// Add records one deletion action against a node.
	Add(nodeID int64, alias, nodeType string, deletedBy int64, action string) error
}

func NewNodeDeletionLogsRepo(db *sql.DB) NodeDeletionLogsRepoInterface {
	return &nodeDeletionLogs{db: db, dbTimeout: constants.DBTimeout}
}

type nodeDeletionLogs struct {
	db        *sql.DB
	dbTimeout time.Duration
}

// Add writes one row.
//
// The alias is stored alongside the id rather than looked up later, because
// later there may be nothing to look up: after the sweeper runs the node is a
// tombstone, and a verified erasure request can scrub even that. This row is
// then the only record of which alias went.
//
// created_on is set here rather than defaulted in the schema so the timestamp
// comes from the service that observed the event, not from whichever database
// clock happens to answer.
func (repo *nodeDeletionLogs) Add(nodeID int64, alias, nodeType string, deletedBy int64, action string) error {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
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
