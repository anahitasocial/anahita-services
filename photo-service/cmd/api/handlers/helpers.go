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
	var portraitNodes []entities.AnahitaNodePortraitInterface
	var ownerAvatarNodes []entities.AnahitaNodeAvatarInterface
	var authorAvatarNodes []entities.AnahitaNodeAvatarInterface
	for _, item := range items {
		portraitNodes = append(portraitNodes, item)
		ownerAvatarNodes = append(ownerAvatarNodes, item.Owner)
		authorAvatarNodes = append(authorAvatarNodes, item.Author)
	}

	services.BatchPresignPortrait(env.Srv.FilesGRPC, portraitNodes)
	services.BatchPresignAvatar(env.Srv.FilesGRPC, ownerAvatarNodes)
	services.BatchPresignAvatar(env.Srv.FilesGRPC, authorAvatarNodes)
}
