package services

import (
	"anahita-libs/constants"
)

type FeedLeadersRequest struct {
	Browse *FeedLeadersBrowseRequest
	Count  *FeedLeadersCountRequest
}

func NewFeedLeadersRequest() *FeedLeadersRequest {
	return &FeedLeadersRequest{
		Browse: &FeedLeadersBrowseRequest{
			IncludeComments:   false,
			IncludeLiked:      false,
			IncludeReposts:    false,
			MediumEntityTypes: []string{},
			LeaderEntityTypes: []string{},
			Offset:            0,
			Limit:             constants.RequestLimitDefault,
		},
		Count: &FeedLeadersCountRequest{
			IncludeComments:   false,
			IncludeLiked:      false,
			IncludeReposts:    false,
			MediumEntityTypes: []string{},
			LeaderEntityTypes: []string{},
		},
	}
}

type FeedLeadersBrowseRequest struct {
	IncludeComments   bool
	IncludeLiked      bool
	IncludeReposts    bool
	MediumEntityTypes []string
	LeaderEntityTypes []string
	Offset            uint64
	Limit             uint64
}

type FeedLeadersCountRequest struct {
	IncludeComments   bool
	IncludeLiked      bool
	IncludeReposts    bool
	MediumEntityTypes []string
	LeaderEntityTypes []string
}
