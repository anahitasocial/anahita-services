package constants

// How long what people write may be, the same in every service and in the
// web app (anahita-web src/constants/limits.js), and fit for the networks
// it goes to (the user, 2026-10-17). Counted in characters (runes); a
// browser counts an emoji as two, so it is the stricter of the two.
//
// The validate tags of the requests carry these numbers too, as Go's struct
// tags cannot name a constant; requests/limits_test.go holds them to these.
const (
	// A display name: a person's, a group's, an event's. Bluesky's
	// (64 graphemes); Mastodon writes 40 and takes up to 2,048 from
	// elsewhere.
	DisplayNameMax = 64
	// A person's username, and so their handle: Mastodon's.
	UsernameMin = 3
	UsernameMax = 30
	// What a person or a group says of itself: Mastodon's. Bluesky's is
	// 256, but a profile here is never written there.
	BioMax = 500
	// A post's title: an article's, a topic's, a photo's, a video's.
	TitleMax = 100
	// What a post says: a note, a reply, an article, a topic, a photo's or
	// a video's description; and an event's details. Mastodon shows any
	// length it is sent; Bluesky is sent the first 300, and a link to the
	// rest.
	PostBodyMax = 8000
	// A place's name.
	PlaceNameMax = 100
	// An image's description, as written here: what Bluesky's own app
	// takes. One from elsewhere is kept up to AltTextKeptMax, Mastodon's.
	AltTextMax     = 2000
	AltTextKeptMax = 10000
)
