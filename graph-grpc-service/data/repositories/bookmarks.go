package repositories

import (
	"context"
	"database/sql"

	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// BookmarksRepoInterface is saving a post to find it again.
//
// A saved post is an edge from the person to the post. It is PRIVATE: every
// method here takes the person it is about, and none answers about anybody
// else. Nothing is counted on the post, no event is published and nobody is
// told, so the post's author cannot find out who saved it or that anybody
// did.
type BookmarksRepoInterface interface {
	// Set saves a post for a person, or takes it out of what they saved.
	// Doing either twice is the same as doing it once.
	Set(personID int64, post *entities.AnahitaNode, saved bool) error

	// IsSavedMany reports which of these posts the person has saved.
	IsSavedMany(personID int64, ids []int64) (map[int64]bool, error)

	// List returns what a person saved, most recently saved first, as that
	// person may see it now.
	List(viewer *entities.Viewer, limit uint64, offset uint64) ([]*entities.AnahitaNode, error)

	// Count is how many of them there are to list.
	Count(viewer *entities.Viewer) (uint64, error)
}

func NewBookmarksRepo(db *sql.DB) BookmarksRepoInterface {
	return &bookmarksRepo{db: db}
}

type bookmarksRepo struct {
	db *sql.DB
}

const deleteBookmark = `
DELETE FROM edges
WHERE type = ?
  AND node_a_id = ?
  AND node_b_id = ?`

const insertBookmark = `
INSERT INTO edges (
  type,
  node_a_id,
  node_a_type,
  node_b_id,
  node_b_type,
  created_at,
  created_by,
  updated_at,
  modified_by
)
VALUES (?, ?, ?, ?, ?, UTC_TIMESTAMP(), ?, UTC_TIMESTAMP(), ?)`

// Set removes any edge there is and, to save, writes one, in one
// transaction. Saving twice is saving once, and the date is the last time.
func (r *bookmarksRepo) Set(personID int64, post *entities.AnahitaNode, saved bool) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, deleteBookmark,
		constants.EdgeBaseBookmarkEntityType, personID, post.ID,
	); err != nil {
		return err
	}

	if saved {
		if _, err = tx.ExecContext(ctx, insertBookmark,
			constants.EdgeBaseBookmarkEntityType,
			personID, constants.NodeActorPersonEntityType,
			post.ID, post.Type,
			personID, personID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *bookmarksRepo) IsSavedMany(personID int64, ids []int64) (map[int64]bool, error) {
	saved := map[int64]bool{}
	if personID <= 0 || len(ids) == 0 {
		return saved, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	query, args, err := sq.Select("node_b_id").
		From("edges").
		Where(sq.Eq{
			"type":      constants.EdgeBaseBookmarkEntityType,
			"node_a_id": personID,
			"node_b_id": ids,
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		saved[id] = true
	}

	return saved, rows.Err()
}

// savedBy joins a query over posts to what one person saved.
func savedBy(query sq.SelectBuilder, personID int64) sq.SelectBuilder {
	return query.Join(
		"edges AS saved ON saved.node_b_id = node.id AND saved.type = ? AND saved.node_a_id = ?",
		constants.EdgeBaseBookmarkEntityType, personID,
	)
}

// List returns what the viewer saved.
//
// Through the gate a post is read through: a post saved while it was public
// and since made private, or whose author has since blocked the viewer, is
// no longer theirs to see, and saving it does not keep it open. It is left
// out, and comes back if it is opened up again. A reply is judged by its
// thread, as everywhere.
func (r *bookmarksRepo) List(viewer *entities.Viewer, limit uint64, offset uint64) ([]*entities.AnahitaNode, error) {
	if viewer == nil || viewer.ID <= 0 {
		return []*entities.AnahitaNode{}, nil
	}
	if limit == 0 || limit > constants.MaxPageSize {
		limit = constants.DefaultPageSize
	}

	query := savedBy(GetBaseSelectQuery(), viewer.ID).
		Where("node.deleted_at IS NULL").
		Where(sq.Eq{"node.enabled": true})

	query = SetMediumAccessQuery(query, viewer)

	query = query.OrderBy("saved.created_at DESC", "saved.id DESC").
		Limit(limit).
		Offset(offset)

	return FetchNodes(r.db, query)
}

func (r *bookmarksRepo) Count(viewer *entities.Viewer) (uint64, error) {
	if viewer == nil || viewer.ID <= 0 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	query := savedBy(
		sq.Select("COUNT(*)").
			From("nodes AS node").
			LeftJoin("nodes AS owner ON owner.id = node.owner_id"),
		viewer.ID,
	).
		Where("node.deleted_at IS NULL").
		Where(sq.Eq{"node.enabled": true})

	query = SetMediumAccessQuery(query, viewer)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var total uint64
	err = r.db.QueryRowContext(ctx, sqlStr, args...).Scan(&total)
	return total, err
}
