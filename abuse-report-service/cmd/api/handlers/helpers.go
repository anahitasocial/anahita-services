package handlers

import (
	"strings"

	"abusereport/constants"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"

	"github.com/labstack/echo/v4"
)

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

// language is the language to answer in: ?lang= when given, otherwise the
// first one the browser asks for, otherwise the default.
func language(c echo.Context) string {
	if lang := strings.TrimSpace(c.QueryParam("lang")); lang != "" {
		return lang
	}

	header := c.Request().Header.Get("Accept-Language")
	first, _, _ := strings.Cut(header, ",")
	first, _, _ = strings.Cut(first, ";")
	if first = strings.TrimSpace(first); first != "" && first != "*" {
		return first
	}

	return constants.DefaultLanguage
}

// responsibleFor is who answers for a node: the actor itself for a person or
// a group, the author for a post or a comment, and nobody for a hashtag or a
// place.
func responsibleFor(node *entities.AnahitaNode) int64 {
	switch {
	case node.IsActor():
		return node.ID
	case node.CreatedBy != 0:
		return node.CreatedBy
	default:
		return 0
	}
}

// readNodes reads the given nodes, with no access check, and signs their
// image links. For administrators only: it is how a case shows what was
// reported, who reported it and who decided it, whatever their audience.
func (env *Env) readNodes(ids []int64) map[int64]*entities.AnahitaNode {
	nodes := map[int64]*entities.AnahitaNode{}

	unique := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nodes
	}

	list, err := env.Srv.GraphNodeGRPC.BatchRead(&services.GraphNodeBatchReadRequest{IDs: unique})
	if err != nil {
		// A case is still worth showing without its faces.
		return nodes
	}

	var avatars []entities.AnahitaNodeAvatarInterface
	for _, node := range list {
		if node == nil {
			continue
		}
		nodes[node.ID] = node
		if node.Filename != "" && node.IsActor() {
			avatars = append(avatars, node)
		}
	}
	if len(avatars) > 0 && env.Srv.FilesGRPC != nil {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, avatars)
	}

	return nodes
}
