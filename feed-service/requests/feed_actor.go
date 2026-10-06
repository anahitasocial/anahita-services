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
	ActorID int64 `param:"actor_id" validate:"required,number"`
	// Which of the profile's lists: posts (the default), replies or
	// reposts.
	Filter string `query:"filter" validate:"omitempty,oneof=posts replies reposts"`
	Offset uint64 `query:"start"`
	Limit  uint64 `query:"limit"`
}
