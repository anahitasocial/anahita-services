package requests

import "anahita-libs/constants"

type SearchRequest struct {
	Browse *SearchBrowseRequest
}

func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		Browse: &SearchBrowseRequest{
			Offset:      0,
			Limit:       constants.RequestLimitDefault,
			Ordering:    constants.OrderingRecent,
			OrderingDir: constants.DirDesc,
			Keywords:    "",
			Scope:       "all",
			CoordRadius: 0, // 0 is infinity!
		},
	}
}

type SearchBrowseRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// The order: the most relevant first, or the latest.
	Ordering string `query:"sort" validate:"max=15,omitempty,oneof=relevant recent"`
	// Which way the order runs.
	OrderingDir string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	// The words looked for.
	Keywords string `query:"term" validate:"required,max=100,search_query"`
	// What is looked among: everything, posts of every kind, photos and
	// videos alone (media), profiles, or custom feeds.
	Scope string `query:"scope" validate:"omitempty,max=20,oneof=all posts media actors feeds"`
	// How far from coord_lat and coord_lng, in metres, when both are given.
	CoordRadius int32 `query:"search_range" validate:"number,omitempty"`
	// Where to search near: longitude.
	CoordLng float32 `query:"coord_lng" validate:"number,omitempty"`
	// Where to search near: latitude.
	CoordLat float32 `query:"coord_lat" validate:"number,omitempty"`
	// The searcher's own choices for "Most relevant": people and groups
	// they follow first, and interest that fades within a month.
	FollowedFirst bool `query:"followed_first"`
	RecentFirst   bool `query:"recent_first"`
}
