package handlers

import (
	"anahita-libs/data/entities"
	an_entities "anahita-libs/data/entities"
	an_services "anahita-libs/data/services"
	an_mw "anahita-libs/middleware"

	"github.com/labstack/echo/v4"
)

// getViewer returns the Viewer the OAuth middleware built from the
// access token claims. Returns a non-nil unauthenticated Viewer when no
// token was present, so callers never need a nil check.
func getViewer(c echo.Context) *an_entities.Viewer {
	return an_mw.GetViewer(c)
}

// onAfterFetched presigns the avatar of the actor
func onAfterActorFetched(env *Env, actors []*entities.AnahitaNode) {
	// Convert to interface slices for batch operations
	var avatarNodes []entities.AnahitaNodeAvatarInterface
	var coverNodes []entities.AnahitaNodeCoverInterface
	for _, actor := range actors {
		avatarNodes = append(avatarNodes, actor)
		coverNodes = append(coverNodes, actor)
	}

	an_services.BatchPresignAvatar(env.Srv.FilesGRPC, avatarNodes)
	an_services.BatchPresignCover(env.Srv.FilesGRPC, coverNodes)
}

// extractNodeIDs extracts IDs from a list of AnahitaNode
func extractNodeIDs(nodes []*entities.AnahitaNode) []int64 {
	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}
