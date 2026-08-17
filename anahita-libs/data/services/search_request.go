package services

import "anahita-libs/constants"

type SearchRequest struct {
	Browse *SearchBrowseRequest
}

func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		Browse: &SearchBrowseRequest{
			Offset:          0,
			Limit:           constants.RequestLimitDefault,
			Ordering:        constants.OrderingRecent,
			OrderingDir:     constants.DirDesc,
			Keywords:        "",
			Scope:           "all",
			IncludeComments: false,
			CoordRadius:     0, // 0 is infinity!
			AccessCheck:     true,
		},
	}
}

type SearchBrowseRequest struct {
	Offset          uint64  `query:"start"`
	Limit           uint64  `query:"limit"`
	Ordering        string  `query:"sort" validate:"max=15,omitempty,oneof=relevant recent"`
	OrderingDir     string  `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	Keywords        string  `query:"term" validate:"required,max=100,search_query"`
	Scope           string  `query:"scope" validate:"omitempty,max=20,oneof=all media actors"`
	IncludeComments bool    `query:"search_comments" validate:"omitempty"`
	CoordRadius     int32   `query:"search_range" validate:"number,omitempty"`
	CoordLng        float32 `query:"coord_lng" validate:"number,omitempty"`
	CoordLat        float32 `query:"coord_lat" validate:"number,omitempty"`
	AccessCheck     bool    `query:"access_check" validate:"omitempty"`
}
