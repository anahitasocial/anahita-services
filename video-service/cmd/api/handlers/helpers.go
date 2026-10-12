package handlers

import (
	"log"

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

// batchPresignPaths signs the pictures of whoever posted each video and
// of the profile it is on. A video's own files are not signed here: they
// are asked for at addresses of this service's. See files.go.
func batchPresignPaths(env *Env, items []*entities.AnahitaNode) {
	var ownerAvatarNodes []entities.AnahitaNodeAvatarInterface
	var authorAvatarNodes []entities.AnahitaNodeAvatarInterface
	for _, item := range items {
		ownerAvatarNodes = append(ownerAvatarNodes, item.Owner)
		authorAvatarNodes = append(authorAvatarNodes, item.Author)
	}

	services.BatchPresignAvatar(env.Srv.FilesGRPC, ownerAvatarNodes)
	services.BatchPresignAvatar(env.Srv.FilesGRPC, authorAvatarNodes)
}

// deleteStored removes a video's files from storage. A file that cannot be
// removed is logged and left: the post is gone either way.
func (h *videoHandler) deleteStored(keys []string) {
	for _, key := range keys {
		req := services.NewFileRequest().Delete
		req.Path = key
		if err := h.Srv.FilesGRPC.Delete(req); err != nil {
			log.Printf("could not remove %s: %v", key, err)
		}
	}
}
