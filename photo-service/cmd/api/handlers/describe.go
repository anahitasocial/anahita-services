package handlers

import (
	"anahita-libs/apidoc"
	"anahita-libs/responses"
)

// What the routes take and send, for the API reference (cmd/api/routes.go).

// Uploaded is an image stored for a post that is not made yet.
type Uploaded struct {
	// What the post names it by, in `uploads`.
	UploadID int64 `json:"upload_id"`
	// In pixels.
	Width int `json:"width"`
	// In pixels.
	Height int `json:"height"`
	// Each size made of it, as a post's pictures are sent.
	URLs map[string]responses.Image `json:"urls"`
}

// FilesRefusal says why a post's images were refused, and how many a post
// may have.
type FilesRefusal struct {
	// Why: `files_required`, `too_many_files`, `file_not_found`,
	// `file_repeated`, `upload_not_yours`, `alt_text_too_long`,
	// `file_unreadable`, `file_unsupported` or `file_too_large`.
	Error string `json:"error"`
	// How many images a post may have.
	Max int `json:"max"`
}

// PhotoUpload is one image for a post. The handler reads these with
// c.Param and c.FormFile; nothing binds into it.
type PhotoUpload struct {
	// The profile the post will be on.
	OwnerID int64 `param:"owner_id"`
	// The image: a JPEG or a PNG, of at most 10 MB.
	File apidoc.File `form:"file" validate:"required"`
}

// PhotoDownloadRequest names an image of a post to download. The handler
// reads these with c.Param and c.QueryParam; nothing binds into it.
type PhotoDownloadRequest struct {
	// The post's id.
	ID int64 `param:"id"`
	// Which of its images, from 0. The first when left out.
	File int `query:"file"`
}
