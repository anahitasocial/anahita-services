package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"time"
)

type CommentNodeItem struct {
	ID              int64     `json:"id"`
	Type            string    `json:"type"`
	Name            string    `json:"name,omitempty"`
	Alias           string    `json:"alias,omitempty"`
	Body            string    `json:"body,omitempty"`
	QuoteCount      int64     `json:"quote_count,omitempty"`
	Verified        bool      `json:"verified,omitempty"`
	CreationTime    time.Time `json:"creation_time,omitempty"`
	UpdateTime      time.Time `json:"update_time,omitempty"`
	Access          string    `json:"access,omitempty"`
	LikesCount      int64     `json:"likes_count,omitempty"`
	IsLikedByViewer bool      `json:"is_liked_by_viewer,omitempty"`

	// Related Entities
	Author *responses.Author `json:"author,omitempty"`
	Editor *responses.Editor `json:"editor,omitempty"`
	Parent *responses.Parent `json:"parent,omitempty"`

	Authorized *responses.NodeAuthorized `json:"authorized,omitempty"`
}

type CommentNodeResponse struct{}

func (r *CommentNodeResponse) MapCommentNodeItem(node *entities.AnahitaNode) *CommentNodeItem {
	if node == nil {
		return nil
	}

	return &CommentNodeItem{
		ID:              node.ID,
		Type:            node.Type,
		Name:            node.Name,
		Alias:           node.Alias,
		Body:            node.Body,
		QuoteCount:      node.QuoteCount,
		Verified:        node.Verified,
		CreationTime:    node.CreationTime,
		UpdateTime:      node.UpdateTime,
		Access:          node.Access,
		LikesCount:      node.LikesCount,
		IsLikedByViewer: node.IsLikedByViewer,
		Author: func() *responses.Author {
			if node.Author != nil {
				return responses.NewAuthor(node.Author)
			}
			return nil
		}(),
		Editor: func() *responses.Editor {
			if node.Editor != nil {
				return responses.NewEditor(node.Editor)
			}
			return nil
		}(),
		Parent: func() *responses.Parent {
			if node.Parent != nil {
				return responses.NewParent(node.Parent)
			}
			return nil
		}(),
		Authorized: responses.NewNodeAuthorized(node),
	}
}

func (r *CommentNodeResponse) MapCommentNodeItems(nodes []*entities.AnahitaNode) []*CommentNodeItem {
	var items []*CommentNodeItem
	for _, node := range nodes {
		items = append(items, r.MapCommentNodeItem(node))
	}
	return items
}
