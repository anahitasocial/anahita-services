package constants

const (
	// Entity types
	NodeMediumPhotoType = "node.medium.photo-service.photo.v1"
	PhotoService        = "photo-service"

	// The largest image accepted. What arrives is never stored as it is:
	// every stored size is made from it, and the largest of those is far
	// smaller. So this is as much as the gateway lets one request carry,
	// which is what a photo straight off a phone needs.
	MaxFileSize = 10 * 1024 * 1024

	// The most pixels a picture may have: 50 megapixels, which is beyond
	// what a phone takes. Checked before the picture is opened.
	MaxPixels = 50_000_000

	// The largest any one stored size may be. Each goes to storage in one
	// message, and a message holds 4 MB.
	MaxStoredSize = 4*1024*1024 - 64*1024

	// The widest the "original" size is stored. A photo is never enlarged
	// to this; one that is wider is brought down to it.
	OriginalMaxWidth = 2560
)
