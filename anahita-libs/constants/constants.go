package constants

import "time"

const (
	RequestBodyMaxSizeLimit = "2M"
	RequestLimitDefault     = 20
	SessionTimeout          = 60 * 60 * 24 * 7 // 7 days
	ViewerContextKey        = "viewer"

	DefaultPageSize = 20
	MaxPageSize     = 100
	MaxFetchSize    = 10000

	// db
	DBTimeout = 3 * time.Second

	// Ordering
	OrderingPopular   = "popular"
	OrderingTrending  = "trending"
	OrderingRecent    = "recent"
	OrderingOldest    = "oldest"
	OrderingNewest    = "newest"
	OrderingCreatedOn = "created_on"
	OrderingUpdatedOn = "updated_on"
	DirAsc            = "asc"
	DirDesc           = "desc"

	// Trending Interval
	TrendingInterval = 365 // days
)
