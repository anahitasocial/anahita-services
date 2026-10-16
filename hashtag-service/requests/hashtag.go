package requests

import "anahita-libs/constants"

type HashtagRequest struct {
	Browse                *HashtagBrowseRequest
	BrowseTrending        *HashtagBrowseTrendingRequest
	BrowseOrderedByDegree *HashtagBrowseOrderedByDegreeRequest
	Read                  *HashtagReadRequest
	Edit                  *HashtagEditRequest
	Add                   *HashtagAddRequest
	Delete                *HashtagDeleteRequest
}

func NewHashtagRequest() *HashtagRequest {
	return &HashtagRequest{
		Browse: &HashtagBrowseRequest{
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			OrderingDir: constants.DirDesc,
			Ordering:    constants.OrderingRecent,
		},
		BrowseTrending: &HashtagBrowseTrendingRequest{
			Limit:      constants.RequestLimitDefault,
			TimeWindow: "weekly",
		},
		BrowseOrderedByDegree: &HashtagBrowseOrderedByDegreeRequest{
			Offset: 0,
			Limit:  constants.RequestLimitDefault,
		},
		Read:   &HashtagReadRequest{},
		Edit:   &HashtagEditRequest{},
		Add:    &HashtagAddRequest{},
		Delete: &HashtagDeleteRequest{},
	}
}

type HashtagBrowseRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// The order: in the most posts first, or the latest first.
	Ordering string `query:"sort" validate:"max=10,omitempty,oneof=popular recent"`
	// Which way the order runs.
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
}

type HashtagBrowseOrderedByDegreeRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
}

type HashtagBrowseTrendingRequest struct {
	// The most to send.
	Limit uint64 `query:"limit"`
	// Asked for in lower case and told to the graph in upper case, as its
	// constants are. "all" was accepted and never understood: the graph
	// fell back to a day for anything it did not know.
	TimeWindow string `query:"time_window" validate:"omitempty,oneof=daily weekly monthly yearly"`
}

type HashtagReadRequest struct {
	Alias string `param:"alias" validate:"required,hashtag"`
}

type HashtagEditRequest struct {
	Alias string `param:"alias" validate:"required,hashtag"`
}

type HashtagAddRequest struct{}

type HashtagDeleteRequest struct {
	// The hashtag's id.
	ID int64 `param:"id" validate:"required,number"`
}
