package constants

// What a profile's feed is asked for: the three lists a profile shows.
//
//	posts     what was posted on the profile, and what its owner reposted.
//	          Not replies.
//	replies   the replies its owner wrote, wherever the threads are.
//	reposts   what its owner reposted, and nothing else.
//	tagged    the photos and videos a person is in, their tag approved
//	          (roadmap row 103), whoever posted them and wherever.
const (
	FeedFilterPosts   = "posts"
	FeedFilterReplies = "replies"
	FeedFilterReposts = "reposts"
	FeedFilterTagged  = "tagged"
)
