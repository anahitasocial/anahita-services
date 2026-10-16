package handlers

// What the routes take and send, for the API reference (cmd/api/routes.go).

// LookupRequest asks for somebody on Bluesky. The handler reads it with
// c.QueryParam; nothing binds into it.
type LookupRequest struct {
	// What was typed: a handle with or without its @ (alice.bsky.social),
	// a DID, or the address of their profile page.
	Q string `query:"q"`
}

// PostPicturesRequest names a post from Bluesky. The handler reads it
// with c.Param; nothing binds into it.
type PostPicturesRequest struct {
	// The post's id here.
	ID int64 `param:"id"`
}

// ImportPreview is what following here whoever the viewer follows on
// Bluesky would do, before it is done.
type ImportPreview struct {
	// How many accounts the viewer follows on Bluesky.
	Following int `json:"following"`
	// The most followed here in one go.
	Most int `json:"most"`
	// How many of each one's latest posts are read.
	PostsOfEach int `json:"posts_of_each"`
}

// ImportStarted says following them here has begun: it carries on after
// the answer, and for a long list takes minutes.
type ImportStarted struct {
	// Always true.
	Started bool `json:"started"`
	// How many will be followed.
	Limit int `json:"limit"`
}

// PicturesRestored says the pictures of a post are here again.
type PicturesRestored struct {
	// How many were copied.
	Restored int `json:"restored"`
}
