package constants

import "time"

const (
	RequestBodyMaxSizeLimit = "2M"
	// For the services that take an uploaded image: a photo, an avatar, a
	// cover. One file may be 10 MB (their MaxFileSize), and the form it
	// arrives in adds a little. The gateway stops a request at 10M first.
	//
	// They used the limit above, 2M, so anything over that was refused
	// before the 10 MB check was reached: a picture straight off a phone,
	// which is what the 10 MB is for.
	ImageRequestBodyMaxSizeLimit = "11M"
	RequestLimitDefault          = 20
	SessionTimeout               = 60 * 60 * 24 * 7 // 7 days
	ViewerContextKey             = "viewer"

	DefaultPageSize = 20
	MaxPageSize     = 100
	MaxFetchSize    = 10000

	// db
	DBTimeout = 3 * time.Second

	// Ordering
	OrderingPopular  = "popular"
	OrderingTrending = "trending"
	OrderingRecent   = "recent"
	// For a list of who is connected to something, such as a profile's
	// followers: the newest connection first.
	OrderingConnected = "connected"
	OrderingOldest    = "oldest"
	OrderingNewest    = "newest"
	OrderingCreatedAt = "created_at"
	OrderingUpdatedAt = "updated_at"
	DirAsc            = "asc"
	DirDesc           = "desc"

	// Trending Interval
	TrendingInterval = 365 // days
)
