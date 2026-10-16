package responses

// Pagination is where a page is among everything there is to list.
type Pagination struct {
	// How many come before this page: what was asked for as `start`.
	Offset uint64 `json:"offset"`
	// The most a page holds: what was asked for as `limit`.
	Limit uint64 `json:"limit"`
	// How many there are in all.
	Total uint64 `json:"total"`
}

// Unpaged is a list sent whole, not a page of it: the answer of a route
// that lists a few things and says no more of them.
type Unpaged[T any] struct {
	// What there is.
	Data []T `json:"data"`
}

// List is a page of things, and where it is among them: the answer of
// every route that lists. Its `data` is null when the page is empty.
type List[T any] struct {
	// What is on this page.
	Data []T `json:"data"`
	// Where this page is among everything there is.
	Pagination Pagination `json:"pagination"`
}
