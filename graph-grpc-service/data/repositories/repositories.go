package repositories

import (
	"database/sql"
)

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		ActorAdmins:       NewActorAdminRepo(db),
		NodeDeletionLogs:  NewNodeDeletionLogsRepo(db),
		Edges:             NewEdgesRepo(db),
		FeedActor:         NewFeedActorRepo(db),
		FeedLeaders:       NewFeedLeadersRepo(db),
		Graph:             NewGraphRepo(db),
		LikesRel:          NewLikesRelRepo(db),
		Nodes:             NewNodesRepo(db),
		PhotoFiles:        NewPhotoFilesRepo(db),
		PublicAccess:      NewPublicAccessRepo(db),
		PurgeRequests:     NewPurgeRequestsRepo(db),
		Replies:           NewRepliesRepo(db),
		Quotes:            NewQuotesRepo(db),
		Pins:              NewPinsRepo(db),
		Reposts:           NewRepostsRepo(db),
		Search:            NewSearchRepo(db),
		SocialgraphRel:    NewSocialgraphRelRepo(db),
		SocialGraphCounts: NewSocialGraphCountsRepo(db),
		SubscriptionsRel:  NewSubscriptionsRelRepo(db),
	}
}

type Repositories struct {
	ActorAdmins       ActorAdminRepoInterface
	NodeDeletionLogs  NodeDeletionLogsRepoInterface
	Edges             EdgesRepoInterface
	FeedActor         FeedActorRepoInterface
	FeedLeaders       FeedLeadersRepoInterface
	Graph             GraphRepoInterface
	LikesRel          LikesRelInterface
	Nodes             NodesRepoInterface
	PhotoFiles        PhotoFilesRepoInterface
	PublicAccess      PublicAccessRepoInterface
	PurgeRequests     PurgeRequestsRepoInterface
	Replies           RepliesRepoInterface
	Quotes            QuotesRepoInterface
	Pins              PinsRepoInterface
	Reposts           RepostsRepoInterface
	Search            SearchRepoInterface
	SocialgraphRel    SocialgraphRelInterface
	SocialGraphCounts SocialGraphCountsInterface
	SubscriptionsRel  SubscriptionsRelInterface
}
