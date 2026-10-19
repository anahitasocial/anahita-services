package constants

// The kinds of thing a person may choose not to be told about (roadmap row
// 97, part 5), each some of the kinds of notification. Everything is on to
// begin with; one turned off is a row in notifications_off.
const (
	TopicLikes       = "likes"
	TopicReposts     = "reposts"
	TopicReplies     = "replies"
	TopicMentions    = "mentions"
	TopicFollows     = "follows"
	TopicPosts       = "posts"
	TopicEvents      = "events"
	TopicInvitations = "invitations"
)

// Topics are all of them, in the order they are offered.
var Topics = []string{
	TopicLikes,
	TopicReposts,
	TopicReplies,
	TopicMentions,
	TopicFollows,
	TopicPosts,
	TopicEvents,
	TopicInvitations,
}

var topicOf = map[string]string{
	"voteup":                      TopicLikes,
	"repost_add":                  TopicReposts,
	"quote_add":                   TopicReposts,
	"reply_add":                   TopicReplies,
	"actor_mention":               TopicMentions,
	"actor_mention_reply":         TopicMentions,
	"actor_follow":                TopicFollows,
	"actor_follow_request":        TopicFollows,
	"actor_follow_request_accept": TopicFollows,
	"note_add":                    TopicPosts,
	"article_add":                 TopicPosts,
	"photo_add":                   TopicPosts,
	"topic_add":                   TopicPosts,
	"video_add":                   TopicPosts,
	"event_rsvp":                  TopicEvents,
	"event_changed":               TopicEvents,
	"event_cancelled":             TopicEvents,
	"event_reminder":              TopicEvents,
	"actor_invite":                TopicInvitations,
	"actor_invite_accept":         TopicInvitations,
	"admin_add":                   TopicInvitations,
}

// TopicOf is the kind of thing a notification is about, or "" for one no
// choice turns off.
func TopicOf(notificationType string) string {
	return topicOf[notificationType]
}

// IsTopic says whether it is one of them.
func IsTopic(topic string) bool {
	for _, each := range Topics {
		if each == topic {
			return true
		}
	}
	return false
}
