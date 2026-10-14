package constants

import "anahita-libs/videos"

const (
	// Entity types
	NodeMediumVideoType = videos.NodeType
	VideoService        = videos.Service

	// How big each part of an upload is. A store wants every part but the
	// last to be at least 5 MB; larger parts are fewer requests, smaller
	// ones lose less when a connection drops.
	PartSize = 8 * 1024 * 1024

	// How long an upload may sit unfinished before it is thrown away.
	AbandonedAfterHours = 24

	// How many times a worker tries a video before giving up on it.
	MaxAttempts = 3
)

// What a browser may say it is sending. What it really is is decided by
// the worker, which reads it; this only turns away what could never be one.
// A GIF or an animated PNG is posted here too, as a clip that loops.
var UploadTypes = []string{"video/mp4", "video/quicktime", "video/webm", "image/gif", "image/png"}

// AnimationTypes are those of UploadTypes that are moving pictures, held to
// the smaller limits of one.
var AnimationTypes = []string{"image/gif", "image/png"}

// Why a video failed, as the uploader is told.
const (
	FailedNotAVideo = "not_a_video"
	FailedTooLong   = "too_long"
	FailedTooLarge  = "too_large"
	FailedEncoding  = "encoding"
	// A GIF or a PNG that does not move: a photo, posted as one.
	FailedStill = "still"
)
