package constants

// The kinds of actor are named for the services that used to serve them
// apart, "person-service" and "group-service". Those names are in every node
// and edge that was ever stored, and in where avatars and covers are kept, so
// they stay as they are: they are names now, not addresses. Both kinds are
// served by this one service.
const (
	NodeActorGroupType = "node.actor.group-service.group.v1"
	GroupService       = "group-service"

	// Mention
	EdgeTagMentionType = "edge.tag.mention.v1"
)
