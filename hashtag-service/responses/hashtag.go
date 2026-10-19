package responses

import (
	"anahita-libs/data/entities"
)

// HashtagNodeItem is a hashtag.
type HashtagNodeItem struct {
	// Its id.
	ID int64 `json:"id"`
	// Always `node.tag.hashtag-service.hashtag.v1`.
	Type string `json:"type"`
	// The hashtag as it is shown, without its #.
	Name string `json:"name,omitempty"`
	// What it is found by: `/hashtags/<alias>`.
	Alias string `json:"alias,omitempty"`
	// In a trending list: how many profiles used it in the window, each
	// once: the writers of posts carrying it, and the profiles tagged with it.
	Profiles uint64 `json:"profiles,omitempty"`
	// In the following and all lists: how many posts it is on that anybody
	// may see, and the viewer's own.
	Uses uint64 `json:"uses,omitempty"`
	// In the following and all lists: the viewer follows it.
	Followed bool `json:"followed,omitempty"`
}

type HashtagNode struct {
	HashtagNodeItem
	EdgeCount int64 `json:"edge_count,omitempty"`
}

type HashtagNodeResponse struct{}

func (r *HashtagNodeResponse) MapHashtagNodeItem(node *entities.AnahitaNode) *HashtagNodeItem {
	if node == nil {
		return nil
	}

	return &HashtagNodeItem{
		ID:    node.ID,
		Type:  node.Type,
		Name:  node.Name,
		Alias: node.Alias,
	}
}

func (r *HashtagNodeResponse) MapHashtagNode(node *entities.AnahitaNode) *HashtagNode {
	if node == nil {
		return nil
	}

	hashtagNodeItem := r.MapHashtagNodeItem(node)

	return &HashtagNode{
		HashtagNodeItem: *hashtagNodeItem,
		EdgeCount:       node.EdgeCount,
	}
}

func (r *HashtagNodeResponse) MapHashtagNodeItems(nodes []*entities.AnahitaNode) []*HashtagNodeItem {
	if nodes == nil {
		return nil
	}

	mapped := make([]*HashtagNodeItem, 0, len(nodes))
	for _, node := range nodes {
		mapped = append(mapped, r.MapHashtagNodeItem(node))
	}
	return mapped
}
