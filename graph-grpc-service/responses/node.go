package responses

type NodeReadConnectionStatsResponse struct {
	FollowerCount   uint64
	LeaderCount     uint64
	SubscriberCount uint64
	CommentCount    uint64
	VoteUpCount     uint64
	RepostCount     uint64
	QuoteCount      uint64
}
