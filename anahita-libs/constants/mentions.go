package constants

// NodeActorGroupEntityType is a group.
const NodeActorGroupEntityType = "node.actor.group-service.group.v1"

// ProfileTypes are the kinds of profile a post can name: by its @handle, or
// by its address on a line of its own, embedded. People here, groups,
// events, websites, and people elsewhere, by each protocol.
var ProfileTypes = []string{
	NodeActorPersonEntityType,
	NodeActorGroupEntityType,
	NodeActorEventEntityType,
	NodeActorWebsiteEntityType,
	NodeActorActivityPubPersonEntityType,
	NodeActorATProtoPersonEntityType,
}

// MentionPermission is the permission a person sets for who may mention
// them (the user, 2026-10-17), under socialgraph-service: one of the access
// levels a like takes. Left unset, anybody signed in may.
const (
	MentionPermission        = "mention"
	MentionPermissionService = "socialgraph-service"
)
