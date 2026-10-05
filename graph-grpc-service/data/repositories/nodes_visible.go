package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// FilterVisible returns which of the given nodes this viewer may see, of any
// kind: people and groups, posts, comments, hashtags and places.
//
// It answers one question for callers that hold ids and need to know whether
// to act on them: may this person see this? Reporting something, being shown
// a suggestion, anything that would otherwise confirm that a hidden node
// exists. A node that is missing, deleted, disabled, hidden by its audience,
// or on either side of a block, is simply not in the answer, and the caller
// cannot tell which of those it was.
//
// Each kind is judged by the gate its own listings use, so this cannot drift
// from them:
//
//	actors            actorAccessCondition
//	posts             postAccessCondition (a reply by its thread's root)
//	comments          commentAccessCondition (the post they sit under)
//	hashtags, places  public
//
// Administrators see everything, as they do everywhere else.
//
// The order of the answer is not the order of the question.
func (r *nodesRepo) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}
	if viewer == nil {
		viewer = entities.NewViewer()
	}

	query := visibleQuery(ids, viewer)

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	visible := make([]int64, 0, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		visible = append(visible, id)
	}

	return visible, rows.Err()
}

func visibleQuery(ids []int64, viewer *entities.Viewer) sq.SelectBuilder {
	query := sq.Select("node.id").
		From("nodes AS node").
		LeftJoin("nodes AS owner ON node.owner_id = owner.id").
		Where(sq.Eq{"node.id": ids})

	// Deleted and disabled, on the node and on whoever it belongs to. A
	// no-op for administrators.
	query = SetLifecycleQuery(query, viewer)

	if viewer.IsAdmin() {
		return query
	}

	isActor := sq.Like{"node.type": constants.NodeActorPrefixEntityType + "%"}
	isMedium := sq.Like{"node.type": constants.NodeMediumPrefixEntityType + "%"}
	isComment := sq.Eq{"node.type": constants.NodeBaseCommentEntityType}

	return query.Where(sq.Or{
		sq.And{isActor, actorAccessCondition(viewer)},
		sq.And{isMedium, postAccessCondition(viewer)},
		sq.And{isComment, commentAccessCondition(viewer)},
		// Everything else is a tag, a hashtag or a place, and those have no
		// audience: anybody may see one.
		sq.And{sq.NotLike{"node.type": constants.NodeActorPrefixEntityType + "%"},
			sq.NotLike{"node.type": constants.NodeMediumPrefixEntityType + "%"},
			sq.NotEq{"node.type": constants.NodeBaseCommentEntityType}},
	})
}
