package repositories

import (
	"context"

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

// DeleteSubtree deletes rootID, all descendants via parent_id, and all edges attached to any of them.
func (r *nodesRepo) DeleteSubtree(rootID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if rootID <= 0 {
		return false, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const query = `
WITH RECURSIVE subtree (id) AS (
  SELECT id
  FROM nodes
  WHERE id = ?

  UNION ALL

  SELECT c.id
  FROM nodes c
  JOIN subtree s ON c.parent_id = s.id
)
DELETE n, e
FROM subtree s
JOIN nodes n ON n.id = s.id
LEFT JOIN edges e
  ON e.node_a_id = n.id
  OR e.node_b_id = n.id;
`

	res, err := tx.ExecContext(ctx, query, rootID)
	if err != nil {
		return false, err
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// PurgeByOwner deletes the owner node, all nodes owned by it, all descendants via parent_id,
// and all edges connected to any of those nodes (including the owner).
func (r *nodesRepo) PurgeByOwner(ownerID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if ownerID <= 0 {
		return false, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const deleteEdges = `
WITH RECURSIVE content (id) AS (
  SELECT id
  FROM nodes
  WHERE owner_id = ?

  UNION ALL

  SELECT c.id
  FROM nodes c
  JOIN content p ON c.parent_id = p.id
),
targets (id) AS (
  SELECT id FROM content
  UNION ALL
  SELECT ?  -- owner_id
)
DELETE e
FROM edges e
JOIN targets t
  ON e.node_a_id = t.id
  OR e.node_b_id = t.id;
`

	if _, err = tx.ExecContext(ctx, deleteEdges, ownerID, ownerID); err != nil {
		return false, err
	}

	const deleteContentNodes = `
WITH RECURSIVE content (id) AS (
  SELECT id
  FROM nodes
  WHERE owner_id = ?

  UNION ALL

  SELECT c.id
  FROM nodes c
  JOIN content p ON c.parent_id = p.id
)
DELETE n
FROM nodes n
JOIN content c ON n.id = c.id;
`
	if _, err = tx.ExecContext(ctx, deleteContentNodes, ownerID); err != nil {
		return false, err
	}

	const deleteOwner = `DELETE FROM nodes WHERE id = ?;`
	if _, err = tx.ExecContext(ctx, deleteOwner, ownerID); err != nil {
		return false, err
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}
