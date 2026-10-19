package handlers

import (
	"anahita-libs/cmd/api/permissions"
	an_constants "anahita-libs/constants"
	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"notification/data/entities"
	"strings"

	"github.com/labstack/echo/v4"
)

// getViewer returns the Viewer the OAuth middleware built from the
// access token claims. Returns a non-nil unauthenticated Viewer when no
// token was present, so callers never need a nil check.
func getViewer(c echo.Context) *an_entities.Viewer {
	return an_mw.GetViewer(c)
}

func getEntityNameFromType(entityType string) string {
	parts := strings.Split(entityType, ".")
	if len(parts) > 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// batchPresignPaths presigns the avatar and cover paths of the given actors
func batchPresignPaths(env *Env, items []*entities.Notification) {
	var subjectAvatarNodes []an_entities.AnahitaNodeAvatarInterface
	var objectPortraitNodes []an_entities.AnahitaNodePortraitInterface
	for _, item := range items {
		if item.Subject != nil {
			subjectAvatarNodes = append(subjectAvatarNodes, item.Subject)
		}
		if item.Object != nil {
			objectPortraitNodes = append(objectPortraitNodes, item.Object)
		}
	}

	services.BatchPresignAvatar(env.Srv.FilesGRPC, subjectAvatarNodes)
	services.BatchPresignPortrait(env.Srv.FilesGRPC, objectPortraitNodes)
}

// collects Subject, Object, and Target node IDs from the given notifications
func collectRelationNodeIDs(items []*entities.Notification) []int64 {
	ids := make([]int64, 0)

	for _, item := range items {
		if item.Subject != nil {
			ids = append(ids, item.Subject.ID)
		}
		if item.Object != nil {
			ids = append(ids, item.Object.ID)
		}
		if item.Target != nil {
			ids = append(ids, item.Target.ID)
		}
	}

	return ids
}

// indexNodesByID indexes the given nodes by their ID
func indexNodesByID(nodes []*an_entities.AnahitaNode) map[int64]*an_entities.AnahitaNode {
	nodeMap := make(map[int64]*an_entities.AnahitaNode)
	for _, node := range nodes {
		nodeMap[node.ID] = node
	}
	return nodeMap
}

// unsubscribeUnlessWanted ends a subscription to a profile when nothing
// keeps it any more: following it, or being one of its administrators,
// each of which subscribes (roadmap row 96, part 2). Losing one leaves the
// other's.
func (h *eventHandler) unsubscribeUnlessWanted(subject, target *an_entities.AnahitaNode) error {
	relation, err := h.Srv.SocialgraphGRPC.Read(target.ID, subject.ID)
	if err != nil {
		return err
	}
	if relation != nil && (relation.IsLeading || relation.HasAdmin) {
		return nil
	}
	return h.Srv.SubscriptionsGRPC.Delete(subject, target)
}

// mayRead reports whether a person here may read these nodes, a post and the
// reply that holds what was done, before they are told of them (roadmap row
// 96, part 1): nobody is told of a post they may not see. Asked as a
// registered member, which can only leave somebody out, never show them
// more: a site administrator is told what they would be as a member.
func (h *eventHandler) mayRead(recipientID int64, ids ...int64) bool {
	reader := an_entities.NewViewer()
	reader.ID = recipientID
	reader.PersonType = an_constants.PersonTypeRegistered
	reader.Authenticated = true

	readable, err := permissions.CanReadEach(h.Srv.GraphNodeGRPC, ids, reader)
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id > 0 && !readable[id] {
			return false
		}
	}
	return true
}
