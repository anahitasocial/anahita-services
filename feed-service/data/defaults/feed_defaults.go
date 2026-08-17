package defaults

// FeedDefaults represents the default filters for feeds.
type FeedDefaults struct {
	MediaEntityTypes []string `json:"media_entity_types"`
	ActorEntityTypes []string `json:"actor_entity_type"`
}
