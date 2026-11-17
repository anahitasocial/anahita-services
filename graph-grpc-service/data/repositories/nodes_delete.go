package repositories

import (
	"anahita-libs/data/entities"
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

// Delete removes a node based on the provided delete request and viewer context.
func (r *nodesRepo) Delete(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if id <= 0 {
		return false, nil
	}

	query := sq.Delete("nodes").Where(sq.Eq{"id": id})

	sql, args, err := query.ToSql()
	if err != nil {
		return false, err
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return false, err
	}

	// Delete Children Nodes
	_, err = r.DeleteChildren([]int64{id})
	if err != nil {
		return false, err
	}

	// Delete associated edges
	_, err = r.edgesRepo.BatchDeleteByNodeIDs([]int64{id})
	if err != nil {
		return false, err
	}

	return true, nil
}

// BatchDelete removes a node based on the provided delete request and viewer context.
func (r *nodesRepo) BatchDelete(ids []int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if len(ids) == 0 {
		return false, nil
	}

	query := sq.Delete("nodes").Where(sq.Eq{"id": ids})

	sql, args, err := query.ToSql()
	if err != nil {
		return false, err
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return false, err
	}

	// Delete Children Nodes
	_, err = r.DeleteChildren(ids)
	if err != nil {
		return false, err
	}

	// Delete associated edges
	_, err = r.edgesRepo.BatchDeleteByNodeIDs(ids)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Delete associated children nodes
func (r *nodesRepo) DeleteChildren(parentIDs []int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if len(parentIDs) == 0 {
		return false, nil
	}

	// First, get the IDs of children nodes to delete their edges
	childQuery := sq.Select("id").From("nodes").Where(sq.Eq{"parent_id": parentIDs})
	childSQL, childArgs, err := childQuery.ToSql()
	if err != nil {
		return false, err
	}

	rows, err := r.db.QueryContext(ctx, childSQL, childArgs...)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var childIDs []int64
	for rows.Next() {
		var childID int64
		if err := rows.Scan(&childID); err != nil {
			return false, err
		}
		childIDs = append(childIDs, childID)
	}

	if len(childIDs) > 0 {
		// Delete associated edges of children
		_, err = r.edgesRepo.BatchDeleteByNodeIDs(childIDs)
		if err != nil {
			return false, err
		}
	}

	// Delete children nodes
	query := sq.Delete("nodes").Where(sq.Eq{"parent_id": parentIDs})

	sql, args, err := query.ToSql()
	if err != nil {
		return false, err
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return false, err
	}

	return true, nil
}

// subtreeNodes is rootID and everything under it by parent_id: a post and
// its whole thread.
const subtreeNodes = `
WITH RECURSIVE subtree (id) AS (
  SELECT id
  FROM nodes
  WHERE id = ?

  UNION ALL

  SELECT c.id
  FROM nodes c
  JOIN subtree s ON c.parent_id = s.id
)`

// subtreeRead is what is told of each node that goes: enough for the
// event that says it went.
const subtreeRead = subtreeNodes + `
SELECT
  n.id,
  n.type,
  n.service,
  n.owner_id,
  n.owner_type
FROM subtree s
JOIN nodes n ON n.id = s.id
ORDER BY n.id`

const subtreeDelete = subtreeNodes + `
DELETE n, e
FROM subtree s
JOIN nodes n ON n.id = s.id
LEFT JOIN edges e
  ON e.node_a_id = n.id
  OR e.node_b_id = n.id`

// DeleteSubtree deletes rootID, all descendants via parent_id, and all
// edges attached to any of them. It answers with every node it deleted,
// the root first, so that each can be said to have gone: a thread's later
// parts are copies elsewhere too, on Bluesky and the fediverse.
func (r *nodesRepo) DeleteSubtree(rootID int64) ([]*entities.AnahitaNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if rootID <= 0 {
		return nil, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var gone []*entities.AnahitaNode
	gone, err = readSubtree(ctx, tx, rootID)
	if err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, subtreeDelete, rootID); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return gone, nil
}

// readSubtree is each node DeleteSubtree is about to delete, in id order,
// which puts the root, the oldest, first.
func readSubtree(ctx context.Context, tx *sql.Tx, rootID int64) ([]*entities.AnahitaNode, error) {
	rows, err := tx.QueryContext(ctx, subtreeRead, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*entities.AnahitaNode
	for rows.Next() {
		var (
			node      = &entities.AnahitaNode{}
			service   sql.NullString
			ownerType sql.NullString
			ownerID   sql.NullInt64
		)
		if err := rows.Scan(&node.ID, &node.Type, &service, &ownerID, &ownerType); err != nil {
			return nil, err
		}
		node.Service = service.String
		node.OwnerID = ownerID.Int64
		node.OwnerType = ownerType.String
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}
