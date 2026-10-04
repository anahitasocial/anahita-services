package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"database/sql"
	"graph/requests"
	"time"

	sq "github.com/Masterminds/squirrel"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ActorAdminRepoInterface interface {
	// Browse returns a list of ActorAdmins
	Browse(actorID int64) ([]*graphpb.Node, error)

	// Add adds actor_admin relationship to actor
	Add(actor *graphpb.Node, admin *graphpb.Node, viewer *entities.Viewer) error

	// Delete removes actor_admin relationship from actor
	Delete(actor *graphpb.Node, admin *graphpb.Node) error
}

// NewActorAdmins creates a new ActorAdmins repository
func NewActorAdminRepo(db *sql.DB) ActorAdminRepoInterface {
	return &ActorAdmin{
		db: db,
	}
}

type ActorAdmin struct {
	db *sql.DB
}

// getBaseSelectQuery returns the base select query for the ActorAdmins repository
func (repo *ActorAdmin) getBaseSelectQuery() sq.SelectBuilder {
	var nodeFields = []string{
		"node.id",
		"node.type",
		"node.service",
		"COALESCE(node.name, '') AS name",
		"node.alias",
		"COALESCE(node.filename, '') AS filename",
		"node.created_at",
		"node.updated_at",
	}

	return sq.Select(nodeFields...).From(
		"nodes AS node",
	).Join(
		"edges AS edge ON node.id = edge.node_a_id",
	)
}

// Browse returns a list of ActorAdmins
func (repo *ActorAdmin) Browse(actorId int64) ([]*graphpb.Node, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DBTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery().Where(
		sq.Eq{"edge.node_b_id": actorId},
	).Where(
		sq.Eq{"edge.type": constants.EdgeActorAdminEntityType},
	).Where(
		sq.Eq{"node.enabled": true},
	).OrderBy("node.name ASC")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*graphpb.Node
	for rows.Next() {
		var (
			createdAt time.Time
			updatedAt time.Time
		)

		item := &graphpb.Node{
			Author: &graphpb.Node{},
			Editor: &graphpb.Node{},
		}

		err := rows.Scan(
			&item.Id,
			&item.Type,
			&item.Service,
			&item.Name,
			&item.Alias,
			&item.Filename,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Convert time.Time to *timestamppb.Timestamp
		item.CreatedAt = timestamppb.New(createdAt)
		item.UpdatedAt = timestamppb.New(updatedAt)

		results = append(results, item)
	}

	return results, nil
}

// Add adds actor_admin relationship to actor
func (repo *ActorAdmin) Add(admin *graphpb.Node, actor *graphpb.Node, viewer *entities.Viewer) error {
	edgeRepo := NewEdgesRepo(repo.db)
	edge := entities.NewAnahitaEdge()
	edge.Type = constants.EdgeActorAdminEntityType
	edge.NodeAID = admin.Id
	edge.NodeAType = admin.Type
	edge.NodeBID = actor.Id
	edge.NodeBType = actor.Type
	edge.CreatedBy = viewer.ID
	edge.ModifiedBy = viewer.ID

	edge, err := edgeRepo.Add(edge)
	if err != nil {
		return err
	}

	return nil
}

// Delete removes actor_admin relationship from actor
func (repo *ActorAdmin) Delete(admin *graphpb.Node, actor *graphpb.Node) error {
	edgeRepo := NewEdgesRepo(repo.db)
	req := &requests.EdgeBatchDeleteByFiltersRequest{
		Types:     []string{constants.EdgeActorAdminEntityType},
		NodeAID:   admin.Id,
		NodeAType: admin.Type,
		NodeBID:   actor.Id,
		NodeBType: actor.Type,
	}
	_, err := edgeRepo.BatchDeleteByFilters(req)
	if err != nil {
		return err
	}

	return nil
}
