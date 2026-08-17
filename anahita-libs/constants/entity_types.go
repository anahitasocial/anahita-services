package constants

const (
	// Services
	PersonService  = "person-service"
	CommentService = "comment-service"

	// Node Types
	NodeActorPrefixEntityType  = "node.actor."
	NodeMediumPrefixEntityType = "node.medium."
	NodeActorPersonEntityType  = "node.actor.person-service.person.v1"
	NodeBaseCommentEntityType  = "node.base.comment-service.comment.v1"
	NodeBaseRepostType         = "node.base.repost-service.repost.v1"
	NodeBaseHashtagEntityType  = "node.tag.hashtag-service.hashtag.v1"
	NodeBaseLocationEntityType = "node.tag.location-service.location.v1"

	// Edge Types
	EdgeActorAdminEntityType       = "edge.actor.admin.v1"
	EdgeFollowEntityType           = "edge.actor.follow.v1"
	EdgeFollowRequestEntityType    = "edge.actor.follow-request.v1"
	EdgeBlockEntityType            = "edge.actor.block.v1"
	EdgeBaseSubscriptionEntityType = "edge.base.subscription.v1"
	EdgeBaseLikeEntityType         = "edge.base.voteup.v1"
	EdgeTagHashtagType             = "edge.tag.hashtag.v1"
	EdgeTagLocationType            = "edge.tag.location.v1"
)
