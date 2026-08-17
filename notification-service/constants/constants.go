package constants

import "time"

const (
	DBTimeout = 3 * time.Second

	// Entity types
	NodeBaseNotificationType = "node.base.notification-service.notification.v1"
	NotificationService      = "notification-service"
)
