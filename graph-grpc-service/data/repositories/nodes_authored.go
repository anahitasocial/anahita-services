package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// The most read at once.
const authoredPageMax = 200

// Authored returns what one person wrote, lowest id first, starting after
// an id: their posts of every kind, their replies, their reposts and the
// custom feeds they made, wherever each is.
//
// NO ACCESS RULE IS APPLIED, AND THAT IS THE POINT OF IT: it is for giving
// a person their own copy of what they wrote, some of which only they may
// see. It is never to be used to show one person what another wrote.
//
// What they removed is left out: it is gone for them too.
func (r *nodesRepo) Authored(personID, afterID int64, limit uint64) ([]*entities.AnahitaNode, error) {
	if personID <= 0 {
		return nil, nil
	}
	if limit == 0 || limit > authoredPageMax {
		limit = authoredPageMax
	}

	query := GetBaseSelectQuery().
		Where(sq.Eq{"node.created_by": personID}).
		Where(sq.Gt{"node.id": afterID}).
		Where(sq.Or{
			sq.Like{"node.type": constants.NodeMediumPrefixEntityType + "%"},
			sq.Like{"node.type": "node.feed.%"},
			sq.Eq{"node.type": constants.NodeBaseRepostType},
		}).
		Where(sq.Eq{"node.deleted_at": nil}).
		OrderBy("node.id ASC").
		Limit(limit)

	return FetchNodes(r.db, query)
}
