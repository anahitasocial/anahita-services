package handlers

import (
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

	count, err := h.Repos.Notifications.UnreadCount(viewer)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, responses.UnreadCount{Count: count})
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
