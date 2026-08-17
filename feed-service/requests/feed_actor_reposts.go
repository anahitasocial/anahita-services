package requests

import "anahita-libs/constants"

type FeedActorRepostsRequest struct {
	Browse *FeedActorRepostsBrowseRequest
}

func NewFeedActorRepostsRequest() *FeedActorRepostsRequest {
	return &FeedActorRepostsRequest{
		Browse: &FeedActorRepostsBrowseRequest{
			Offset: 0,
			Limit:  constants.RequestLimitDefault,
		},
	}
}

type FeedActorRepostsBrowseRequest struct {
	ActorID uint64 `param:"actor_id" validate:"required,number"`
	Offset  uint64 `query:"start"`
	Limit   uint64 `query:"limit"`
}
