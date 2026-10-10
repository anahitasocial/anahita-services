package constants

const (
	// Services
	PersonService = "person-service"

	// Node Types
	NodeActorPrefixEntityType  = "node.actor."
	NodeMediumPrefixEntityType = "node.medium."
	// A photo post. The one kind of post the shared code has to tell from
	// the others, because it alone holds several images.
	NodeMediumPhotoEntityType  = "node.medium.photo-service.photo.v1"
	NodeActorPersonEntityType  = "node.actor.person-service.person.v1"
	NodeBaseRepostType         = "node.base.repost-service.repost.v1"
	NodeBaseHashtagEntityType  = "node.tag.hashtag-service.hashtag.v1"
	NodeBaseLocationEntityType = "node.tag.location-service.location.v1"

	// Edge Types
	EdgeActorAdminEntityType = "edge.actor.admin.v1"
	// From a post to a reply in its thread that has been hidden from the
	// thread. An edge and not a flag on the reply: the reply is its
	// author's, and "hidden" is a fact about the thread.
	EdgeMediumHiddenReplyType = "edge.medium.hidden-reply.v1"
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
