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
		PublicAccess:      NewPublicAccessRepo(db),
		PurgeRequests:     NewPurgeRequestsRepo(db),
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
	PublicAccess      PublicAccessRepoInterface
	PurgeRequests     PurgeRequestsRepoInterface
	Reposts           RepostsRepoInterface
	Search            SearchRepoInterface
	SocialgraphRel    SocialgraphRelInterface
	SocialGraphCounts SocialGraphCountsInterface
	SubscriptionsRel  SubscriptionsRelInterface
}
