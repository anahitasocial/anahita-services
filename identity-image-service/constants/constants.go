package constants

const (
	// The largest file accepted, for an avatar or a cover. What arrives is
	// never stored as it is; every stored size is made from it and is far
	// smaller. So this is as much as the gateway lets one request carry,
	// which is what a picture straight off a phone needs.
	MaxFileSize = 10 * 1024 * 1024

	// The most pixels a picture may have: 50 megapixels. Checked before the
	// picture is opened.
	MaxPixels = 50_000_000

	// The largest any one stored size may be. Each goes to storage in one
	// message, and a message holds 4 MB.
	MaxStoredSize = 4*1024*1024 - 64*1024
)
