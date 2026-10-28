package repositories

import (
	"database/sql"
)

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		ActorAdmins:        NewActorAdminRepo(db),
		NodeDeletionLogs:   NewNodeDeletionLogsRepo(db),
		Edges:              NewEdgesRepo(db),
		Events:             NewEventsRepo(db),
		FeedActor:          NewFeedActorRepo(db),
		FeedCustom:         NewFeedCustomRepo(db),
		FeedLeaders:        NewFeedLeadersRepo(db),
		Graph:              NewGraphRepo(db),
		Invites:            NewInvitesRepo(db),
		LikesRel:           NewLikesRelRepo(db),
		Nodes:              NewNodesRepo(db),
		PhotoFiles:         NewPhotoFilesRepo(db),
		ProfileRules:       NewProfileRulesRepo(db),
		PublicAccess:       NewPublicAccessRepo(db),
		PurgeRequests:      NewPurgeRequestsRepo(db),
		Replies:            NewRepliesRepo(db),
		Quotes:             NewQuotesRepo(db),
		Citations:          NewCitationsRepo(db),
		Pins:               NewPinsRepo(db),
		Bookmarks:          NewBookmarksRepo(db),
		Reposts:            NewRepostsRepo(db),
		Search:             NewSearchRepo(db),
		HashtagSuggestions: NewHashtagSuggestionsRepo(db),
		SocialgraphRel:     NewSocialgraphRelRepo(db),
		SocialGraphCounts:  NewSocialGraphCountsRepo(db),
		SubscriptionsRel:   NewSubscriptionsRelRepo(db),
		Tags:               NewTagsRepo(db),
	}
}

type Repositories struct {
	ActorAdmins        ActorAdminRepoInterface
	NodeDeletionLogs   NodeDeletionLogsRepoInterface
	Edges              EdgesRepoInterface
	Events             EventsRepoInterface
	FeedActor          FeedActorRepoInterface
	FeedCustom         FeedCustomRepoInterface
	FeedLeaders        FeedLeadersRepoInterface
	Graph              GraphRepoInterface
	Invites            InvitesRepoInterface
	LikesRel           LikesRelInterface
	Nodes              NodesRepoInterface
	PhotoFiles         PhotoFilesRepoInterface
	ProfileRules       ProfileRulesRepoInterface
	PublicAccess       PublicAccessRepoInterface
	PurgeRequests      PurgeRequestsRepoInterface
	Replies            RepliesRepoInterface
	Quotes             QuotesRepoInterface
	Citations          CitationsRepoInterface
	Pins               PinsRepoInterface
	Bookmarks          BookmarksRepoInterface
	Reposts            RepostsRepoInterface
	Search             SearchRepoInterface
	HashtagSuggestions HashtagSuggestionsRepoInterface
	SocialgraphRel     SocialgraphRelInterface
	SocialGraphCounts  SocialGraphCountsInterface
	SubscriptionsRel   SubscriptionsRelInterface
	Tags               TagsRepoInterface
}
