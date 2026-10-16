package requests

import "anahita-libs/constants"

type FeedLeadersRequest struct {
	Browse *FeedLeadersBrowseRequest
}

func NewFeedLeadersRequest() *FeedLeadersRequest {
	return &FeedLeadersRequest{
		Browse: &FeedLeadersBrowseRequest{
			IncludeReposts: false,
			IncludeLiked:   false,
			Offset:         0,
			Limit:          constants.RequestLimitDefault,
		},
	}
}

type FeedLeadersBrowseRequest struct {
	// Reposts by the profiles the viewer follows as well.
	IncludeReposts bool `query:"include_reposts"`
	// What they liked as well.
	IncludeLiked bool `query:"include_liked"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
}
