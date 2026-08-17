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
