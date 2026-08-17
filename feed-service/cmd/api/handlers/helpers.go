package handlers

import (
	"anahita-libs/data/entities"
	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"

	"github.com/labstack/echo/v4"
)

// getViewer returns the Viewer the OAuth middleware built from the
// access token claims. Returns a non-nil unauthenticated Viewer when no
// token was present, so callers never need a nil check.
func getViewer(c echo.Context) *an_entities.Viewer {
	return an_mw.GetViewer(c)
}

// batchPresignPaths presigns the avatar and cover paths of the given actors
func batchPresignPaths(env *Env, items []*entities.AnahitaNode) {
	var authorAvatarNodes []entities.AnahitaNodeAvatarInterface
	var ownerAvatarNodes []entities.AnahitaNodeAvatarInterface
	var parentPortraitNodes []entities.AnahitaNodePortraitInterface
	var parentCoverNodes []entities.AnahitaNodeCoverInterface
	var parentOwnerAvatarNodes []entities.AnahitaNodeAvatarInterface
	var parentAuthorAvatarNodes []entities.AnahitaNodeAvatarInterface
	var coverNodes []entities.AnahitaNodeCoverInterface
	var portraitNodes []entities.AnahitaNodePortraitInterface

	for _, item := range items {
		if item.Author != nil {
			authorAvatarNodes = append(authorAvatarNodes, item.Author)
		}
		if item.Owner != nil {
			ownerAvatarNodes = append(ownerAvatarNodes, item.Owner)
		}
		if item.Parent != nil {
			if item.Parent.Filename != "" {
				parentPortraitNodes = append(parentPortraitNodes, item.Parent)
			}
			if item.Parent.CoverFilename != "" {
				parentCoverNodes = append(parentCoverNodes, item.Parent)
			}
			if item.Parent.Owner != nil {
				parentOwnerAvatarNodes = append(parentOwnerAvatarNodes, item.Parent.Owner)
			}
			if item.Parent.Author != nil {
				parentAuthorAvatarNodes = append(parentAuthorAvatarNodes, item.Parent.Author)
			}
		}
		if item.Filename != "" {
			portraitNodes = append(portraitNodes, item)
		}
		if item.CoverFilename != "" {
			coverNodes = append(coverNodes, item)
		}
	}

	if len(authorAvatarNodes) > 0 {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, authorAvatarNodes)
	}
	if len(ownerAvatarNodes) > 0 {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, ownerAvatarNodes)
	}
	if len(portraitNodes) > 0 {
		services.BatchPresignPortrait(env.Srv.FilesGRPC, portraitNodes)
	}
	if len(coverNodes) > 0 {
		services.BatchPresignCover(env.Srv.FilesGRPC, coverNodes)
	}
	if len(parentPortraitNodes) > 0 {
		services.BatchPresignPortrait(env.Srv.FilesGRPC, parentPortraitNodes)
	}
	if len(parentCoverNodes) > 0 {
		services.BatchPresignCover(env.Srv.FilesGRPC, parentCoverNodes)
	}
	if len(parentOwnerAvatarNodes) > 0 {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, parentOwnerAvatarNodes)
	}
	if len(parentAuthorAvatarNodes) > 0 {
		services.BatchPresignAvatar(env.Srv.FilesGRPC, parentAuthorAvatarNodes)
	}
}
