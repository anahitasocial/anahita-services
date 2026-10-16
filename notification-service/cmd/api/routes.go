package main

import (
	"net/http"

	"notification/cmd/api/handlers"
	"notification/requests"
	"notification/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	"anahita-libs/middleware"
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	authorize echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		authorize: authorize,
	}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/notification-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("notification-service", "Notifications",
		"What the viewer is told of: who followed them, liked, replied to or reposted what they wrote, and the rest; and what they asked to be told about.")
	api := doc.On(e)
	read := libs_constants.OAuthScopeNotificationsRead
	write := libs_constants.OAuthScopeNotificationsWrite

	// notifications — every route requires a valid OAuth access token.
	notifications := api.Group("notifications", r.authorize)
	notifications.GET("/", r.handler.Notification.Browse).
		Summary("The viewer's notifications").
		Describe("The latest first, each with who did what to what, as they are now.").
		Viewer(apidoc.SignedIn).
		Scope(read).
		Takes(requests.NewNotificationRequest().Browse).
		Sends(http.StatusOK, libs_responses.List[*responses.NotificationItem]{})
	notifications.GET("/unread-count", r.handler.Notification.UnreadCount).
		Summary("How many are unread").
		Viewer(apidoc.SignedIn).
		Scope(read).
		Sends(http.StatusOK, responses.UnreadCount{})
	notifications.PATCH("/:id", r.handler.Notification.Edit).
		Summary("Mark one as read").
		Describe("It takes nothing else: the one thing that changes is that it is read. Answers with it.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(requests.NewNotificationRequest().Edit).
		Sends(http.StatusOK, responses.NotificationItem{}).
		Refuses(http.StatusForbidden, "it is somebody else's").
		Refuses(http.StatusNotFound, "there is no such notification")
	notifications.DELETE("/:id", r.handler.Notification.Delete).
		Summary("Delete one").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(requests.NewNotificationRequest().Delete).
		SendsNothing(http.StatusNoContent, "it is gone").
		Refuses(http.StatusForbidden, "it is somebody else's").
		Refuses(http.StatusNotFound, "there is no such notification")

	// subscriptions — gated at the route rather than in a permission
	// method: there is no per-row ownership question here, the caller is
	// always acting on their own subscription, so a coarse scope check
	// is the whole of the authorization.
	subscriptionWrite := middleware.RequireScope(write)
	notifications.POST("/subscriptions", r.handler.Subscription.Add, subscriptionWrite).
		Summary("Be told about a post or a profile").
		Describe("The viewer is notified of what happens to it from now on. Answers with what they are told about.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(requests.NewSubscriptionRequest().Add).
		Sends(http.StatusCreated, libs_responses.NodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not see it").
		Refuses(http.StatusNotFound, "there is no such post or profile")
	notifications.DELETE("/subscriptions/:target_id", r.handler.Subscription.Delete, subscriptionWrite).
		Summary("Stop being told about it").
		Describe("Answers 201 with what they were told about, for now, though nothing is made.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(requests.NewSubscriptionRequest().Delete).
		Sends(http.StatusCreated, libs_responses.NodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not see it").
		Refuses(http.StatusNotFound, "there is no such post or profile, or the viewer was not told about it")

	return doc
}
