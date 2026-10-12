package entities

// Where pictures are kept in storage.
//
// v1 is the layout of this platform's first release. The PHP application
// before it kept its files under assets/public, which is v0 in all but
// name; and while this platform was being built its files were under
// assets/v2, with avatars and covers in folders named for two services
// that have since become one. scripts/s3-migration moves a bucket from
// either to this.
//
//	assets/v1/identity-image-service/avatars/<kind>/<id>/<name>_<size>.<ext>
//	assets/v1/identity-image-service/covers/<kind>/<id>/<name>_<size>.<ext>
//	assets/v1/photo-service/photo/<owner id>/<name>_<size>.<ext>
const (
	AssetsPrefix = "assets/v1/"
	// Avatars and covers: what a profile or a post is recognised by. Named
	// for the service that looks after them.
	IdentityImagesPrefix = AssetsPrefix + "identity-image-service/"
)
