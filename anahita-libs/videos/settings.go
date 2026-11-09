package videos

import (
	"os"
	"strconv"
	"strings"
)

// What an installation says about video, read from the environment by
// every service that needs to know: the video service and its worker, which
// hold uploads to it, and auth-service, which tells the web app.
//
//	VIDEO_UPLOADS      "on" where people may post videos. Anything else, and
//	                   unset, is off: the two video containers cost storage
//	                   and processor time, and are not run by default.
//	VIDEO_MAX_BYTES    the largest file taken. 500 MB unless said.
//	VIDEO_MAX_SECONDS  the longest video taken. Ten minutes unless said.
//	ANIMATION_MAX_BYTES    the largest GIF or animated PNG taken. 25 MB
//	                       unless said, and never more than a video.
//	ANIMATION_MAX_SECONDS  the longest one. 30 seconds unless said, and
//	                       never more than a video.
const (
	EnvUploads             = "VIDEO_UPLOADS"
	EnvMaxBytes            = "VIDEO_MAX_BYTES"
	EnvMaxSeconds          = "VIDEO_MAX_SECONDS"
	EnvAnimationMaxBytes   = "ANIMATION_MAX_BYTES"
	EnvAnimationMaxSeconds = "ANIMATION_MAX_SECONDS"

	DefaultMaxBytes            = 500 * 1024 * 1024
	DefaultMaxSeconds          = 600
	DefaultAnimationMaxBytes   = 25 * 1024 * 1024
	DefaultAnimationMaxSeconds = 30
)

func positive(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(key)), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// UploadsOn says whether people may post videos here.
func UploadsOn() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvUploads)), "on")
}

func MaxBytes() int64 {
	return positive(EnvMaxBytes, DefaultMaxBytes)
}

func MaxSeconds() int {
	return int(positive(EnvMaxSeconds, DefaultMaxSeconds))
}

// AnimationMaxBytes is the largest moving picture taken: a GIF of two
// hundred megabytes is a mistake or an attack.
func AnimationMaxBytes() int64 {
	return min(positive(EnvAnimationMaxBytes, DefaultAnimationMaxBytes), MaxBytes())
}

func AnimationMaxSeconds() int {
	return min(int(positive(EnvAnimationMaxSeconds, DefaultAnimationMaxSeconds)), MaxSeconds())
}

// Videos and sounds from other networks, copied here when first played
// (roadmap rows 120 and 137, elsewhere.go):
//
//	REMOTE_MEDIA_MAX_BYTES    the largest file from elsewhere copied. 100 MB
//	                          unless said.
//	REMOTE_MEDIA_MAX_SECONDS  the longest one. An hour unless said.
//	REMOTE_VIDEO_DAYS         how many days a copy is kept after it was last
//	                          played. 7 unless said; 0 for never removed.
const (
	EnvRemoteMaxBytes   = "REMOTE_MEDIA_MAX_BYTES"
	EnvRemoteMaxSeconds = "REMOTE_MEDIA_MAX_SECONDS"
	EnvRemoteDays       = "REMOTE_VIDEO_DAYS"

	DefaultRemoteMaxBytes   = 100 * 1024 * 1024
	DefaultRemoteMaxSeconds = 3600
	DefaultRemoteDays       = 7
)

func RemoteMaxBytes() int64 { return positive(EnvRemoteMaxBytes, DefaultRemoteMaxBytes) }
func RemoteMaxSeconds() int {
	return int(positive(EnvRemoteMaxSeconds, DefaultRemoteMaxSeconds))
}

// RemoteDays is how long a copy from elsewhere is kept unplayed; 0 keeps it
// for ever.
func RemoteDays() int {
	raw := strings.TrimSpace(os.Getenv(EnvRemoteDays))
	if raw == "" {
		return DefaultRemoteDays
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 0 {
		return DefaultRemoteDays
	}
	return days
}
