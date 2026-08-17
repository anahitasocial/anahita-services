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
			TimeWindow: constants.TimeWindowWeekly,
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
	Offset          uint64  `query:"start"`
	Limit           uint64  `query:"limit"`
	Search          string  `query:"q" validate:"max=100,omitempty,search_query"`
	Ordering        string  `query:"sort" validate:"max=10,omitempty,oneof=popular trending recent"`
	Direction       string  `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	SourceID        int64   `query:"source_id" validate:"omitempty,numeric"`
	NearbyLatitude  float64 `query:"nearby_latitude" validate:"omitempty,latitude"`
	NearbyLongitude float64 `query:"nearby_longitude" validate:"omitempty,longitude"`
	NearbyRadius    uint64  `query:"nearby_radius" validate:"omitempty,numeric"`
}

type LocationBrowseOrderedByDegreeRequest struct {
	Offset uint64 `query:"start"`
	Limit  uint64 `query:"limit"`
}

type LocationBrowseTrendingRequest struct {
	Limit      uint64 `query:"limit"`
	TimeWindow string `query:"time_window" validate:"omitempty,oneof=daily weekly monthly all"`
}

type LocationReadRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}

type LocationEditRequest struct {
	ID            int64  `param:"id" validate:"required,numeric"`
	Name          string `form:"name" validate:"required,max=255"`
	Address       string `form:"address" validate:"required,max=255"`
	City          string `form:"city" validate:"required,max=100"`
	StateProvince string `form:"state_province" validate:"max=100"`
	Country       string `form:"country" validate:"required,max=50"`
}

type LocationAddRequest struct {
	Name          string `form:"name" validate:"required,max=255"`
	Address       string `form:"address" validate:"required,max=255"`
	City          string `form:"city" validate:"required,max=100"`
	StateProvince string `form:"state_province" validate:"max=100"`
	Country       string `form:"country" validate:"required,max=50"`
	SourceID      int64  `form:"source_id" validate:"omitempty,numeric"`
}

type LocationDeleteRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}
