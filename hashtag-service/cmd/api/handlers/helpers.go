package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"strings"

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
		if item.Filename != "" && item.IsActor() {
			avatarNodes = append(avatarNodes, item)
		}
		if item.Filename != "" && item.IsMedium() {
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

// CleanStringArray removes empty/whitespace strings, trims values,
// deduplicates, and returns a normalized slice.
func CleanStringArray(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))

	for _, v := range input {
		// Trim whitespace
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}

		// Normalize (optional: lowercase)
		normalized := trimmed

		// Deduplicate
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	return out
}
