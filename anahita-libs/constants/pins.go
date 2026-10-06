package constants

// MaxPinnedPosts is how many posts a profile can have pinned at once: one.
//
// One is what a profile leads with. It is also exactly what Bluesky has, a
// single pinned post on the profile, so a pin here is a pin there with
// nothing left over; Mastodon shows up to five and is content with one.
//
// Pinning another post moves the pin: the post that had it gives it up.
const MaxPinnedPosts = 1
