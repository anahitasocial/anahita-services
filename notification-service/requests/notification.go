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
	Offset uint64 `query:"start"`
	Limit  uint64 `query:"limit"`
}

type NotificationEditRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}

type NotificationDeleteRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}
