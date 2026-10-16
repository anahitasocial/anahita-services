package handlers

import "anahita-libs/apidoc"

// What the routes take, for the API reference (routes.go). The handlers
// read these values themselves, with c.Param and c.FormFile by the same
// names; nothing binds into these structs.

// NodeImageRequest names the node an image is for.
type NodeImageRequest struct {
	// The id of the profile or the post.
	NodeID int64 `param:"node_id"`
}

// AvatarUpload is a new avatar.
type AvatarUpload struct {
	// The id of the profile.
	NodeID int64 `param:"node_id"`
	// The picture: a JPEG or a PNG, of at most 10 MB.
	Avatar apidoc.File `form:"avatar" validate:"required"`
}

// CoverUpload is a new cover.
type CoverUpload struct {
	// The id of the profile or the post.
	NodeID int64 `param:"node_id"`
	// The picture: a JPEG or a PNG, of at most 10 MB.
	Cover apidoc.File `form:"cover" validate:"required"`
}

// PublicImageRequest names a public profile's picture at an address that
// does not change.
type PublicImageRequest struct {
	// The node's UUID, which other servers are given.
	UUID string `param:"uuid"`
	// Which size: for an avatar `small`, `medium`, `large` or `square`; for
	// a cover `medium` or `large`.
	Size string `param:"size"`
}
