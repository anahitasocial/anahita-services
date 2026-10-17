package constants

const (
	// Services
	PersonService = "person-service"
	// The `service` of an event's node and a website's: actor-service serves
	// them. A group's is still the older group-service.
	ActorService = "actor-service"
	// The `service` of a note's, an article's and a topic's node.
	TextService = "text-service"

	// Node Types
	NodeActorPrefixEntityType  = "node.actor."
	NodeMediumPrefixEntityType = "node.medium."
	// A photo post. The one kind of post the shared code has to tell from
	// the others, because it alone holds several images.
	NodeMediumPhotoEntityType = "node.medium.photo-service.photo.v1"
	NodeActorPersonEntityType = "node.actor.person-service.person.v1"
	// An event: an actor people say they are going to. Served by
	// actor-service, and named for it. See constants/events.go.
	NodeActorEventEntityType = "node.actor.actor-service.event.v1"
	// A note: the kind of post that arrives from another server.
	NodeMediumNoteEntityType = "node.medium.text-service.note.v1"
	// An article: long content with a title. What a website posts for each
	// item of its feed.
	NodeMediumArticleEntityType = "node.medium.text-service.article.v1"
	// A website: a site's feed, verified, as a profile to follow. Served by
	// actor-service as a group is; connected, and its feed read, by
	// Website-service (docs/plans/website.md).
	NodeActorWebsiteEntityType = "node.actor.actor-service.website.v1"
	// Somebody on another server, one type for each protocol they are
	// reached by. Types of their own, and not the person type with a flag:
	// everything that asks for people by type (the people list, who a name
	// in a post means, the count of members) then leaves them out without
	// having been told to. Their names end as a person's does, so what asks
	// whether a node is person-like still says yes.
	NodeActorActivityPubPersonEntityType = "node.actor.activitypub-service.person.v1"
	NodeActorATProtoPersonEntityType     = "node.actor.atproto-service.person.v1"
	NodeBaseRepostType                   = "node.base.repost-service.repost.v1"
	NodeBaseHashtagEntityType            = "node.tag.hashtag-service.hashtag.v1"
	NodeBaseLocationEntityType           = "node.tag.location-service.location.v1"

	// Edge Types
	EdgeActorAdminEntityType = "edge.actor.admin.v1"
	// A group hosting an event: from the group to the event. Set when the
	// event is made, and what puts it on the group's list of events.
	EdgeActorHostEntityType = "edge.actor.host.v1"
	// From a post to a reply in its thread that has been hidden from the
	// thread. An edge and not a flag on the reply: the reply is its
	// author's, and "hidden" is a fact about the thread.
	EdgeMediumHiddenReplyType = "edge.medium.hidden-reply.v1"
	// A citation: from a post to a post of this installation it shows
	// within its text, an address on a line of its own in an article or a
	// topic. Written by text-service as the post is saved; for search and
	// recommendations to tell which posts belong together (the user named
	// it, 2026-10-16).
	EdgeMediumCitationType = "edge.medium.citation.v1"
	// From a person to a post they have saved, to find again. Private:
	// nobody but that person is ever told it is there, the post's author
	// included, and nothing is counted from it.
	EdgeBaseBookmarkEntityType  = "edge.base.bookmark.v1"
	EdgeFollowEntityType        = "edge.actor.follow.v1"
	EdgeFollowRequestEntityType = "edge.actor.follow-request.v1"
	// A person invited to follow an actor: from the person to the actor,
	// created by whoever invited them. See constants/invites.go.
	EdgeActorInviteEntityType      = "edge.actor.invite.v1"
	EdgeBlockEntityType            = "edge.actor.block.v1"
	EdgeBaseSubscriptionEntityType = "edge.base.subscription.v1"
	EdgeBaseLikeEntityType         = "edge.base.voteup.v1"
	EdgeTagHashtagType             = "edge.tag.hashtag.v1"
	EdgeTagLocationType            = "edge.tag.location.v1"
)
