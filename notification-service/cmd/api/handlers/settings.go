package handlers

import (
	"log"
	"net/http"

	"notification/constants"
	"notification/requests"
	"notification/responses"

	"github.com/labstack/echo/v4"
)

// ReadSettings answers what the viewer is told about: every kind of thing,
// on or off (roadmap row 97, part 5).
func (h *notificationHandler) ReadSettings(c echo.Context) error {
	viewer := getViewer(c)

	if ok, err := h.Perms.Notifications.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	return h.sendSettings(c, viewer.ID)
}

// EditSettings turns one kind of thing on or off for the viewer, and
// answers with them all.
func (h *notificationHandler) EditSettings(c echo.Context) error {
	var (
		req    = requests.NewNotificationRequest().EditSettings
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Notifications.CanEditAll(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil || !constants.IsTopic(req.Topic) {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Repos.Notifications.SetTopic(viewer.ID, req.Topic, req.On); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return h.sendSettings(c, viewer.ID)
}

func (h *notificationHandler) sendSettings(c echo.Context, recipientID int64) error {
	off, err := h.Repos.Notifications.OffTopics(recipientID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	isOff := map[string]bool{}
	for _, topic := range off {
		isOff[topic] = true
	}

	settings := responses.Settings{Topics: make([]responses.TopicChoice, 0, len(constants.Topics))}
	for _, topic := range constants.Topics {
		settings.Topics = append(settings.Topics, responses.TopicChoice{Topic: topic, On: !isOff[topic]})
	}
	return c.JSON(http.StatusOK, settings)
}
