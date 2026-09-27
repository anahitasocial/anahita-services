package handlers

import (
	"net/http"
	"strings"

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

// isLastSuperAdmin reports whether the graph refused a lifecycle transition
// because it would leave the installation with nobody able to administer it.
//
// The check itself lives in graph-grpc-service, next to the lifecycle methods
// it guards, because this floor was previously written out four times here —
// Delete, Archive, SetEnabled and Purge — and a check every caller has to
// remember is one a new caller will not. What stays on this side is the
// translation to an HTTP answer.
//
// Matched on the sentinel's message, which is all gRPC carries across.
func isLastSuperAdmin(err error) bool {
	return err != nil && strings.Contains(err.Error(), "last_super_administrator")
}

// refuseLastSuperAdmin is the 409 that names the way out.
//
// 409 rather than 403: the request is well formed and the caller is permitted;
// it conflicts with the state of the site. The body says how to proceed,
// because a refusal that does not is a support ticket.
func refuseLastSuperAdmin(c echo.Context, action string) error {
	return c.JSON(http.StatusConflict, map[string]string{
		"error": "last_super_administrator",
		"message": "This is the only super administrator. Promote someone " +
			"else, or change this account to administrator, and then " + action + " it.",
	})
}

// refuseNotFeaturable is the 409 for featuring a profile that is disabled,
// archived or deleted.
//
// 409 for the reason refuseLastSuperAdmin gives: the caller is permitted and
// the request is well formed; it conflicts with the profile's state. The list
// would drop the profile anyway, and saying so beats a 200 for a feature that
// never appears.
func refuseNotFeaturable(c echo.Context) error {
	return c.JSON(http.StatusConflict, map[string]string{
		"error": "not_featurable",
		"message": "A disabled, archived or deleted profile cannot be " +
			"featured. Enable it first.",
	})
}
