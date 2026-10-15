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

func batchPresignPaths(env *Env, items []*entities.AnahitaNode) error {
	var avatarNodes []entities.AnahitaNodeAvatarInterface
	var coverNodes []entities.AnahitaNodeCoverInterface
	var portraitNodes []entities.AnahitaNodePortraitInterface
	var ownerAvatarNodes []entities.AnahitaNodeAvatarInterface
	var authorAvatarNodes []entities.AnahitaNodeAvatarInterface
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.Filename != "" {
			portraitNodes = append(portraitNodes, item)
		}
		if item.CoverFilename != "" {
			coverNodes = append(coverNodes, item)
		}
		if item.Owner != nil {
			ownerAvatarNodes = append(ownerAvatarNodes, item.Owner)
		}
		if item.Author != nil {
			authorAvatarNodes = append(authorAvatarNodes, item.Author)
		}
	}

	// Every picture of a photo post among them: a photo reply in a
	// thread, or the photo post a thread answers.
	services.AttachPhotoFiles(env.Srv.MediaGRPC, items)
	services.BatchPresignPhotoFiles(env.Srv.FilesGRPC, items)

	if len(avatarNodes) > 0 {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, avatarNodes)
	}
	if len(ownerAvatarNodes) > 0 {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, ownerAvatarNodes)
	}
	if len(authorAvatarNodes) > 0 {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, authorAvatarNodes)
	}
	if len(coverNodes) > 0 {
		services.BatchPresignCover(env.Srv.FilesGRPC, coverNodes)
	}
	if len(portraitNodes) > 0 {
		services.BatchPresignPortrait(env.Srv.FilesGRPC, portraitNodes)
	}

	return nil
}
