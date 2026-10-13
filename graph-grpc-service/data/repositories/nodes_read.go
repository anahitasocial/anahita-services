package repositories

import (
	"anahita-libs/data/entities"
	"graph/requests"
	"graph/responses"
	"strings"

	sq "github.com/Masterminds/squirrel"
)

// Read retrieves a single node by id, of ANY type.
//
// Callers that know what family they expect should use ReadOfType instead —
// see the note there for why.
func (r *nodesRepo) Read(id int64) (*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.id": id})
	return FetchNode(r.db, query)
}

// ReadOfType retrieves a single node by id, but only if it belongs to the
// given type family — "node.actor." or "node.medium.".
//
// Node ids are unique across every type in one table, so a bare id lookup
// happily returns a hashtag to a caller that asked for a group. That was
// live: GET /groups/<a hashtag id> returned 200 with a
// node.tag.hashtag-service.hashtag.v1 body, and so did /photos/ and /topics/.
// Every service serves its own id-addressed routes, so the wrong-shaped
// object reached the client of whichever one was asked.
//
// ReadByAlias in this file has always filtered on type. The id path simply
// never grew the same predicate.
//
// A prefix rather than an exact type, because the RPC boundary is per family:
// the actor read serves people, groups and any future actor, and the medium
// read serves photos, articles and topics. Narrowing to the exact entity is
// the calling service's job, since only it knows that /groups/ means a group.
//
// Not found and wrong-type are the same answer on purpose: both mean "no such
// group here", and distinguishing them would let a caller probe which ids
// exist as some other type.
func (r *nodesRepo) ReadOfType(id int64, nodeType string) (*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.id": id})

	// A trailing dot marks a family — "node.actor." — and anything else is a
	// complete type. The constants are written that way
	// (NodeActorPrefixEntityType ends in a dot, NodeActorPersonEntityType does
	// not), so the distinction is visible at the call site rather than encoded
	// in a second argument nobody remembers to pass.
	//
	// Matching a complete type with LIKE would work today but is wrong in
	// principle: "…group.v1%" also matches a hypothetical "…group.v10", and
	// equality says what is meant.
	if strings.HasSuffix(nodeType, ".") {
		query = query.Where(sq.Like{"node.type": nodeType + "%"})
	} else {
		query = query.Where(sq.Eq{"node.type": nodeType})
	}

	return FetchNode(r.db, query)
}

// ReadByAlias retrieves a single node based on the provided alias and viewer context.
func (r *nodesRepo) ReadByAlias(req *requests.NodeReadByAliasRequest) (*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.alias": req.Alias}).Where(sq.Eq{"node.type": req.NodeType})
	return FetchNode(r.db, query)
}

// ReadByUUID retrieves a single node by the name it goes by to other
// servers. Of any type, as Read is: a UUID is only ever learned from this
// installation, which gave it out as a person's or a post's.
//
// No access rule, as with Read. The caller decides who is asking.
func (r *nodesRepo) ReadByUUID(uuid string) (*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.uuid": uuid})
	return FetchNode(r.db, query)
}

// BatchRead retrieves multiple nodes based on the provided read request and viewer context.
func (r *nodesRepo) BatchRead(req *requests.NodeBatchReadRequest) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.id": req.IDs})

	// Order the results the way the caller asked for them. These are int64s
	// rather than strings, so the old formatting round-trip through
	// fmt.Sprint was not injectable — but placeholders make that true by
	// construction rather than by the element type.
	if len(req.IDs) > 0 {
		placeholders := make([]string, len(req.IDs))
		args := make([]interface{}, len(req.IDs))
		for i, id := range req.IDs {
			placeholders[i] = "?"
			args[i] = id
		}
		fieldClause := "FIELD(node.id, " + strings.Join(placeholders, ", ") + ")"
		query = query.OrderByClause(fieldClause, args...)
	}

	return FetchNodes(r.db, query)
}

// BatchReadByAliases retrieves multiple nodes based on the provided aliases and viewer context.
func (r *nodesRepo) BatchReadByAliases(req *requests.NodeBatchReadByAliasesRequest) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.alias": req.Aliases}).Where(sq.Eq{"node.type": req.NodeType})

	// Order the results the way the caller asked for them, via placeholders.
	//
	// This used to build the FIELD() list by wrapping each alias in single
	// quotes and concatenating. An alias containing a quote would have closed
	// the string and continued the statement. No caller can reach it with one
	// today — the only one is hashtag-service, whose aliases come out of
	// HashtagRegexPattern and so can only hold word characters and emoji — but
	// that is a property of one regex in another repository, not of this
	// function, and nothing here would fail if it changed.
	if len(req.Aliases) > 0 {
		placeholders := make([]string, len(req.Aliases))
		args := make([]interface{}, len(req.Aliases))
		for i, alias := range req.Aliases {
			placeholders[i] = "?"
			args[i] = alias
		}
		fieldClause := "FIELD(node.alias, " + strings.Join(placeholders, ", ") + ")"
		query = query.OrderByClause(fieldClause, args...)
	}

	return FetchNodes(r.db, query)
}

// ReadConnectionStats retrieves connection statistics for a node based on the provided request and viewer context.
func (r *nodesRepo) ReadConnectionStats(req *requests.NodeReadConnectionStatsRequest) (*responses.NodeReadConnectionStatsResponse, error) {
	// Implementation goes here
	return nil, nil
}
