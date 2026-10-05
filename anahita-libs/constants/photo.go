package constants

// PhotoServiceName is the service a photo post belongs to. Part of where
// its images are stored.
const PhotoServiceName = "photo-service"

// PhotoMaxFiles is how many images one photo post can hold.
//
// Four, which is what Mastodon and Bluesky both allow, so a post goes to
// either whole. A constant and not a setting for that reason: an
// installation that allowed more would have its posts cut short elsewhere.
const PhotoMaxFiles = 4

// PhotoAltTextMaxLength is the longest description one image can have. The
// same limit Mastodon uses.
const PhotoAltTextMaxLength = 1500
