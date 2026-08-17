package repositories

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"person/constants"
	"time"

	sq "github.com/Masterminds/squirrel"
)

type MentionRelationRepoInterface interface {
	// Update updates mention relations of a node according to the given mention aliases
	Update(node TaggableNodeInterface, mentionAliases []string, viewer *entities.Viewer) error

	// Create a new mention relation
	Create(node TaggableNodeInterface, mentionID int64, viewer *entities.Viewer) error

	// Delete a mention relation
	Delete(node TaggableNodeInterface, mentionID int64) error
}

// NewMentionRelationRepo creates a new instance of MentionRelationRepo
func NewMentionRelationRepo(db *sql.DB) MentionRelationRepoInterface {
	return &MentionRelationRepo{
		db:        db,
		dbTimeout: an_constants.DBTimeout,
		edgeType:  constants.EdgeTagMentionType,
	}
}

// MentionRelationRepo is the repository for mention relations
type MentionRelationRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
	edgeType  string
}

// Update updates mention relations of a node according to the given mention aliases
func (r *MentionRelationRepo) Update(node TaggableNodeInterface, mentionAliases []string, viewer *entities.Viewer) error {
	// Get mention IDs
	ids, err := r.getMentionIDs(mentionAliases)
	if err != nil {
		return err
	}

	// Get existing mention relations
	currentIDs, err := r.getCurrentMentionIDs(node)
	if err != nil {
		return err
	}

	// Create new relations
	for id := range ids {
		if _, ok := currentIDs[id]; !ok {
			if err := r.Create(node, id, viewer); err != nil {
				return err
			}
		}
	}

	// Delete old relations
	for id := range currentIDs {
		if _, ok := ids[id]; !ok {
			if err := r.Delete(node, id); err != nil {
				return err
			}
		}
	}

	return nil
}

// Create a new mention relation
func (r *MentionRelationRepo) Create(node TaggableNodeInterface, personID int64, viewer *entities.Viewer) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Insert("edges").Columns(
		"type",
		"node_a_id",
		"node_a_type",
		"node_b_id",
		"node_b_type",
		"created_on",
		"created_by",
		"modified_on",
		"modified_by",
	).Values(
		r.edgeType,
		personID,
		an_constants.NodeActorPersonEntityType,
		node.GetID(),
		node.GetType(),
		time.Now(),
		viewer.ID,
		time.Now(),
		viewer.ID,
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return err
	}

	return nil
}

// Delete a mention relation
func (r *MentionRelationRepo) Delete(node TaggableNodeInterface, personID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Delete("edges").
		Where(sq.Eq{
			"type":        r.edgeType,
			"node_a_id":   personID,
			"node_a_type": an_constants.NodeActorPersonEntityType,
			"node_b_id":   node.GetID(),
			"node_b_type": node.GetType(),
		})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return err
	}

	return nil
}

// getMentionIDs returns mention IDs for the given mention aliases
func (r *MentionRelationRepo) getMentionIDs(aliases []string) (map[int64]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Select("id").
		From("nodes").
		Where(sq.Eq{"alias": aliases}).
		Where(sq.Eq{"type": an_constants.NodeActorPersonEntityType})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids[id] = true
	}

	return ids, nil
}

// getCurrentMentionIDs returns mention IDs for the given node
func (r *MentionRelationRepo) getCurrentMentionIDs(node TaggableNodeInterface) (map[int64]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Select("node_a_id").From("edges").Where(
		sq.Eq{"type": r.edgeType},
	).Where(
		sq.Eq{"node_b_id": node.GetID()},
	)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids[id] = true
	}

	return ids, nil
}
