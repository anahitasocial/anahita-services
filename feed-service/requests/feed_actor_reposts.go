package requests

type FeedActorRepostsRequest struct {
	Browse *FeedActorRepostsBrowseRequest
}

type FeedActorRepostsBrowseRequest struct {
	ActorID uint64 `param:"actor_id" validate:"required,number"`
	Offset  uint64 `query:"start"`
	Limit   uint64 `query:"limit"`
}
