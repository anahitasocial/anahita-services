package requests

import "anahita-libs/constants"

type NotificationRequest struct {
	Browse *NotificationBrowseRequest
	Edit   *NotificationEditRequest
	Delete *NotificationDeleteRequest
}

func NewNotificationRequest() *NotificationRequest {
	return &NotificationRequest{
		Browse: &NotificationBrowseRequest{
			Offset: 0,
			Limit:  constants.RequestLimitDefault,
		},
		Edit:   &NotificationEditRequest{},
		Delete: &NotificationDeleteRequest{},
	}
}

type NotificationBrowseRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
}

type NotificationEditRequest struct {
	// The notification's id.
	ID int64 `param:"id" validate:"required,numeric"`
}

type NotificationDeleteRequest struct {
	// The notification's id.
	ID int64 `param:"id" validate:"required,numeric"`
}
