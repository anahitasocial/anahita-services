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

func extractIdsFromActors(actors []*entities.AnahitaNode) []int64 {
	ids := make([]int64, 0, len(actors))
	for _, actor := range actors {
		ids = append(ids, actor.ID)
	}
	return ids
}

func mapActorNodesById(actors []*entities.AnahitaNode) map[int64]*entities.AnahitaNode {
	ids := make(map[int64]*entities.AnahitaNode, len(actors))
	for _, actor := range actors {
		ids[actor.ID] = actor
	}
	return ids
}

// onAfterFetched presigns the avatar of the actor
func batchPresignPaths(env *Env, actors []*entities.AnahitaNode) {
	// Convert to interface slices for batch operations
	var avatarNodes []entities.AnahitaNodeAvatarInterface
	var coverNodes []entities.AnahitaNodeCoverInterface
	for _, actor := range actors {
		avatarNodes = append(avatarNodes, actor)
		coverNodes = append(coverNodes, actor)
	}

	services.BatchPresignAvatar(env.Srv.FilesGRPC, avatarNodes)
	services.BatchPresignCover(env.Srv.FilesGRPC, coverNodes)
}
