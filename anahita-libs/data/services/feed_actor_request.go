package services

import (
	"anahita-libs/constants"
)

type FeedActorRequest struct {
	Browse *FeedActorBrowseRequest
	Count  *FeedActorCountRequest
}

func NewFeedActorRequest() *FeedActorRequest {
	return &FeedActorRequest{
		Browse: &FeedActorBrowseRequest{
			MediumEntityTypes: []string{},
			IncludeLiked:      false,
			IncludeReposts:    true,
			ActorID:           0,
			Offset:            0,
			Limit:             constants.RequestLimitDefault,
		},
		Count: &FeedActorCountRequest{
			MediumEntityTypes: []string{},
			IncludeLiked:      false,
			IncludeReposts:    true,
			ActorID:           0,
		},
	}
}

type FeedActorBrowseRequest struct {
	IncludeLiked      bool
	IncludeReposts    bool
	MediumEntityTypes []string
	ActorID           int64
	Offset            uint64
	Limit             uint64
}

type FeedActorCountRequest struct {
	IncludeLiked      bool
	IncludeReposts    bool
	MediumEntityTypes []string
	ActorID           int64
}
