package handlers

import (
	"notification/cmd/api/permissions"
	"notification/config"
	"notification/data/repositories"
	"notification/data/services"
)

type Env struct {
	Config *config.Config
	Repos  *repositories.Repositories
	Srv    *services.Services
	Perms  *permissions.Permissions
}

type Handlers struct {
	Event        *eventHandler
	Notification *notificationHandler
	Subscription *subscriptionHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Event:        NewEventHandler(env),
		Notification: NewNotificationHandler(env),
		Subscription: NewSubscriptionHandler(env),
	}
}
