package requests

import "anahita-libs/constants"

type FeedActorRequest struct {
	Browse *FeedActorBrowseRequest
}

func NewFeedActorRequest() *FeedActorRequest {
	return &FeedActorRequest{
		Browse: &FeedActorBrowseRequest{
			Offset: 0,
			Limit:  constants.RequestLimitDefault,
		},
	}
}

type FeedActorBrowseRequest struct {
	// The profile's id.
	ActorID int64 `param:"actor_id" validate:"required,number"`
	// Which of the profile's lists: posts (the default), replies, reposts,
	// or tagged, the photos and videos a person is in.
	Filter string `query:"filter" validate:"omitempty,oneof=posts replies reposts tagged"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
}
