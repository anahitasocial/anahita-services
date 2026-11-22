package services

import (
	"anahita-libs/constants"
)

type FeedLeadersRequest struct {
	Browse *FeedLeadersBrowseRequest
}

func NewFeedLeadersRequest() *FeedLeadersRequest {
	return &FeedLeadersRequest{
		Browse: &FeedLeadersBrowseRequest{
			IncludeLiked:      false,
			IncludeReposts:    false,
			MediumEntityTypes: []string{},
			LeaderEntityTypes: []string{},
			Offset:            0,
			Limit:             constants.RequestLimitDefault,
		},
	}
}

type FeedLeadersBrowseRequest struct {
	IncludeLiked      bool
	IncludeReposts    bool
	MediumEntityTypes []string
	LeaderEntityTypes []string
	Offset            uint64
	Limit             uint64
}
