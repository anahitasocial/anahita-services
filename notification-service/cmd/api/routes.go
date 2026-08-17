package main

import (
	libs_constants "anahita-libs/constants"
	"anahita-libs/middleware"
	"notification/cmd/api/handlers"

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

func (r *Routes) Init(e *echo.Echo) {
	// notifications — every route requires a valid OAuth access token.
	notifications := e.Group("notifications")
	notifications.Use(r.authorize)
	notifications.GET("/", r.handler.Notification.Browse)
	notifications.GET("/unreadcount", r.handler.Notification.UnreadCount)
	notifications.PATCH("/:id", r.handler.Notification.Edit)
	notifications.DELETE("/:id", r.handler.Notification.Delete)

	// subscriptions — gated at the route rather than in a permission
	// method: there is no per-row ownership question here, the caller is
	// always acting on their own subscription, so a coarse scope check
	// is the whole of the authorization.
	subscriptionWrite := middleware.RequireScope(libs_constants.OAuthScopeNotificationsWrite)
	notifications.POST("/subscriptions", r.handler.Subscription.Add, subscriptionWrite)
	notifications.DELETE("/subscriptions/:target_id", r.handler.Subscription.Delete, subscriptionWrite)
}
