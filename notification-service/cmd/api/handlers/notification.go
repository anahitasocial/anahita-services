package handlers

import (
	"anahita-libs/cmd/api/permissions"
	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_responses "anahita-libs/responses"
	"log"
	"net/http"
	"notification/data/entities"
	"notification/requests"
	"notification/responses"

	"github.com/labstack/echo/v4"
)

type notificationHandler struct {
	*Env
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(env *Env) *notificationHandler {
	return &notificationHandler{
		env,
	}
}

// Browse returns a list of notification
func (h *notificationHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNotificationRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Notifications.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	resp := &an_responses.List[*responses.NotificationItem]{}

	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset
	resp.Pagination.Total, err = h.Repos.Notifications.Count(req, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	notifications, err := h.Repos.Notifications.Browse(req, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Only what the viewer may still read: a post made private since, or
	// one whose audience they have left, is not shown, and the notification
	// goes, so that the count says what the list does (roadmap row 96,
	// part 1).
	notifications, gone := h.keepReadable(notifications, viewer)
	if gone > 0 && resp.Pagination.Total >= uint64(gone) {
		resp.Pagination.Total -= uint64(gone)
	}
	if len(notifications) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	ids := collectRelationNodeIDs(notifications)
	nodesReq := &services.GraphNodeBatchReadRequest{
		IDs:    ids,
		Viewer: viewer,
	}
	nodes, err := h.Srv.GraphNodeGRPC.BatchRead(nodesReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	nodeIndex := indexNodesByID(nodes)

	for _, notification := range notifications {
		if notification.Subject.ID > 0 {
			if node, ok := nodeIndex[notification.Subject.ID]; ok {
				notification.Subject = node
			}
		}
		if notification.Object.ID > 0 {
			if node, ok := nodeIndex[notification.Object.ID]; ok {
				notification.Object = node
			}
		}
		if notification.Target.ID > 0 {
			if node, ok := nodeIndex[notification.Target.ID]; ok {
				notification.Target = node
			}
		}
	}

	batchPresignPaths(h.Env, notifications)

	notificationsResp := &responses.NotificationResponse{}
	resp.Data = notificationsResp.MapNotificationItems(notifications)
	return c.JSON(http.StatusOK, resp)
}

// UnreadCount returns the number of unread notifications
func (h *notificationHandler) UnreadCount(c echo.Context) error {
	viewer := getViewer(c)

	if ok, err := h.Perms.Notifications.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	count, newest, err := h.Repos.Notifications.UnreadCount(viewer)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, responses.UnreadCount{Count: count, NewestAt: newest})
}

// EditAll marks all the viewer's notifications read.
func (h *notificationHandler) EditAll(c echo.Context) error {
	viewer := getViewer(c)

	if ok, err := h.Perms.Notifications.CanEditAll(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	count, err := h.Repos.Notifications.EditAll(viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, responses.MarkedRead{Count: count})
}

// Edit updates a notification
func (h *notificationHandler) Edit(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNotificationRequest().Edit
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	notification, err := h.Repos.Notifications.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Notifications.CanEdit(viewer, notification); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if kept, _ := h.keepReadable([]*entities.Notification{notification}, viewer); len(kept) == 0 {
		return c.NoContent(http.StatusNotFound)
	}

	notification.IsRead = true

	if err := h.Repos.Notifications.Edit(notification); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	notification, err = h.Repos.Notifications.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	ids := collectRelationNodeIDs([]*entities.Notification{notification})
	nodesReq := &services.GraphNodeBatchReadRequest{
		IDs:    ids,
		Viewer: viewer,
	}
	nodes, err := h.Srv.GraphNodeGRPC.BatchRead(nodesReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	nodeIndex := indexNodesByID(nodes)

	if notification.Subject.ID > 0 {
		if node, ok := nodeIndex[notification.Subject.ID]; ok {
			notification.Subject = node
		}
	}
	if notification.Object.ID > 0 {
		if node, ok := nodeIndex[notification.Object.ID]; ok {
			notification.Object = node
		}
	}
	if notification.Target.ID > 0 {
		if node, ok := nodeIndex[notification.Target.ID]; ok {
			notification.Target = node
		}
	}

	services.PresignAvatar(h.Srv.FilesGRPC, notification.Subject)

	notificationsResp := &responses.NotificationResponse{}
	resp := notificationsResp.MapNotificationItem(notification)
	return c.JSON(http.StatusOK, resp)
}

// Delete a notification
func (h *notificationHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNotificationRequest().Delete
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	notification, err := h.Repos.Notifications.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Notifications.CanDelete(viewer, notification); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Repos.Notifications.Delete(notification); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}

// keepReadable answers the notifications whose post, and reply, the viewer
// may read, asked once for all of them, and deletes the others: a
// notification about something its reader may not see is noise, and a leak
// if shown. When the answer cannot be had, none is shown and none goes:
// better an empty list for a moment than a post its reader may not see.
func (h *notificationHandler) keepReadable(items []*entities.Notification, viewer *an_entities.Viewer) ([]*entities.Notification, int) {
	ids := []int64{}
	for _, item := range items {
		ids = append(ids, item.Object.ID, item.CommentID)
	}
	readable, err := permissions.CanReadEach(h.Srv.GraphNodeGRPC, ids, viewer)
	if err != nil {
		log.Println("notifications: asking what the viewer may read:", err)
		return []*entities.Notification{}, 0
	}

	kept := make([]*entities.Notification, 0, len(items))
	removed := 0
	for _, item := range items {
		if (item.Object.ID > 0 && !readable[item.Object.ID]) || (item.CommentID > 0 && !readable[item.CommentID]) {
			gone := entities.NewNotification()
			gone.ID = item.ID
			if err := h.Repos.Notifications.Delete(gone); err != nil {
				log.Println("notifications: deleting one about what its reader may not see:", err)
			}
			removed++
			continue
		}
		kept = append(kept, item)
	}
	return kept, removed
}
