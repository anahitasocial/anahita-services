package constants

// What a profile's feed is asked for: the three lists a profile shows.
//
//	posts     what was posted on the profile, and what its owner reposted.
//	          Not replies.
//	replies   the replies its owner wrote, wherever the threads are.
//	reposts   what its owner reposted, and nothing else.
const (
	FeedFilterPosts   = "posts"
	FeedFilterReplies = "replies"
	FeedFilterReposts = "reposts"
)
