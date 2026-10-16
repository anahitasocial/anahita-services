package handlers

// What the routes take and send, for the API reference (cmd/api/routes.go).

// UploadTooLarge refuses a video, a GIF or an animated PNG larger than the
// installation takes, and says how large it may be.
type UploadTooLarge struct {
	// Always `too_large`.
	Error string `json:"error"`
	// The most it may be, in bytes.
	MaxBytes int64 `json:"max_bytes"`
}

// UploadPart is where one part of an upload is sent.
type UploadPart struct {
	// Which part, from 1.
	Number int32 `json:"number"`
	// Where it is sent, with PUT: good for an hour.
	URL string `json:"url"`
}

// UploadRef names an upload. The handler reads it with c.Param; nothing
// binds into it.
type UploadRef struct {
	// The upload's id, as its start answered.
	ID string `param:"id" validate:"uuid"`
}

// VideoFileRequest names a file of a video post. The handler reads these
// with c.Param; nothing binds into it.
type VideoFileRequest struct {
	// The post's id.
	ID int64 `param:"id"`
	// Which file: one size of the video, by its height (`720.mp4`), or
	// `poster.jpg`.
	Name string `param:"name"`
}

// VideoDownloadRequest names a video post to download. The handler reads
// these with c.Param and c.QueryParam; nothing binds into it.
type VideoDownloadRequest struct {
	// The post's id.
	ID int64 `param:"id"`
	// Which size, by its height: `720`. The largest when left out.
	Size int `query:"size"`
}
