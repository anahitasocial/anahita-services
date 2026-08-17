package requests

import "anahita-libs/constants"

type FeedLeadersRequest struct {
	Browse *FeedLeadersBrowseRequest
}

func NewFeedLeadersRequest() *FeedLeadersRequest {
	return &FeedLeadersRequest{
		Browse: &FeedLeadersBrowseRequest{
			IncludeComments: false,
			IncludeReposts:  false,
			IncludeLiked:    false,
			Offset:          0,
			Limit:           constants.RequestLimitDefault,
		},
	}
}

type FeedLeadersBrowseRequest struct {
	IncludeComments bool   `query:"include_comments"`
	IncludeReposts  bool   `query:"include_reposts"`
	IncludeLiked    bool   `query:"include_liked"`
	Offset          uint64 `query:"start"`
	Limit           uint64 `query:"limit"`
}
