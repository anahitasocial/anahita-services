package permissions

import (
	an_entities "anahita-libs/data/entities"
	"errors"
	"notification/data/entities"
)

const (
	ErrInsufficientScope            = "insufficient scope"
	ErrNotificationBrowseNotAllowed = "you are not allowed to browse notifications"
	ErrNotificationEditNotAllowed   = "you are not allowed to edit this notification"
	ErrNotificationDeleteNotAllowed = "you are not allowed to delete this notification"
	ErrNotificationAdminNotAllowed  = "you are not allowed to administrate this notification"
)

type notificationsPermissions struct{}

func newNotificationsPermissions() *notificationsPermissions {
	return &notificationsPermissions{}
}

func (p *notificationsPermissions) CanBrowse(viewer *an_entities.Viewer) (bool, error) {
	if !viewer.HasScope("read:notifications") {
		return false, errors.New(ErrInsufficientScope)
	}

	if !viewer.IsGuest() {
		return true, nil
	}

	return false, errors.New(ErrNotificationBrowseNotAllowed)
}

func (p *notificationsPermissions) CanEdit(viewer *an_entities.Viewer, notification *entities.Notification) (bool, error) {
	if !viewer.HasScope("write:notifications") {
		return false, errors.New(ErrInsufficientScope)
	}

	if notification.RecipientID == viewer.ID {
		return true, nil
	}

	return false, errors.New(ErrNotificationEditNotAllowed)
}

func (p *notificationsPermissions) CanDelete(viewer *an_entities.Viewer, notification *entities.Notification) (bool, error) {
	if !viewer.HasScope("write:notifications") {
		return false, errors.New(ErrInsufficientScope)
	}

	if notification.RecipientID == viewer.ID {
		return true, nil
	}

	return false, errors.New(ErrNotificationDeleteNotAllowed)
}

func (p *notificationsPermissions) CanAdministrate(viewer *an_entities.Viewer, notification *entities.Notification) (bool, error) {
	if !viewer.HasScope("admin:actors") {
		return false, errors.New(ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	if notification.RecipientID == viewer.ID {
		return true, nil
	}

	return false, errors.New(ErrNotificationAdminNotAllowed)
}
