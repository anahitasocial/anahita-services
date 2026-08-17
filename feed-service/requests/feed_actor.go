package requests

import "anahita-libs/constants"

type FeedActorRequest struct {
	Browse *FeedActorBrowseRequest
}

func NewFeedActorRequest() *FeedActorRequest {
	return &FeedActorRequest{
		Browse: &FeedActorBrowseRequest{
			IncludeReposts: false,
			Offset:         0,
			Limit:          constants.RequestLimitDefault,
		},
	}
}

type FeedActorBrowseRequest struct {
	ActorID        int64  `param:"actor_id" validate:"required,number"`
	IncludeReposts bool   `query:"include_reposts"`
	Offset         uint64 `query:"start"`
	Limit          uint64 `query:"limit"`
}
