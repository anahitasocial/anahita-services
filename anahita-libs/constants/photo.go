package constants

import (
	"os"
	"strconv"
	"strings"
)

// PhotoServiceName is the service a photo post belongs to. Part of where
// its images are stored.
const PhotoServiceName = "photo-service"

// How many images one photo post can hold.
//
// Twenty unless the installation says fewer, in PHOTO_MAX_FILES (the user,
// 2026-10-19; roadmap row 100): twenty, as Instagram and Pixelfed; fewer
// where a server wants to hold less. Never more than twenty. A stock
// Mastodon server and Bluesky show four: they are sent the first four, or
// all, with a link to the post here, where the rest are.
const (
	EnvPhotoMaxFiles = "PHOTO_MAX_FILES"
	// PhotoMaxFilesCeiling is the most any installation may allow, and what
	// it allows unless it says fewer.
	PhotoMaxFilesCeiling = 20
)

// PhotoMaxFiles is how many images one photo post can hold here.
func PhotoMaxFiles() int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(EnvPhotoMaxFiles)))
	if err != nil || value <= 0 || value > PhotoMaxFilesCeiling {
		return PhotoMaxFilesCeiling
	}
	return value
}

// PhotoAltTextMaxLength is the longest description one image can have, as
// written here (AltTextMax).
const PhotoAltTextMaxLength = AltTextMax
