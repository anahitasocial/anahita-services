package main

import (
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

	// subscriptions
	notifications.POST("/subscriptions", r.handler.Subscription.Add)
	notifications.DELETE("/subscriptions/:target_id", r.handler.Subscription.Delete)
}
