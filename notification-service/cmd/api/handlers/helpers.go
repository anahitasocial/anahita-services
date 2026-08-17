package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"notification/data/entities"
	"strings"

	"github.com/labstack/echo/v4"
)

// getViewer returns the Viewer the OAuth middleware built from the
// access token claims. Returns a non-nil unauthenticated Viewer when no
// token was present, so callers never need a nil check.
func getViewer(c echo.Context) *an_entities.Viewer {
	return an_mw.GetViewer(c)
}

func getEntityNameFromType(entityType string) string {
	parts := strings.Split(entityType, ".")
	if len(parts) > 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// batchPresignPaths presigns the avatar and cover paths of the given actors
func batchPresignPaths(env *Env, items []*entities.Notification) {
	var subjectAvatarNodes []an_entities.AnahitaNodeAvatarInterface
	var objectPortraitNodes []an_entities.AnahitaNodePortraitInterface
	for _, item := range items {
		if item.Subject != nil {
			subjectAvatarNodes = append(subjectAvatarNodes, item.Subject)
		}
		if item.Object != nil {
			objectPortraitNodes = append(objectPortraitNodes, item.Object)
		}
	}

	services.BatchPresignAvatar(env.Srv.FilesGRPC, subjectAvatarNodes)
	services.BatchPresignPortrait(env.Srv.FilesGRPC, objectPortraitNodes)
}

// collects Subject, Object, and Target node IDs from the given notifications
func collectRelationNodeIDs(items []*entities.Notification) []int64 {
	ids := make([]int64, 0)

	for _, item := range items {
		if item.Subject != nil {
			ids = append(ids, item.Subject.ID)
		}
		if item.Object != nil {
			ids = append(ids, item.Object.ID)
		}
		if item.Target != nil {
			ids = append(ids, item.Target.ID)
		}
	}

	return ids
}

// indexNodesByID indexes the given nodes by their ID
func indexNodesByID(nodes []*an_entities.AnahitaNode) map[int64]*an_entities.AnahitaNode {
	nodeMap := make(map[int64]*an_entities.AnahitaNode)
	for _, node := range nodes {
		nodeMap[node.ID] = node
	}
	return nodeMap
}
