package requests

import "anahita-libs/constants"

type LocationRequest struct {
	Browse                *LocationBrowseRequest
	BrowseTrending        *LocationBrowseTrendingRequest
	BrowseOrderedByDegree *LocationBrowseOrderedByDegreeRequest
	Read                  *LocationReadRequest
	Edit                  *LocationEditRequest
	Add                   *LocationAddRequest
	Delete                *LocationDeleteRequest
}

func NewLocationRequest() *LocationRequest {
	return &LocationRequest{
		Browse: &LocationBrowseRequest{
			NearbyLatitude:  0,
			NearbyLongitude: 0,
			NearbyRadius:    20000,
			Offset:          0,
			Limit:           constants.RequestLimitDefault,
			Direction:       "desc",
			Ordering:        "recent",
		},
		BrowseTrending: &LocationBrowseTrendingRequest{
			Limit:      constants.RequestLimitDefault,
			TimeWindow: "",
		},
		BrowseOrderedByDegree: &LocationBrowseOrderedByDegreeRequest{
			Offset: 0,
			Limit:  constants.RequestLimitDefault,
		},
		Read:   &LocationReadRequest{},
		Edit:   &LocationEditRequest{},
		Add:    &LocationAddRequest{},
		Delete: &LocationDeleteRequest{},
	}
}

type LocationBrowseRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// Words its name or address holds.
	Search string `query:"q" validate:"max=100,omitempty,search_query"`
	// The order they come in.
	Ordering string `query:"sort" validate:"max=10,omitempty,oneof=popular trending recent name"`
	// Which way the order runs.
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	// Only the places a post is tied to: the post's id.
	SourceID int64 `query:"source_id" validate:"omitempty,numeric"`
	// Only places near here: latitude.
	NearbyLatitude float64 `query:"nearby_latitude" validate:"omitempty,latitude"`
	// Only places near here: longitude.
	NearbyLongitude float64 `query:"nearby_longitude" validate:"omitempty,longitude"`
	// How near, in metres. The nearest come first.
	NearbyRadius uint64 `query:"nearby_radius" validate:"omitempty,numeric"`
}

type LocationBrowseOrderedByDegreeRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
}

type LocationBrowseTrendingRequest struct {
	// The most to send.
	Limit uint64 `query:"limit"`
	// Asked for in lower case and told to the graph in upper case, as its
	// constants are. "all" was accepted and never understood: the graph
	// fell back to a day for anything it did not know.
	TimeWindow string `query:"time_window" validate:"omitempty,oneof=daily weekly monthly yearly"`
}

type LocationReadRequest struct {
	// The place's id.
	ID int64 `param:"id" validate:"required,numeric"`
}

type LocationEditRequest struct {
	// The place's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// What it is called.
	Name string `form:"name" validate:"required,max=255"`
	// Its street address.
	Address string `form:"address" validate:"required,max=255"`
	// Its city.
	City string `form:"city" validate:"required,max=100"`
	// Its state or province.
	StateProvince string `form:"state_province" validate:"max=100"`
	// Its country.
	Country string `form:"country" validate:"required,max=50"`
}

type LocationAddRequest struct {
	// What it is called.
	Name string `form:"name" validate:"required,max=255"`
	// Its street address.
	Address string `form:"address" validate:"required,max=255"`
	// Its city.
	City string `form:"city" validate:"required,max=100"`
	// Its state or province.
	StateProvince string `form:"state_province" validate:"max=100"`
	// Its country.
	Country string `form:"country" validate:"required,max=50"`
	// A post to tie to it at once: the post's id.
	SourceID int64 `form:"source_id" validate:"omitempty,numeric"`
}

type LocationDeleteRequest struct {
	// The place's id.
	ID int64 `param:"id" validate:"required,numeric"`
}
