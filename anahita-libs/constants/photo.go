package constants

// PhotoServiceName is the service a photo post belongs to. Part of where
// its images are stored.
const PhotoServiceName = "photo-service"

// PhotoMaxFiles is how many images one photo post can hold.
//
// Four, which is what a stock Mastodon server lets its users attach, so a
// post of four is shown whole there. Bluesky allows more, in its gallery
// embed. Whether to raise this, and make it a setting, is to be decided
// once federation is built and what other servers do with a longer post
// can be seen: docs/plans/federation.md, "How many photos a post can
// hold".
const PhotoMaxFiles = 4

// PhotoAltTextMaxLength is the longest description one image can have. The
// same limit Mastodon uses.
const PhotoAltTextMaxLength = 1500
