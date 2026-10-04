package repositories

import (
	"context"
	"database/sql"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	sq "github.com/Masterminds/squirrel"

	"anahita-libs/constants"
)

type RepostsRepoInterface interface {
	Browse(req *graphpb.RepostBrowseRequest) ([]int64, error)
	BrowseAuthors(req *graphpb.RepostBrowseAuthorsRequest) ([]int64, error)
	Count(req *graphpb.RepostCountRequest) (uint64, error)
	ReadIdByAuthorAndParent(parentID int64, authorID int64) (int64, error)
	Add(item *entities.AnahitaNode) (int64, error)
	IsRepostedMany(authorID int64, nodeIDs []int64) (map[int64]bool, error)
	UpdateRepostCount(parentID int64) error
}

func NewRepostsRepo(db *sql.DB) RepostsRepoInterface {
	return &RepostsRepo{
		db:         db,
		entityType: constants.NodeBaseRepostType,
		dbTimeout:  constants.DBTimeout,
	}
}

type RepostsRepo struct {
	db         *sql.DB
	entityType string
	dbTimeout  time.Duration
}

// Browse
func (repo *RepostsRepo) Browse(req *graphpb.RepostBrowseRequest) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DBTimeout)
	defer cancel()

	query := sq.Select("id").
		From("nodes").
		Where(sq.Eq{"owner_id": req.OwnerId}).
		Where(sq.Eq{"type": repo.entityType})

	// Sort & paginate
	query = query.OrderBy("created_at DESC, id DESC").
		Limit(req.Limit).Offset(req.Offset)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	// log.Println("Reposts Browse SQL:", sqlStr, args)

	rows, err := repo.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// BrowseAuthors
func (repo *RepostsRepo) BrowseAuthors(req *graphpb.RepostBrowseAuthorsRequest) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DBTimeout)
	defer cancel()

	query := sq.Select("node.created_by").
		From("nodes").
		Where(sq.Eq{"parent_id": req.ParentId}).
		Where(sq.Eq{"entity_type": repo.entityType})

	// Sort & paginate
	query = query.OrderBy("node.created_at DESC, node.id DESC").
		Limit(req.Limit).Offset(req.Offset)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	// log.Println("Leaders GetList SQL:", sqlStr, args)

	rows, err := repo.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// ReadIdByAuthorAndParent
func (repo *RepostsRepo) ReadIdByAuthorAndParent(parentID int64, authorID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := sq.Select("id").
		From("nodes").
		Where(sq.Eq{
			"parent_id":  parentID,
			"type":       repo.entityType,
			"created_by": authorID,
		}).Limit(1)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var id int64
	if err := repo.db.QueryRowContext(ctx, sqlStr, args...).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil // No repost found
		}
		return 0, err
	}

	return id, nil
}

// Add
func (repo *RepostsRepo) Add(item *entities.AnahitaNode) (int64, error) {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	if err = repo.validateAdd(item); err != nil {
		return 0, err
	}

	query := sq.Insert("nodes").
		Columns(
			"owner_id",
			"owner_type",
			"parent_id",
			"parent_type",
			"type",
			"service",
			"access",
			"created_at",
			"created_by",
		).Values(
		item.OwnerID,
		item.OwnerType,
		item.ParentID,
		item.ParentType,
		item.Type,
		item.Service,
		item.Access,
		time.Now().UTC(),
		item.CreatedBy,
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	result, err := repo.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, err
	}

	// Obtain the last inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Count
func (repo *RepostsRepo) Count(req *graphpb.RepostCountRequest) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DBTimeout)
	defer cancel()

	query := sq.Select("COUNT(*)").
		From("nodes").
		Where(sq.Eq{"owner_id": req.OwnerId}).
		Where(sq.Eq{"type": repo.entityType})

	var count uint64
	if err := query.QueryRowContext(ctx).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// IsRepostedMany checks if the author has reposted any of the given node IDs.
func (repo *RepostsRepo) IsRepostedMany(authorID int64, nodeIDs []int64) (map[int64]bool, error) {
	if len(nodeIDs) == 0 {
		return map[int64]bool{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sb := sq.StatementBuilder.PlaceholderFormat(sq.Question)

	query := sb.
		Select("node.parent_id").
		From("nodes AS node").
		Where(sq.Eq{
			"node.created_by": authorID,
			"node.parent_id":  nodeIDs,
			"node.type":       repo.entityType,
		})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Only store IDs that are actually reposted
	results := make(map[int64]bool)

	for rows.Next() {
		var parentID int64
		if err := rows.Scan(&parentID); err != nil {
			return nil, err
		}
		results[parentID] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// UpdateRepostCount updates the repost counter for a given parent node.
func (repo *RepostsRepo) UpdateRepostCount(parentID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	// First, count the reposts
	countQuery := sq.Select("COUNT(*)").
		From("nodes").
		Where(sq.Eq{"parent_id": parentID, "type": repo.entityType})

	countSql, countArgs, err := countQuery.ToSql()
	if err != nil {
		return err
	}

	var count int64
	err = repo.db.QueryRowContext(ctx, countSql, countArgs...).Scan(&count)
	if err != nil {
		return err
	}

	// Then, update the repost counter
	updateQuery := sq.Update("nodes").
		Set("repost_count", count).
		Where(sq.Eq{"id": parentID})

	sqlStr, args, err := updateQuery.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return err
	}

	return nil
}
