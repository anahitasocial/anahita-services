package repositories

import (
	"database/sql"
)

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Notifications: NewNotification(db),
	}
}

type Repositories struct {
	Notifications NotificationInterface
}
