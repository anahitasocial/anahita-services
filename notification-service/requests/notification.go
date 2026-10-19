package requests

import "anahita-libs/constants"

type NotificationRequest struct {
	Browse *NotificationBrowseRequest
	Edit   *NotificationEditRequest
	Delete *NotificationDeleteRequest
	// Turning a kind of thing on or off.
	EditSettings *NotificationSettingsEditRequest
}

func NewNotificationRequest() *NotificationRequest {
	return &NotificationRequest{
		Browse: &NotificationBrowseRequest{
			Offset: 0,
			Limit:  constants.RequestLimitDefault,
		},
		Edit:         &NotificationEditRequest{},
		Delete:       &NotificationDeleteRequest{},
		EditSettings: &NotificationSettingsEditRequest{},
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

type NotificationSettingsEditRequest struct {
	// The kind of thing: likes, reposts, replies, mentions, follows, posts,
	// events or invitations.
	Topic string `json:"topic" form:"topic" validate:"required,oneof=likes reposts replies mentions follows posts events invitations"`
	// Whether to be told about it.
	On bool `json:"on" form:"on"`
}
