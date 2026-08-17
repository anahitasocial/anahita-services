package responses

import (
	"anahita-libs/data/entities"
)

type HashtagNodeItem struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`
	Alias string `json:"alias,omitempty"`
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
