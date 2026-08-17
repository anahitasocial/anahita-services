package repositories

import (
	"anahita-libs/data/entities"
	"fmt"
	"graph/requests"
	"graph/responses"
	"strings"

	sq "github.com/Masterminds/squirrel"
)

// Read retrieves a single node based on the provided read request and viewer context.
func (r *nodesRepo) Read(id int64) (*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.id": id})
	return FetchNode(r.db, query)
}

// ReadByAlias retrieves a single node based on the provided alias and viewer context.
func (r *nodesRepo) ReadByAlias(req *requests.NodeReadByAliasRequest) (*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.alias": req.Alias}).Where(sq.Eq{"node.type": req.NodeType})
	return FetchNode(r.db, query)
}

// BatchRead retrieves multiple nodes based on the provided read request and viewer context.
func (r *nodesRepo) BatchRead(req *requests.NodeBatchReadRequest) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.id": req.IDs})

	// Create ORDER BY clause to maintain the same order as the input IDs
	idStrings := strings.Fields(strings.Trim(fmt.Sprint(req.IDs), "[]"))
	fieldClause := "FIELD(node.id, " + strings.Join(idStrings, ", ") + ")"
	query = query.OrderBy(fieldClause)

	return FetchNodes(r.db, query)
}

// BatchReadByAliases retrieves multiple nodes based on the provided aliases and viewer context.
func (r *nodesRepo) BatchReadByAliases(req *requests.NodeBatchReadByAliasesRequest) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery().Where(sq.Eq{"node.alias": req.Aliases}).Where(sq.Eq{"node.type": req.NodeType})

	// Create ORDER BY clause to maintain the same order as the input aliases
	aliasStrings := make([]string, len(req.Aliases))
	for i, alias := range req.Aliases {
		aliasStrings[i] = "'" + alias + "'"
	}
	fieldClause := "FIELD(node.alias, " + strings.Join(aliasStrings, ", ") + ")"
	query = query.OrderBy(fieldClause)

	return FetchNodes(r.db, query)
}

// ReadConnectionStats retrieves connection statistics for a node based on the provided request and viewer context.
func (r *nodesRepo) ReadConnectionStats(req *requests.NodeReadConnectionStatsRequest) (*responses.NodeReadConnectionStatsResponse, error) {
	// Implementation goes here
	return nil, nil
}
