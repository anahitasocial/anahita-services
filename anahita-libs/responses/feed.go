package responses

import (
	"encoding/json"

	"anahita-libs/data/entities"
	"anahita-libs/feeds"
)

// FeedBrief is what a custom feed is beyond its name and its owner, for
// where one is listed among other things, as in search results: its picture
// and what it shows.
type FeedBrief struct {
	Icon       string          `json:"icon"`
	Definition json.RawMessage `json:"definition"`
}

// NewFeedBrief reads it off a feed's node. Nil for a node that is not a
// feed, and for one whose definition cannot be read.
func NewFeedBrief(node *entities.AnahitaNode) *FeedBrief {
	if node == nil || node.Type != feeds.NodeType {
		return nil
	}

	stored, err := feeds.ReadStored(node.Meta)
	if err != nil {
		return nil
	}

	return &FeedBrief{
		Icon:       stored.Icon,
		Definition: stored.Definition.JSON(),
	}
}
