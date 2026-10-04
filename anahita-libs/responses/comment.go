package responses

import (
	"anahita-libs/data/entities"
	"time"
)

type CommentItem struct {
	ID                 int64     `json:"id"`
	Type               string    `json:"type"`
	Name               string    `json:"name,omitempty"`
	Alias              string    `json:"alias,omitempty"`
	Body               string    `json:"body,omitempty"`
	QuoteCount         int64     `json:"quote_count,omitempty"`
	Verified           bool      `json:"verified,omitempty"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	FollowerCount      int64     `json:"follower_count,omitempty"`
	Mimetype           string    `json:"mimetype,omitempty"`
	Access             string    `json:"access,omitempty"`
	LikesCount         int64     `json:"likes_count,omitempty"`
	IsLikedByViewer    bool      `json:"is_liked_by_viewer,omitempty"`
	RepostCount        int64     `json:"repost_count,omitempty"`
	IsRepostedByViewer bool      `json:"is_reposted_by_viewer,omitempty"`
	IsLeadingViewer    bool      `json:"is_leading_viewer,omitempty"`
	IsAdminedByViewer  bool      `json:"is_admined_by_viewer,omitempty"`
	IsFollowingViewer  bool      `json:"is_following_viewer,omitempty"`
	IsBlockedByViewer  bool      `json:"is_blocked_by_viewer,omitempty"`

	// Related Entities
	Author *Author `json:"author,omitempty"`
	Editor *Editor `json:"editor,omitempty"`
	Parent *Parent `json:"parent,omitempty"`
}

type CommentResponse struct{}

func (r *CommentResponse) MapCommentItem(node *entities.AnahitaNode) *CommentItem {
	if node == nil {
		return nil
	}

	return &CommentItem{
		ID:                 node.ID,
		Type:               node.Type,
		Name:               node.Name,
		Alias:              node.Alias,
		Body:               node.Body,
		QuoteCount:         node.QuoteCount,
		Verified:           node.Verified,
		CreatedAt:          node.CreatedAt,
		FollowerCount:      node.FollowerCount,
		Mimetype:           node.Mimetype,
		Access:             node.Access,
		LikesCount:         node.LikesCount,
		IsLikedByViewer:    node.IsLikedByViewer,
		RepostCount:        node.RepostCount,
		IsRepostedByViewer: node.IsRepostedByViewer,
		IsLeadingViewer:    node.IsLeadingViewer,
		IsAdminedByViewer:  node.IsAdminedByViewer,
		IsFollowingViewer:  node.IsFollowingViewer,
		IsBlockedByViewer:  node.IsBlockedByViewer,
		Author: func() *Author {
			if node.Author != nil {
				return NewAuthor(node.Author)
			}
			return nil
		}(),
		Editor: func() *Editor {
			if node.Editor != nil {
				return NewEditor(node.Editor)
			}
			return nil
		}(),
		Parent: func() *Parent {
			if node.Parent != nil {
				p := NewParent(node.Parent)
				if node.Parent.Owner != nil {
					p.Owner = NewOwner(node.Parent.Owner)
				}
				return p
			}
			return nil
		}(),
	}
}

func (r *CommentResponse) MapCommentItems(nodes []*entities.AnahitaNode) []*CommentItem {
	if nodes == nil {
		return nil
	}

	results := make([]*CommentItem, 0, len(nodes))
	for _, node := range nodes {
		results = append(results, r.MapCommentItem(node))
	}
	return results
}
