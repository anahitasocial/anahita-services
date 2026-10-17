package constants

// The most hashtags a post is filed under (the user, 2026-10-17). A soft
// cap: a post may say more, from here or from any other server, and is
// kept whole, but only the first this many in its text, each counted once
// whatever its capitals, become hashtags: linked, filed and sent out as
// tags. The rest are words. Ten is what a custom feed may filter on too
// (feeds.MaxHashtags); the fediverse's servers set no limit, Bluesky only
// on the extra tags a post carries outside its text (eight).
const MaxHashtagsOfAPost = 10

// The most places a post is tied to (the user, 2026-10-17). A hard cap:
// places are added one at a time, on purpose, and the one past it is
// refused (`too_many_locations`). Five is what a custom feed may filter on
// too (feeds.MaxLocations).
const MaxLocationsOfAPost = 5
