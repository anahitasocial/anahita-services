package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"

	"github.com/labstack/echo/v4"
)

// getViewer returns the Viewer the OAuth middleware built from the
// access token claims. Returns a non-nil unauthenticated Viewer when no
// token was present, so callers never need a nil check.
func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

// batchPresignPaths presigns the avatar and cover paths of the given actors
func batchPresignPaths(env *Env, items []*entities.AnahitaNode) {
	authorAvatarNodes := make([]entities.AnahitaNodeAvatarInterface, len(items))
	for i, item := range items {
		authorAvatarNodes[i] = item.Author
	}

	services.BatchPresignAvatar(env.Srv.FilesGRPC, authorAvatarNodes)
}

func extractNodeIDs(nodes []*entities.AnahitaNode) []int64 {
	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}
